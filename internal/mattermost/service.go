package mattermost

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

type ConversationKind int

const (
	ConversationChannel ConversationKind = iota
	ConversationDM
	ConversationGroup
)

type Conversation struct {
	Channel    *model.Channel
	Member     *model.ChannelMember
	Kind       ConversationKind
	Display    string
	SearchName string
	Unread     bool
	Mention    bool
}

type Session struct {
	User          *model.User
	Team          *model.Team
	Conversations []*Conversation
}

type EventKind int

const (
	EventConnected EventKind = iota
	EventReconnecting
	EventOffline
	EventPosted
	EventRefreshConversations
)

type Event struct {
	Kind EventKind
	Post *model.Post
	Raw  *model.WebSocketEvent
	Err  error
}

type Service struct {
	baseURL string
	token   string
	client  *model.Client4
	infoLog *slog.Logger
	errLog  *slog.Logger
	events  chan Event
	mu      sync.Mutex
	ws      *model.WebSocketClient
}

func New(baseURL, token string, infoLog, errLog *slog.Logger) *Service {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	client := model.NewAPIv4Client(baseURL)
	client.SetToken(strings.TrimSpace(token))
	return &Service{baseURL: baseURL, token: strings.TrimSpace(token), client: client, infoLog: infoLog, errLog: errLog, events: make(chan Event, 128)}
}

func (s *Service) Events() <-chan Event { return s.events }

func (s *Service) Validate(ctx context.Context) (*model.User, error) {
	user, _, err := s.client.GetMe(ctx, "")
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) Bootstrap(ctx context.Context) (*Session, error) {
	user, err := s.Validate(ctx)
	if err != nil {
		return nil, err
	}
	teams, _, err := s.client.GetTeamsForUser(ctx, user.Id, "")
	if err != nil {
		return nil, fmt.Errorf("load teams: %w", err)
	}
	if len(teams) == 0 {
		return nil, fmt.Errorf("Mattermost account has no accessible team")
	}
	team := teams[0]
	conversations, err := s.LoadConversations(ctx, user, team)
	if err != nil {
		return nil, err
	}
	return &Session{User: user, Team: team, Conversations: conversations}, nil
}

func (s *Service) LoadConversations(ctx context.Context, user *model.User, team *model.Team) ([]*Conversation, error) {
	channels, _, err := s.client.GetChannelsForTeamForUser(ctx, team.Id, user.Id, false, "")
	if err != nil {
		return nil, fmt.Errorf("load channels: %w", err)
	}
	members, _, err := s.client.GetChannelMembersForUser(ctx, user.Id, team.Id, "")
	if err != nil {
		return nil, fmt.Errorf("load channel memberships: %w", err)
	}
	memberByChannel := make(map[string]*model.ChannelMember, len(members))
	for i := range members {
		member := &members[i]
		memberByChannel[member.ChannelId] = member
	}
	userIDs := collectConversationUserIDs(channels, user.Id)
	usersByID := map[string]*model.User{user.Id: user}
	if len(userIDs) > 0 {
		users, _, getErr := s.client.GetUsersByIds(ctx, userIDs)
		if getErr == nil {
			for _, u := range users {
				usersByID[u.Id] = u
			}
		} else if s.errLog != nil {
			s.errLog.Warn("failed to resolve conversation users", "component", "mattermost", "error", getErr)
		}
	}
	result := make([]*Conversation, 0, len(channels))
	for _, ch := range channels {
		member := memberByChannel[ch.Id]
		conv := &Conversation{Channel: ch, Member: member}
		switch ch.Type {
		case model.ChannelTypeDirect:
			conv.Kind = ConversationDM
			otherID := directOtherUserID(ch.Name, user.Id)
			if u := usersByID[otherID]; u != nil {
				conv.Display = userDisplayName(u)
				conv.SearchName = u.Username
			} else {
				conv.Display = "Direct message"
				conv.SearchName = ch.Name
			}
		case model.ChannelTypeGroup:
			conv.Kind = ConversationGroup
			if ch.DisplayName != "" {
				conv.Display = ch.DisplayName
			} else {
				names := []string{}
				for _, id := range channelUserIDs(ch.Name) {
					if id == user.Id {
						continue
					}
					if u := usersByID[id]; u != nil {
						names = append(names, userDisplayName(u))
					}
				}
				if len(names) > 0 {
					conv.Display = strings.Join(names, ", ")
				} else {
					conv.Display = "Group message"
				}
			}
			conv.SearchName = ch.Name
		default:
			conv.Kind = ConversationChannel
			conv.Display = ch.DisplayName
			if conv.Display == "" {
				conv.Display = ch.Name
			}
			conv.SearchName = ch.Name
		}
		if member != nil {
			conv.Unread = member.LastViewedAt < ch.LastPostAt
			conv.Mention = member.MentionCount > 0
		}
		result = append(result, conv)
	}
	SortConversations(result)
	return result, nil
}

func SortConversations(items []*Conversation) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Unread != b.Unread {
			return a.Unread
		}
		return strings.ToLower(a.Display) < strings.ToLower(b.Display)
	})
}

func (s *Service) LoadPosts(ctx context.Context, channelID string, page, perPage int) ([]*model.Post, bool, error) {
	list, _, err := s.client.GetPostsForChannel(ctx, channelID, page, perPage, "", false, false)
	if err != nil {
		return nil, false, err
	}
	posts := postsFromList(list)
	return posts, len(posts) >= perPage, nil
}

func postsFromList(list *model.PostList) []*model.Post {
	if list == nil {
		return nil
	}
	posts := make([]*model.Post, 0, len(list.Posts))
	seen := map[string]bool{}
	for _, id := range list.Order {
		if p := list.Posts[id]; p != nil {
			posts = append(posts, p)
			seen[id] = true
		}
	}
	for id, p := range list.Posts {
		if !seen[id] && p != nil {
			posts = append(posts, p)
		}
	}
	sort.SliceStable(posts, func(i, j int) bool { return posts[i].CreateAt < posts[j].CreateAt })
	return posts
}

func MergePosts(existing, incoming []*model.Post) []*model.Post {
	byID := map[string]*model.Post{}
	for _, p := range existing {
		if p != nil {
			byID[p.Id] = p
		}
	}
	for _, p := range incoming {
		if p != nil {
			byID[p.Id] = p
		}
	}
	out := make([]*model.Post, 0, len(byID))
	for _, p := range byID {
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreateAt == out[j].CreateAt {
			return out[i].Id < out[j].Id
		}
		return out[i].CreateAt < out[j].CreateAt
	})
	return out
}

func (s *Service) SendPost(ctx context.Context, channelID, message, rootID string) (*model.Post, error) {
	created, _, err := s.client.CreatePost(ctx, &model.Post{ChannelId: channelID, Message: message, RootId: rootID})
	return created, err
}

func (s *Service) MarkRead(ctx context.Context, userID, channelID string) error {
	_, _, err := s.client.ViewChannel(ctx, userID, &model.ChannelView{ChannelId: channelID})
	return err
}

func (s *Service) StartWebSocket(ctx context.Context) { go s.websocketLoop(ctx) }

func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws != nil {
		s.ws.Close()
		s.ws = nil
	}
}

func (s *Service) websocketLoop(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		s.emit(Event{Kind: EventReconnecting})
		ws, err := model.NewWebSocketClient4(websocketBaseURL(s.baseURL), s.token)
		if err != nil {
			s.emit(Event{Kind: EventOffline, Err: err})
			sleepContext(ctx, backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		s.mu.Lock()
		s.ws = ws
		s.mu.Unlock()
		ws.Listen()
		backoff = time.Second
		s.emit(Event{Kind: EventConnected})
		if s.infoLog != nil {
			s.infoLog.Info("websocket connected", "component", "mattermost")
		}
		reconnect := false
		for !reconnect {
			select {
			case <-ctx.Done():
				ws.Close()
				return
			case <-ws.PingTimeoutChannel:
				reconnect = true
			case ev, ok := <-ws.EventChannel:
				if !ok || ev == nil {
					reconnect = true
					continue
				}
				s.handleWebSocketEvent(ev)
			case _, ok := <-ws.ResponseChannel:
				if !ok {
					reconnect = true
				}
			}
		}
		ws.Close()
		s.emit(Event{Kind: EventReconnecting, Err: ws.ListenError})
		sleepContext(ctx, backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (s *Service) handleWebSocketEvent(ev *model.WebSocketEvent) {
	switch ev.EventType() {
	case model.WebsocketEventPosted:
		value, ok := ev.GetData()["post"]
		if !ok {
			return
		}
		var post model.Post
		switch v := value.(type) {
		case string:
			if err := json.Unmarshal([]byte(v), &post); err != nil {
				if s.errLog != nil {
					s.errLog.Warn("decode websocket post", "component", "mattermost", "error", err)
				}
				return
			}
		case map[string]any:
			buf, _ := json.Marshal(v)
			if json.Unmarshal(buf, &post) != nil {
				return
			}
		default:
			return
		}
		s.emit(Event{Kind: EventPosted, Post: &post, Raw: ev})
	case model.WebsocketEventDirectAdded, model.WebsocketEventGroupAdded, model.WebsocketEventChannelCreated, model.WebsocketEventChannelUpdated:
		s.emit(Event{Kind: EventRefreshConversations, Raw: ev})
	}
}

func (s *Service) emit(ev Event) {
	select {
	case s.events <- ev:
	default:
		if s.errLog != nil {
			s.errLog.Warn("dropping mattermost event because buffer is full", "component", "mattermost", "event_kind", ev.Kind)
		}
	}
}

func websocketBaseURL(base string) string {
	if strings.HasPrefix(base, "https://") {
		return "wss://" + strings.TrimPrefix(base, "https://")
	}
	if strings.HasPrefix(base, "http://") {
		return "ws://" + strings.TrimPrefix(base, "http://")
	}
	return base
}

func sleepContext(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func collectConversationUserIDs(channels []*model.Channel, self string) []string {
	set := map[string]bool{}
	for _, ch := range channels {
		if ch.Type != model.ChannelTypeDirect && ch.Type != model.ChannelTypeGroup {
			continue
		}
		for _, id := range channelUserIDs(ch.Name) {
			if id != "" && id != self {
				set[id] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

func channelUserIDs(name string) []string {
	parts := strings.Split(name, "__")
	if len(parts) <= 1 {
		parts = strings.Split(name, "_")
	}
	return parts
}

func directOtherUserID(name, self string) string {
	for _, id := range channelUserIDs(name) {
		if id != self {
			return id
		}
	}
	return ""
}

func userDisplayName(u *model.User) string {
	if u == nil {
		return "Unknown"
	}
	if u.FirstName != "" || u.LastName != "" {
		name := strings.TrimSpace(u.FirstName + " " + u.LastName)
		if name != "" {
			return name
		}
	}
	if u.Nickname != "" {
		return u.Nickname
	}
	return u.Username
}

func (s *Service) ResolveUsers(ctx context.Context, posts []*model.Post) map[string]*model.User {
	set := map[string]bool{}
	for _, p := range posts {
		if p != nil && p.UserId != "" {
			set[p.UserId] = true
		}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	out := make(map[string]*model.User, len(ids))
	if len(ids) == 0 {
		return out
	}
	users, _, err := s.client.GetUsersByIds(ctx, ids)
	if err != nil {
		return out
	}
	for _, u := range users {
		out[u.Id] = u
	}
	return out
}

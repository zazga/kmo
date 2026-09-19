package app

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/zazga/kmo/internal/daemon"
	mm "github.com/zazga/kmo/internal/mattermost"
)

type bootstrapMsg struct { session *mm.Session; err error }
type setupValidatedMsg struct { service *mm.Service; session *mm.Session; url string; err error }
type messagesLoadedMsg struct { channelID string; posts []*model.Post; users map[string]*model.User; hasOlder bool; page int; err error }
type postSentMsg struct { post *model.Post; err error }
type conversationRefreshMsg struct { items []*mm.Conversation; err error }
type websocketMsg struct { event mm.Event }
type markedReadMsg struct { channelID string; err error }
type daemonStatusMsg struct { status daemon.Status }

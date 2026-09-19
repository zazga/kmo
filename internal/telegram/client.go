package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxMessageRunes = 3900

type Client struct {
	token string
	http  *http.Client
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type Message struct {
	MessageID      int64    `json:"message_id"`
	From           *User    `json:"from"`
	Chat           Chat     `json:"chat"`
	Date           int64    `json:"date"`
	Text           string   `json:"text"`
	ReplyToMessage *Message `json:"reply_to_message"`
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type apiResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	Description string `json:"description"`
	ErrorCode   int    `json:"error_code"`
}

func New(token string) *Client {
	return &Client{token: strings.TrimSpace(token), http: &http.Client{Timeout: 70 * time.Second}}
}

func (c *Client) endpoint(method string) string {
	return "https://api.telegram.org/bot" + c.token + "/" + method
}

func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var out apiResponse[User]
	if err := c.get(ctx, "getMe", nil, &out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, apiError(out.ErrorCode, out.Description)
	}
	return &out.Result, nil
}

func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSeconds int) ([]Update, error) {
	q := url.Values{}
	if offset > 0 {
		q.Set("offset", strconv.FormatInt(offset, 10))
	}
	q.Set("timeout", strconv.Itoa(timeoutSeconds))
	q.Set("allowed_updates", `["message"]`)
	var out apiResponse[[]Update]
	if err := c.get(ctx, "getUpdates", q, &out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, apiError(out.ErrorCode, out.Description)
	}
	return out.Result, nil
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) (*Message, error) {
	payload := map[string]any{"chat_id": chatID, "text": text, "disable_web_page_preview": true}
	var out apiResponse[Message]
	if err := c.post(ctx, "sendMessage", payload, &out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, apiError(out.ErrorCode, out.Description)
	}
	return &out.Result, nil
}

func (c *Client) get(ctx context.Context, method string, q url.Values, dst any) error {
	u := c.endpoint(method)
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode Telegram response: %w", err)
	}
	return nil
}

func (c *Client) post(ctx context.Context, method string, payload any, dst any) error {
	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(method), bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode Telegram response: %w", err)
	}
	return nil
}

func SplitMessage(header, body string) []string {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	prefix := header
	if prefix != "" {
		prefix += "\n"
	}
	available := maxMessageRunes - len([]rune(prefix)) - 12
	if available < 200 {
		available = 200
	}
	runes := []rune(body)
	if len(runes) <= available {
		return []string{prefix + body}
	}
	parts := make([]string, 0, (len(runes)/available)+1)
	for len(runes) > 0 {
		n := available
		if len(runes) < n {
			n = len(runes)
		}
		cut := n
		for i := n - 1; i > n/2; i-- {
			if runes[i] == '\n' || runes[i] == ' ' {
				cut = i + 1
				break
			}
		}
		parts = append(parts, strings.TrimSpace(string(runes[:cut])))
		runes = runes[cut:]
	}
	for i := range parts {
		parts[i] = fmt.Sprintf("%s(%d/%d)\n%s", prefix, i+1, len(parts), parts[i])
	}
	return parts
}

func apiError(code int, description string) error {
	if description == "" {
		description = "unknown Telegram API error"
	}
	return fmt.Errorf("Telegram API %d: %s", code, description)
}

package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"call-my-agent/pkg/model"
)

// Provider polls Telegram and sends replies.
type Provider struct {
	Token        string
	Client       *http.Client
	PollInterval time.Duration
	baseURL      string
}

// NewProvider constructs a Provider with defaults.
func NewProvider(token string, pollInterval time.Duration) *Provider {
	// Use an HTTP timeout that is comfortably larger than the poll interval.
	// Telegram's getUpdates may hold the connection open for up to the
	// requested timeout, so if the HTTP client timeout is too small we will
	// see spurious "context deadline exceeded (Client.Timeout ...)" errors
	// even when everything is healthy.
	httpTimeout := pollInterval*2 + 5*time.Second
	return &Provider{
		Token:        token,
		Client:       &http.Client{Timeout: httpTimeout},
		PollInterval: pollInterval,
		baseURL:      "https://api.telegram.org",
	}
}

// Receive starts polling Telegram for updates.
func (p *Provider) Receive(ctx context.Context) (<-chan model.InboundMessage, <-chan error) {
	msgCh := make(chan model.InboundMessage)
	errCh := make(chan error)
	go func() {
		defer close(msgCh)
		defer close(errCh)
		offset := int64(0)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(p.PollInterval):
				updates, err := p.getUpdates(ctx, offset)
				if err != nil {
					errCh <- err
					continue
				}
				for _, upd := range updates {
					offset = upd.UpdateID + 1
					if upd.Message == nil || upd.Message.Text == "" {
						continue
					}
					msgCh <- model.InboundMessage{
						ChatID:    fmt.Sprintf("%d", upd.Message.Chat.ID),
						MessageID: upd.Message.MessageID,
						Text:      upd.Message.Text,
						Timestamp: time.Unix(upd.Message.Date, 0),
					}
				}
			}
		}
	}()
	return msgCh, errCh
}

// Send sends a text message back to Telegram.
func (p *Provider) Send(ctx context.Context, chatID string, text string) error {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL("sendMessage"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram sendMessage status %s", resp.Status)
	}
	return nil
}

func (p *Provider) getUpdates(ctx context.Context, offset int64) ([]update, error) {
	payload := map[string]interface{}{
		"offset":  offset,
		"timeout": int(p.PollInterval.Seconds()),
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL("getUpdates"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("getUpdates status %s", resp.Status)
	}
	var response updateResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	if !response.OK {
		return nil, fmt.Errorf("telegram error: %v", response.Description)
	}
	return response.Result, nil
}

func (p *Provider) apiURL(method string) string {
	return fmt.Sprintf("%s/bot%s/%s", p.baseURL, p.Token, method)
}

func (p *Provider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

type updateResponse struct {
	OK          bool     `json:"ok"`
	Result      []update `json:"result"`
	Description string   `json:"description"`
}

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	MessageID int64  `json:"message_id"`
	From      user   `json:"from"`
	Chat      chat   `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text"`
}

type user struct {
	ID int64 `json:"id"`
}

type chat struct {
	ID int64 `json:"id"`
}

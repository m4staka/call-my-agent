package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
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
			if ctx.Err() != nil {
				return
			}
			updates, err := p.getUpdates(ctx, offset)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case errCh <- err:
				}
				// Avoid hammering Telegram on persistent failures by sleeping for the poll interval.
				select {
				case <-ctx.Done():
					return
				case <-time.After(p.PollInterval):
				}
				continue
			}
			for _, upd := range updates {
				offset = upd.UpdateID + 1
				if upd.Message == nil {
					continue
				}
				msg, err := p.toInboundMessage(ctx, upd.Message)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case errCh <- err:
					}
					continue
				}
				if msg == nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case msgCh <- *msg:
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

func (p *Provider) toInboundMessage(ctx context.Context, msg *message) (*model.InboundMessage, error) {
	if msg == nil {
		return nil, nil
	}
	base := &model.InboundMessage{
		ChatID:    fmt.Sprintf("%d", msg.Chat.ID),
		MessageID: msg.MessageID,
		Text:      msg.Text,
		Timestamp: time.Unix(msg.Date, 0),
	}
	if msg.Text != "" {
		return base, nil
	}

	if msg.Voice != nil {
		audio, err := p.fetchAudio(ctx, msg.Voice.FileID, msg.Voice.MimeType, "")
		if err != nil {
			return nil, err
		}
		base.Audio = audio
		return base, nil
	}

	if msg.Audio != nil {
		audio, err := p.fetchAudio(ctx, msg.Audio.FileID, msg.Audio.MimeType, msg.Audio.FileName)
		if err != nil {
			return nil, err
		}
		base.Audio = audio
		return base, nil
	}

	return nil, nil
}

func (p *Provider) fetchAudio(ctx context.Context, fileID, mimeType, providedName string) (*model.Audio, error) {
	fileInfo, err := p.getFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	name := providedName
	if name == "" {
		name = path.Base(fileInfo.FilePath)
	}
	content, err := p.downloadFile(ctx, fileInfo.FilePath)
	if err != nil {
		return nil, err
	}
	return &model.Audio{FileID: fileID, FileName: name, MimeType: mimeType, Data: content}, nil
}

func (p *Provider) getFile(ctx context.Context, fileID string) (fileResult, error) {
	payload := map[string]interface{}{"file_id": fileID}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL("getFile"), bytes.NewReader(body))
	if err != nil {
		return fileResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client().Do(req)
	if err != nil {
		return fileResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fileResult{}, fmt.Errorf("getFile status %s", resp.Status)
	}
	var response fileResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fileResult{}, err
	}
	if !response.OK {
		return fileResult{}, fmt.Errorf("telegram error: %v", response.Description)
	}
	return response.Result, nil
}

func (p *Provider) downloadFile(ctx context.Context, filePath string) ([]byte, error) {
	url := fmt.Sprintf("%s/file/bot%s/%s", p.baseURL, p.Token, filePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download file status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
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
	Voice     *voice `json:"voice"`
	Audio     *audio `json:"audio"`
}

type user struct {
	ID int64 `json:"id"`
}

type chat struct {
	ID int64 `json:"id"`
}

type voice struct {
	FileID   string `json:"file_id"`
	MimeType string `json:"mime_type"`
}

type audio struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
}

type fileResponse struct {
	OK          bool       `json:"ok"`
	Result      fileResult `json:"result"`
	Description string     `json:"description"`
}

type fileResult struct {
	FilePath string `json:"file_path"`
}

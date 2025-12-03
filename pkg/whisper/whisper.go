package whisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

// Audio represents a binary audio payload to transcribe.
type Audio struct {
	Data     []byte
	FileName string
	MimeType string
}

// Client calls the OpenAI Whisper API for transcription.
type Client struct {
	APIKey     string
	HTTPClient *http.Client
	APIBase    string
}

// NewClientFromEnv constructs a client using OPENAI_API_KEY.
func NewClientFromEnv() (*Client, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}
	return &Client{APIKey: key, APIBase: "https://api.openai.com/v1"}, nil
}

// Transcribe sends the provided audio clip to Whisper and returns the transcript.
func (c *Client) Transcribe(ctx context.Context, audio Audio) (string, error) {
	if len(audio.Data) == 0 {
		return "", fmt.Errorf("audio data is empty")
	}
	if audio.FileName == "" {
		audio.FileName = "audio.ogg"
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", audio.FileName)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio.Data); err != nil {
		return "", err
	}
	if err := writer.WriteField("model", "whisper-1"); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL("audio/transcriptions"), &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper transcription failed: status %s body %s", resp.Status, string(respBody))
	}
	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Text) == "" {
		return "", fmt.Errorf("empty transcription result")
	}
	return result.Text, nil
}

func (c *Client) apiURL(path string) string {
	base := c.APIBase
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(base, "/"), path)
}

func (c *Client) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 60 * time.Second}
}

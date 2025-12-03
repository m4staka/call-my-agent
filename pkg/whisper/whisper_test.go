package whisper

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranscribe(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("multipart reader: %v", err)
		}
		form, err := reader.ReadForm(1024)
		if err != nil {
			t.Fatalf("read form: %v", err)
		}
		if form.Value["model"][0] != "whisper-1" {
			t.Fatalf("unexpected model: %v", form.Value)
		}
		file := form.File["file"][0]
		f, err := file.Open()
		if err != nil {
			t.Fatalf("open file: %v", err)
		}
		data, _ := io.ReadAll(f)
		if string(data) != "audio-bytes" {
			t.Fatalf("unexpected payload: %s", string(data))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"hello"}`))
	}))
	defer server.Close()

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("client from env: %v", err)
	}
	client.APIBase = server.URL
	client.HTTPClient = server.Client()

	text, err := client.Transcribe(context.Background(), Audio{Data: []byte("audio-bytes"), FileName: "clip.ogg"})
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
	if text != "hello" {
		t.Fatalf("unexpected text: %s", text)
	}
}

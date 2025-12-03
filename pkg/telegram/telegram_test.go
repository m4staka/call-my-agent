package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"call-my-agent/pkg/model"
)

func TestReceiveVoiceMessage(t *testing.T) {
	audioData := []byte("voice-bytes")
	updatePayload, _ := json.Marshal(updateResponse{
		OK: true,
		Result: []update{{
			UpdateID: 1,
			Message: &message{
				MessageID: 10,
				Chat:      chat{ID: 123},
				Date:      1,
				Voice:     &voice{FileID: "voice1", MimeType: "audio/ogg"},
			},
		}},
	})
	filePayload, _ := json.Marshal(fileResponse{OK: true, Result: fileResult{FilePath: "voice/file_1.ogg"}})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			fmt.Fprint(w, string(updatePayload))
		case strings.HasSuffix(r.URL.Path, "/getFile"):
			fmt.Fprint(w, string(filePayload))
		case strings.Contains(r.URL.Path, "/file/bot"):
			w.Write(audioData)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewProvider("TOKEN", time.Millisecond)
	provider.baseURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgCh, errCh := provider.Receive(ctx)

	var msg model.InboundMessage
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case msg = <-msgCh:
		cancel()
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for message")
	}

	if msg.Audio == nil {
		t.Fatalf("expected audio payload")
	}
	if msg.Audio.FileID != "voice1" || msg.Audio.FileName != "file_1.ogg" {
		t.Fatalf("unexpected audio metadata: %+v", msg.Audio)
	}
	if string(msg.Audio.Data) != string(audioData) {
		t.Fatalf("unexpected audio data: %s", string(msg.Audio.Data))
	}
}

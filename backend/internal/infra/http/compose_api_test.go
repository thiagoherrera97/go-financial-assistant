package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
)

type fakeComposer struct{ got chat.Message }

func (f *fakeComposer) Send(ctx context.Context, msg chat.Message) []string {
	f.got = msg
	if msg.Audio != nil {
		data, _ := msg.Audio.Load(ctx)
		return []string{"áudio " + msg.Audio.MimeType + " " + string(data)}
	}
	return []string{"ok: " + msg.Text}
}

func composeMux(c Composer) *http.ServeMux {
	mux := http.NewServeMux()
	(&api{}).registerCompose(routes{mux: mux, protected: func(h http.HandlerFunc) http.Handler { return h }}, c)
	return mux
}

func TestCompose_Text(t *testing.T) {
	c := &fakeComposer{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/compose", strings.NewReader(`{"text":"  gastei 50  "}`))
	req.Header.Set("Content-Type", "application/json")
	composeMux(c).ServeHTTP(rec, req)
	var body struct{ Replies []string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != 200 || len(body.Replies) != 1 || body.Replies[0] != "ok: gastei 50" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestCompose_Audio(t *testing.T) {
	c := &fakeComposer{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/compose/audio", strings.NewReader("voz"))
	req.Header.Set("Content-Type", "audio/mp4;codecs=mp4a")
	composeMux(c).ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "áudio audio/mp4 voz") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestCompose_Rejects(t *testing.T) {
	cases := []struct {
		name, path, ctype, origin, body string
		want                            int
	}{
		{"texto vazio", "/api/compose", "application/json", "", `{"text":"  "}`, 400},
		{"texto sem JSON", "/api/compose", "text/plain", "", `{"text":"x"}`, 403},
		{"áudio de outro site", "/api/compose/audio", "audio/webm", "https://evil.example", "x", 403},
		{"não é áudio", "/api/compose/audio", "text/plain", "", "x", 415},
		{"áudio vazio", "/api/compose/audio", "audio/webm", "", "", 400},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, c.path, strings.NewReader(c.body))
		req.Header.Set("Content-Type", c.ctype)
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		composeMux(&fakeComposer{}).ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: %d, quer %d", c.name, rec.Code, c.want)
		}
	}
}

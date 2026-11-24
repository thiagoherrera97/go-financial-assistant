package httpserver

import (
	"context"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
)

const (
	maxComposeText  = 4 << 10
	maxComposeAudio = 15 << 20 // o mesmo teto do áudio no chat
)

// Composer é o painel "Lançar": manda uma mensagem ao mesmo cérebro do chat e devolve as respostas.
type Composer interface {
	Send(ctx context.Context, msg chat.Message) []string
}

// SetComposer liga o painel "Lançar" (chame antes de MountAPI).
func (s *Server) SetComposer(c Composer) { s.composer = c }

func (a *api) registerCompose(rt routes, c Composer) {
	rt.mux.Handle("POST /api/compose", rt.protected(jsonOnly(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
		}
		if !decodeJSON(w, r, maxComposeText, &body) {
			return
		}
		text := strings.TrimSpace(body.Text)
		if text == "" {
			writeError(w, http.StatusBadRequest, "mensagem vazia")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"replies": nonNil(c.Send(r.Context(), chat.Message{Text: text}))})
	})))

	// Áudio vai cru no corpo, com Content-Type audio/*: um formulário de outro site não consegue mandar esse tipo (CSRF).
	rt.mux.Handle("POST /api/compose/audio", rt.protected(sameOriginOnly(func(w http.ResponseWriter, r *http.Request) {
		mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if !strings.HasPrefix(mt, "audio/") {
			writeError(w, http.StatusUnsupportedMediaType, "mande um áudio")
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxComposeAudio))
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "áudio grande demais")
			return
		}
		if len(data) == 0 {
			writeError(w, http.StatusBadRequest, "áudio vazio")
			return
		}
		msg := chat.Message{Audio: &chat.Attachment{MimeType: mt, Load: func(context.Context) ([]byte, error) { return data, nil }}}
		writeJSON(w, http.StatusOK, map[string]any{"replies": nonNil(c.Send(r.Context(), msg))})
	})))
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

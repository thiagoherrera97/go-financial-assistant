package chat

import (
	"context"
	"errors"
	"strings"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// maxAudioBytes limita o áudio mandado ao Gemini (o envio inline aceita até ~20 MB); uma mensagem de voz de minutos fica bem abaixo.
const maxAudioBytes = 15 << 20

// Transcriber transforma áudio em texto (o Gemini).
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error)
}

// SetTranscriber habilita mensagens de voz: viram texto e seguem como se tivessem sido digitadas.
func (h *Handler) SetTranscriber(t Transcriber) { h.transcriber = t }

// transcribe devolve o texto do áudio, já mostrando ao usuário o que entendeu. ok=false quando já respondeu (erro ou sem fala).
func (h *Handler) transcribe(ctx context.Context, a *Attachment) (string, bool) {
	if h.transcriber == nil {
		h.sendText(ctx, "🎙️ Ainda não entendo áudio por aqui. Mande por texto.")
		return "", false
	}
	data, err := a.Load(ctx)
	if err != nil || len(data) == 0 {
		h.logger.Error("erro ao baixar áudio", "error", err)
		h.sendText(ctx, "🎙️ Não consegui baixar o áudio. Tente de novo.")
		return "", false
	}
	if len(data) > maxAudioBytes {
		h.sendText(ctx, "🎙️ Áudio grande demais. Mande um mais curto (até alguns minutos).")
		return "", false
	}
	text, err := h.transcriber.Transcribe(ctx, data, a.MimeType)
	if errors.Is(err, ports.ErrAIUnavailable) {
		h.sendText(ctx, aiOffText)
		return "", false
	}
	if err != nil {
		h.logger.Error("erro ao transcrever áudio", "error", err)
		h.sendText(ctx, "🎙️ Não consegui entender o áudio. Tente de novo ou mande por texto.")
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		h.sendText(ctx, "🎙️ Não ouvi nada no áudio. Tente de novo.")
		return "", false
	}
	h.sendText(ctx, "🎙️ Entendi: _"+text+"_")
	return text, true
}

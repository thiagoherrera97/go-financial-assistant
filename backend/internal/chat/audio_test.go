package chat

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

type fakeTranscriber struct {
	text     string
	err      error
	gotMime  string
	gotBytes int
}

func (f *fakeTranscriber) Transcribe(_ context.Context, audio []byte, mimeType string) (string, error) {
	f.gotMime, f.gotBytes = mimeType, len(audio)
	return f.text, f.err
}

func voice(data []byte) Message {
	return Message{Audio: &Attachment{MimeType: "audio/ogg", Load: loadOf(data, nil)}}
}

func TestHandle_Audio_ExpenseFollowsTextPath(t *testing.T) {
	a := &mockAnalyzer{}
	h, msgr := newTestHandler(a, &mockExporter{})
	tr := &fakeTranscriber{text: " gastei 50 no mercado "}
	h.SetTranscriber(tr)

	out, err := h.Handle(context.Background(), voice([]byte("ogg")))
	if err != nil || out == nil {
		t.Fatalf("esperava output, got %v, %v", out, err)
	}
	if tr.gotMime != "audio/ogg" || tr.gotBytes != 3 {
		t.Errorf("transcrição recebeu %q / %d bytes", tr.gotMime, tr.gotBytes)
	}
	if a.textSeen != "gastei 50 no mercado" {
		t.Errorf("texto transcrito não foi repassado: %q", a.textSeen)
	}
	if len(msgr.texts) != 2 || !strings.Contains(msgr.texts[0], "Entendi: _gastei 50 no mercado_") || !strings.Contains(msgr.texts[1], "Despesa registrada") {
		t.Errorf("respostas: %q", msgr.texts)
	}
}

func TestHandle_Audio_NoteFollowsNotePath(t *testing.T) {
	id := uuid.New()
	n := &mockNoter{found: [][]domain.NoteCandidate{{{PaymentID: id, Date: time.Now(), Description: "VILLA D ALDEA", Amount: 393}}}}
	h, _ := newTestHandler(&mockAnalyzer{}, &mockExporter{})
	h.SetNoter(n)
	h.SetTranscriber(&fakeTranscriber{text: "nota 393 dia 1/11 jantar pizza"})

	h.Handle(context.Background(), voice([]byte("ogg"))) //nolint:errcheck
	if n.set[id] != "jantar pizza" {
		t.Errorf("nota = %q", n.set[id])
	}
}

func TestHandle_Audio_Failures(t *testing.T) {
	cases := []struct {
		name string
		tr   *fakeTranscriber
		msg  Message
		want string
	}{
		{"sem transcrição", nil, voice([]byte("x")), "Ainda não entendo áudio"},
		{"IA desligada", &fakeTranscriber{err: ports.ErrAIUnavailable}, voice([]byte("x")), "IA está desligada"},
		{"erro do Gemini", &fakeTranscriber{err: errors.New("boom")}, voice([]byte("x")), "Não consegui entender"},
		{"sem fala", &fakeTranscriber{text: "  "}, voice([]byte("x")), "Não ouvi nada"},
		{"falha ao baixar", &fakeTranscriber{text: "oi"}, Message{Audio: &Attachment{Load: loadOf(nil, errors.New("rede"))}}, "Não consegui baixar"},
		{"grande demais", &fakeTranscriber{text: "oi"}, voice(make([]byte, maxAudioBytes+1)), "grande demais"},
	}
	for _, c := range cases {
		a := &mockAnalyzer{}
		h, msgr := newTestHandler(a, &mockExporter{})
		if c.tr != nil {
			h.SetTranscriber(c.tr)
		}
		out, err := h.Handle(context.Background(), c.msg)
		if out != nil || err != nil || a.textSeen != "" {
			t.Errorf("%s: não deveria registrar nada (out=%v err=%v texto=%q)", c.name, out, err, a.textSeen)
		}
		if len(msgr.texts) != 1 || !strings.Contains(msgr.texts[0], c.want) {
			t.Errorf("%s: resposta %q, quer conter %q", c.name, msgr.texts, c.want)
		}
	}
}

package chat

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func TestWeb_SendReturnsRepliesPerMessage(t *testing.T) {
	a := &mockAnalyzer{}
	w := NewWeb(a, &mockExporter{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	got := w.Send(context.Background(), Message{Text: "gastei 50 no almoço"})
	if len(got) != 1 || !strings.Contains(got[0], "Despesa registrada") || a.textSeen != "gastei 50 no almoço" {
		t.Fatalf("respostas %q, texto %q", got, a.textSeen)
	}
	// A próxima mensagem não repete as respostas da anterior.
	w.Handler().SetTranscriber(&fakeTranscriber{text: "gastei 12,50 na padaria"})
	got = w.Send(context.Background(), voice([]byte("m4a")))
	if len(got) != 2 || !strings.Contains(got[0], "Entendi") || !strings.Contains(got[1], "Despesa registrada") {
		t.Fatalf("áudio: %q", got)
	}
}

func TestWeb_NoteChoiceKeepsStateBetweenMessages(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	n := &mockNoter{found: [][]domain.NoteCandidate{{
		{PaymentID: a, Date: time.Now(), Description: "VILLA", Amount: 393},
		{PaymentID: b, Date: time.Now(), Description: "VILLA", Amount: 393},
	}}}
	w := NewWeb(&mockAnalyzer{}, &mockExporter{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w.Handler().SetNoter(n)
	if got := w.Send(context.Background(), Message{Text: "nota 393 pizza"}); len(got) != 1 || !strings.Contains(got[0], "Responda com o número") {
		t.Fatalf("deveria listar opções: %q", got)
	}
	w.Send(context.Background(), Message{Text: "2"})
	if n.set[b] != "pizza" {
		t.Errorf("nota = %+v", n.set)
	}
}

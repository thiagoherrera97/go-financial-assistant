package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func TestParseNoteRequest(t *testing.T) {
	today := time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC)
	day := func(m, d int) string {
		return time.Date(2026, time.Month(m), d, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	cases := []struct {
		in, day, note string
		amount        float64
	}{
		{"nota 5400 dia 10 insulfilm, minha mãe está ajudando", day(11, 10), "insulfilm, minha mãe está ajudando", 5400},
		{"Nota 393 dia 1/11: jantar pizza com os pais da Isa", day(11, 1), "jantar pizza com os pais da Isa", 393},
		{"/nota pix 5.400,00 10/11 insulfilm", day(11, 10), "pix insulfilm", 5400},
		{"nota 279,89 dia 25 presente", day(10, 25), "presente", 279.89}, // 25/11 ainda não chegou → outubro
		{"nota 50 almoço: com 2 amigos", "", "almoço com 2 amigos", 50},
		{"nota R$ 99,99 dia 29/07/26 tomada", "2026-07-29", "tomada", 99.99},
	}
	for _, c := range cases {
		req, ok := parseNoteRequest(c.in, today)
		if !ok {
			t.Errorf("%q: não entendeu", c.in)
			continue
		}
		gotDay := ""
		if req.day != nil {
			gotDay = req.day.Format("2006-01-02")
		}
		if req.amount != c.amount || gotDay != c.day || req.note != c.note {
			t.Errorf("%q = %v %q %q, quer %v %q %q", c.in, req.amount, gotDay, req.note, c.amount, c.day, c.note)
		}
	}
	for _, bad := range []string{"nota", "nota insulfilm", "nota 5400", "nota 50 dia 31/02 x"} {
		if _, ok := parseNoteRequest(bad, today); ok {
			t.Errorf("%q deveria pedir o formato", bad)
		}
	}
	if !isNoteCommand("Nota 10 x") || isNoteCommand("notas fiscais") || isNoteCommand("anotação") {
		t.Error("isNoteCommand")
	}
}

type mockNoter struct {
	found   [][]domain.NoteCandidate // uma resposta por chamada
	calls   int
	set     map[uuid.UUID]string
	pending []string
}

func (m *mockNoter) NoteCandidates(context.Context, float64, time.Time, time.Time) ([]domain.NoteCandidate, error) {
	m.calls++
	if m.calls > len(m.found) {
		return nil, nil
	}
	return m.found[m.calls-1], nil
}
func (m *mockNoter) SetTransactionNote(_ context.Context, id uuid.UUID, note string) (bool, error) {
	if m.set == nil {
		m.set = map[uuid.UUID]string{}
	}
	m.set[id] = note
	return true, nil
}
func (m *mockNoter) SavePendingNote(_ context.Context, amount float64, day time.Time, note string) error {
	m.pending = append(m.pending, note)
	return nil
}

func TestHandle_Note_SingleMatch(t *testing.T) {
	id := uuid.New()
	n := &mockNoter{found: [][]domain.NoteCandidate{{{PaymentID: id, Date: time.Now(), Description: "Pix recebido MONICA", Amount: 5400}}}}
	a := &mockAnalyzer{}
	h, msgr := newTestHandler(a, &mockExporter{})
	h.SetNoter(n)
	if _, err := h.Handle(context.Background(), Message{Text: "nota 5400 dia 10 insulfilm"}); err != nil {
		t.Fatal(err)
	}
	if n.set[id] != "insulfilm" || !strings.Contains(msgr.texts[0], "Pix recebido MONICA") {
		t.Errorf("nota = %q, resposta %q", n.set[id], msgr.texts)
	}
}

func TestHandle_Note_ChoiceThenNumber(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	n := &mockNoter{found: [][]domain.NoteCandidate{{
		{PaymentID: a, Date: time.Now(), Description: "VILLA D ALDEA", Amount: 393},
		{PaymentID: b, Date: time.Now(), Description: "VILLA D ALDEA", Amount: 393},
	}}}
	h, msgr := newTestHandler(&mockAnalyzer{}, &mockExporter{})
	h.SetNoter(n)
	h.Handle(context.Background(), Message{Text: "nota 393 pizza"}) //nolint:errcheck
	if !strings.Contains(msgr.texts[0], "Responda com o número") {
		t.Fatalf("deveria listar opções: %q", msgr.texts[0])
	}
	h.Handle(context.Background(), Message{Text: "2"}) //nolint:errcheck
	if n.set[b] != "pizza" || n.set[a] != "" {
		t.Errorf("nota foi para a opção errada: %+v", n.set)
	}
	// Sem lista esperando, um número volta para o fluxo normal.
	h.Handle(context.Background(), Message{Text: "2"}) //nolint:errcheck
	if len(n.set) != 1 {
		t.Errorf("número sem lista não deveria gravar nota")
	}
}

func TestHandle_Note_NotFoundIsPending(t *testing.T) {
	n := &mockNoter{}
	h, msgr := newTestHandler(&mockAnalyzer{}, &mockExporter{})
	h.SetNoter(n)
	h.Handle(context.Background(), Message{Text: "nota 120 dia 1 farmácia da vó"}) //nolint:errcheck
	if n.calls != 2 || len(n.pending) != 1 || n.pending[0] != "farmácia da vó" || !strings.Contains(msgr.texts[0], "Guardei a nota") {
		t.Errorf("calls=%d pending=%v resposta=%q", n.calls, n.pending, msgr.texts)
	}
}

func TestHandle_Note_BadFormatShowsHelp(t *testing.T) {
	h, msgr := newTestHandler(&mockAnalyzer{}, &mockExporter{})
	h.SetNoter(&mockNoter{})
	h.Handle(context.Background(), Message{Text: "nota insulfilm"}) //nolint:errcheck
	if len(msgr.texts) != 1 || !strings.Contains(msgr.texts[0], "nota 5400 dia 10") {
		t.Errorf("deveria mostrar como usar: %q", msgr.texts)
	}
}

func TestBRL(t *testing.T) {
	for v, want := range map[float64]string{5400: "R$ 5.400,00", 279.89: "R$ 279,89", 1234567.5: "R$ 1.234.567,50", 0.5: "R$ 0,50"} {
		if got := brl(v); got != want {
			t.Errorf("brl(%v) = %q", v, got)
		}
	}
}

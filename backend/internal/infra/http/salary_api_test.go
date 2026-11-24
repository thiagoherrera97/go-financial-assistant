package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/google/uuid"
)

func newSalaryAPI(t *testing.T, schedule string, reader *fakeReader, now time.Time) *Server {
	t.Helper()
	cipher, err := settings.NewCipher("")
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := settings.NewService(&memSettings{rows: map[string]settings.Row{}}, cipher, logger)
	svc.Load(context.Background()) //nolint:errcheck
	if schedule != "" {
		if _, err := svc.Set(context.Background(), map[string]string{"SALARY_SCHEDULE": schedule}); err != nil {
			t.Fatal(err)
		}
	}
	s := NewServer(0, logger)
	s.SetSettings(&SettingsDeps{Service: svc, Channel: "telegram"})
	if err := s.mountAPI(StaticPassword(testPassword), reader, func() time.Time { return now }); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAPI_Salary(t *testing.T) {
	r := &fakeReader{salaryCredits: []domain.SalaryCredit{{Date: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC), Amount: 5200}, {Date: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC), Amount: 1234.5}}}
	s := newSalaryAPI(t, "15=5200; 30=4366,62", r, time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC))

	rec := do(s, "GET", "/api/salary", "", nil, login(t, s))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Configured bool    `json:"configured"`
		Month      string  `json:"month"`
		Expected   float64 `json:"expected"`
		Received   float64 `json:"received"`
		Pending    float64 `json:"pending"`
		Extras     []struct {
			Date   string  `json:"date"`
			Amount float64 `json:"amount"`
		} `json:"extras"`
		ExtrasTotal float64 `json:"extrasTotal"`
		Parts       []struct {
			Day          int     `json:"day"`
			ExpectedDate string  `json:"expectedDate"`
			Received     bool    `json:"received"`
			ReceivedDate *string `json:"receivedDate"`
		} `json:"parts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Configured || got.Month != "2026-11" || got.Expected != 9566.62 || got.Received != 5200 || got.Pending != 4366.62 {
		t.Errorf("totais: %+v", got)
	}
	if len(got.Extras) != 1 || got.Extras[0].Date != "2026-11-18" || got.ExtrasTotal != 1234.5 {
		t.Errorf("extras: %+v", got.Extras)
	}
	if len(got.Parts) != 2 || got.Parts[0].ExpectedDate != "2026-11-13" || !got.Parts[0].Received || *got.Parts[0].ReceivedDate != "2026-11-13" ||
		got.Parts[1].ExpectedDate != "2026-11-30" || got.Parts[1].Received {
		t.Errorf("partes: %+v", got.Parts)
	}
}

func TestAPI_SalaryNotConfigured(t *testing.T) {
	s := newSalaryAPI(t, "", &fakeReader{}, time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC))
	rec := do(s, "GET", "/api/salary", "", nil, login(t, s))
	var got struct {
		Configured bool  `json:"configured"`
		Parts      []any `json:"parts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != 200 || got.Configured || len(got.Parts) != 0 {
		t.Errorf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestAPI_ExpectedIncome(t *testing.T) {
	r := &fakeReader{salaryCredits: []domain.SalaryCredit{{Date: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC), Amount: 5200}}}
	s := newSalaryAPI(t, "15=5200; 30=4366,62", r, time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC))
	c := login(t, s)

	rec := do(s, "GET", "/api/transactions/expected-income?month=2026-11", "", nil, c)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Total float64 `json:"total"`
		Items []struct {
			Source string  `json:"source"`
			Label  string  `json:"label"`
			Date   string  `json:"date"`
			Amount float64 `json:"amount"`
			Late   bool    `json:"late"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 4366.62 || len(got.Items) != 1 || got.Items[0].Date != "2026-11-30" || got.Items[0].Source != "SALARY" || got.Items[0].Late {
		t.Errorf("got %+v", got)
	}

	// Ciclo passado não tem previstas.
	if body := do(s, "GET", "/api/transactions/expected-income?month=2026-09", "", nil, c).Body.String(); body != `{"items":[],"total":0}`+"\n" {
		t.Errorf("passado: %q", body)
	}
	if code := do(s, "GET", "/api/transactions/expected-income?month=x", "", nil, c).Code; code != 400 {
		t.Errorf("mês inválido: %d", code)
	}
}

func (f *fakeReader) DecideSalary(_ context.Context, id uuid.UUID, kind domain.SalaryKind, month *time.Time, day int) error {
	m := "-"
	if month != nil {
		m = month.Format("2006-01")
	}
	f.salaryDecided = append(f.salaryDecided, fmt.Sprintf("%s %s %s %d", id, kind, m, day))
	return f.err
}

func (f *fakeReader) UndoSalaryDecision(_ context.Context, id uuid.UUID) error {
	f.salaryDecided = append(f.salaryDecided, "undo "+id.String())
	return f.err
}

func TestAPI_SalaryReview(t *testing.T) {
	extra, decided := uuid.New(), uuid.New()
	d := func(day int) time.Time { return time.Date(2026, 11, day, 0, 0, 0, 0, time.UTC) }
	r := &fakeReader{salaryCredits: []domain.SalaryCredit{
		{ID: uuid.NewString(), Date: d(13), Amount: 5200, Label: "SALARIO"},
		{ID: extra.String(), Date: d(18), Amount: 1234.5, Label: "SALARIO ADIC"},
		{ID: decided.String(), Date: d(19), Amount: 900, Label: "SALARIO", Kind: domain.SalaryKindOvertime},
	}}
	s := newSalaryAPI(t, "15=5200; 30=4366,62", r, time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC))
	refreshed := 0
	s.SetCyclesRefresh(func(context.Context) error { refreshed++; return nil })
	c := login(t, s)

	rec := do(s, "GET", "/api/salary/review", "", nil, c)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Pending []struct {
			ID      string  `json:"id"`
			Date    string  `json:"date"`
			Amount  float64 `json:"amount"`
			Label   string  `json:"label"`
			Suggest string  `json:"suggest"`
			Month   string  `json:"month"`
		} `json:"pending"`
		Decided []struct {
			ID        string `json:"id"`
			Kind      string `json:"kind"`
			KindLabel string `json:"kindLabel"`
		} `json:"decided"`
		Parts []struct {
			Day    int     `json:"day"`
			Amount float64 `json:"amount"`
		} `json:"parts"`
		Kinds []struct{ Value, Label string } `json:"kinds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Pending) != 1 || got.Pending[0].ID != extra.String() || got.Pending[0].Suggest != "" || got.Pending[0].Month != "2026-11" || got.Pending[0].Label != "SALARIO ADIC" {
		t.Errorf("pendentes: %+v", got.Pending)
	}
	if len(got.Decided) != 1 || got.Decided[0].ID != decided.String() || got.Decided[0].KindLabel != "Hora extra" {
		t.Errorf("decididos: %+v", got.Decided)
	}
	if len(got.Parts) != 2 || got.Parts[1].Day != 30 || len(got.Kinds) != len(domain.SalaryKinds) {
		t.Errorf("opções: %+v / %+v", got.Parts, got.Kinds)
	}

	put := func(body string) int {
		return do(s, "PUT", "/api/salary/review/"+extra.String(), body, map[string]string{"Content-Type": "application/json"}, c).Code
	}
	for _, bad := range []string{`{"kind":"X"}`, `{"kind":"PART","month":"2026-11","day":20}`, `{"kind":"MONTH"}`, `{"kind":"MONTH","month":"nov"}`} {
		if code := put(bad); code != 400 {
			t.Errorf("%s: %d, quer 400", bad, code)
		}
	}
	if len(r.salaryDecided) != 0 || refreshed != 0 {
		t.Fatalf("decisão inválida gravou: %v", r.salaryDecided)
	}
	for _, ok := range []string{`{"kind":"PLR","month":"2026-11","day":30}`, `{"kind":"PART","month":"2026-11","day":30}`, `{"kind":"MONTH","month":"2026-12"}`} {
		if code := put(ok); code != 204 {
			t.Errorf("%s: %d", ok, code)
		}
	}
	want := []string{extra.String() + " PLR - 0", extra.String() + " PART 2026-11 30", extra.String() + " MONTH 2026-12 0"}
	if fmt.Sprint(r.salaryDecided) != fmt.Sprint(want) || refreshed != 3 {
		t.Errorf("gravado %v (refresh %d), quer %v", r.salaryDecided, refreshed, want)
	}
	if code := do(s, "DELETE", "/api/salary/review/"+extra.String(), "", nil, c).Code; code != 204 || r.salaryDecided[3] != "undo "+extra.String() || refreshed != 4 {
		t.Errorf("desfazer: %d %v", code, r.salaryDecided)
	}
	r.err = domain.ErrSalaryCreditNotFound
	if code := put(`{"kind":"PLR"}`); code != 404 {
		t.Errorf("crédito sumiu: %d", code)
	}
}

func TestAPI_SalaryExtrasKind(t *testing.T) {
	r := &fakeReader{salaryCredits: []domain.SalaryCredit{{ID: uuid.NewString(), Date: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC), Amount: 3000, Kind: domain.SalaryKindPLR}}}
	s := newSalaryAPI(t, "15=5200; 30=4366,62", r, time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC))
	body := do(s, "GET", "/api/salary", "", nil, login(t, s)).Body.String()
	if !strings.Contains(body, `"kind":"PLR"`) || !strings.Contains(body, `"kindLabel":"PLR"`) {
		t.Errorf("extra sem o tipo: %s", body)
	}
}

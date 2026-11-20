package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
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

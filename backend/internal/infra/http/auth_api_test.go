package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
)

// O nome no topo: DISPLAY_NAME; sem ele, o primeiro nome do primeiro de OWN_NAMES.
func TestAPI_MeOwnerName(t *testing.T) {
	cipher, err := settings.NewCipher("")
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := settings.NewService(&memSettings{rows: map[string]settings.Row{}}, cipher, logger)
	svc.Load(context.Background()) //nolint:errcheck
	s := NewServer(0, logger)
	s.SetSettings(&SettingsDeps{Service: svc, Channel: "telegram"})
	if err := s.mountAPI(StaticPassword(testPassword), &fakeReader{}, time.Now); err != nil {
		t.Fatal(err)
	}
	c := login(t, s)
	name := func() string {
		t.Helper()
		var b struct{ Name string }
		rec := do(s, "GET", "/api/me", "", nil, c)
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &b) != nil {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		return b.Name
	}
	if n := name(); n != "" {
		t.Errorf("sem nomes: %q", n)
	}
	svc.Set(context.Background(), map[string]string{"OWN_NAMES": "JOANA PRADO LIMA, Joana P Lima"}) //nolint:errcheck
	if n := name(); n != "Joana" {
		t.Errorf("do OWN_NAMES: %q", n)
	}
	svc.Set(context.Background(), map[string]string{"DISPLAY_NAME": "Jô"}) //nolint:errcheck
	if n := name(); n != "Jô" {
		t.Errorf("DISPLAY_NAME vence: %q", n)
	}
}

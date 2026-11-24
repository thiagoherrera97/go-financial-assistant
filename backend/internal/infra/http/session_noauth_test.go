package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionsOpenDispensaLogin(t *testing.T) {
	s, err := newSessions(StaticPassword("senha-bem-comprida"))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	if s.valid(r) {
		t.Fatal("sem cookie não deveria valer com o login ligado")
	}
	if s.id(r) != "" {
		t.Fatal("sem cookie o id deveria ser vazio com o login ligado")
	}

	s.open = true
	if !s.valid(r) {
		t.Fatal("com DASHBOARD_AUTH=off toda requisição deveria valer")
	}
	if s.id(r) != noAuthSessionID {
		t.Fatalf("id = %q, quer %q", s.id(r), noAuthSessionID)
	}
	rec := httptest.NewRecorder()
	s.require(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })).ServeHTTP(rec, r)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("require respondeu %d", rec.Code)
	}
}

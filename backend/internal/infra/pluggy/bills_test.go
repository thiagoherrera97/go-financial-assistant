package pluggy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func billsServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, `{"apiKey":"KEY"}`) })
	mux.HandleFunc("/bills", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient("id", "secret")
	c.baseURL = srv.URL
	return c
}

func TestBills_ParsesAndPaginates(t *testing.T) {
	c := billsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("accountId") != "card1" {
			t.Errorf("accountId inesperado: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("page") == "2" {
			io.WriteString(w, `{"totalPages":2,"results":[{"id":"b2","dueDate":"2026-10-10T00:00:00.000Z","totalAmount":8796.54,"totalAmountCurrencyCode":"BRL"}]}`)
			return
		}
		io.WriteString(w, `{"totalPages":2,"results":[
			{"id":"b1","dueDate":"2026-11-10T00:00:00.000Z","totalAmount":8583.09,"totalAmountCurrencyCode":"BRL","minimumPaymentAmount":1000},
			{"id":"usd","dueDate":"2026-11-10","totalAmount":10,"totalAmountCurrencyCode":"USD"},
			{"id":"sem-data","dueDate":"","totalAmount":10}]}`)
	})

	got, err := c.bills(context.Background(), "card1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "b1" || got[0].Total != 8583.09 || !got[0].DueDate.Equal(time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)) || got[0].AccountID != "card1" || got[1].ID != "b2" {
		t.Errorf("faturas inesperadas: %+v", got)
	}
}

func TestBills_ErrorIsReturned(t *testing.T) {
	c := billsServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
	if _, err := c.bills(context.Background(), "card1"); err == nil {
		t.Error("esperava erro quando o conector não oferece faturas")
	}
}

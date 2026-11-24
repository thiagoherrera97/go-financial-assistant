package gemini

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/genai"
)

func noDelays(t *testing.T) {
	old := retryDelays
	retryDelays = []time.Duration{0, 0}
	t.Cleanup(func() { retryDelays = old })
}

// fakeCalls responde por modelo com a sequência de erros dada (nil = sucesso) e anota cada chamada.
func fakeCalls(byModel map[string][]error, calls *[]string) generateFunc {
	return func(_ context.Context, m string) (*genai.GenerateContentResponse, error) {
		*calls = append(*calls, m)
		seq := byModel[m]
		if len(seq) == 0 {
			return nil, genai.APIError{Code: 404}
		}
		err := seq[0]
		byModel[m] = seq[1:]
		if err != nil {
			return nil, err
		}
		return makeResponse("ok"), nil
	}
}

var overloaded = genai.APIError{Code: 503, Status: "UNAVAILABLE"}

func TestWithFallback_RetriesSameModel(t *testing.T) {
	noDelays(t)
	var calls []string
	resp, err := withFallback(context.Background(), "primary", fakeCalls(map[string][]error{
		"primary": {overloaded, nil},
	}, &calls))
	if err != nil || resp == nil {
		t.Fatalf("esperava sucesso, got %v", err)
	}
	if len(calls) != 2 || calls[1] != "primary" {
		t.Errorf("esperava 2 chamadas no primary, got %v", calls)
	}
}

func TestWithFallback_SwitchesModelWhenStillOverloaded(t *testing.T) {
	noDelays(t)
	var calls []string
	_, err := withFallback(context.Background(), "primary", fakeCalls(map[string][]error{
		"primary":         {overloaded, overloaded, overloaded},
		fallbackModels[0]: {nil},
	}, &calls))
	if err != nil {
		t.Fatalf("esperava sucesso no fallback, got %v", err)
	}
	if want := []string{"primary", "primary", "primary", fallbackModels[0]}; len(calls) != len(want) || calls[3] != want[3] {
		t.Errorf("chamadas = %v, quer %v", calls, want)
	}
}

func TestWithFallback_SkipsMissingFallback(t *testing.T) {
	noDelays(t)
	var calls []string
	_, err := withFallback(context.Background(), "primary", fakeCalls(map[string][]error{
		"primary":         {overloaded, overloaded, overloaded},
		fallbackModels[1]: {nil},
	}, &calls))
	if err != nil {
		t.Fatalf("esperava sucesso no segundo fallback, got %v", err)
	}
}

func TestWithFallback_NonTransientFailsFast(t *testing.T) {
	noDelays(t)
	var calls []string
	bad := genai.APIError{Code: 400}
	_, err := withFallback(context.Background(), "primary", fakeCalls(map[string][]error{
		"primary": {bad},
	}, &calls))
	var apiErr genai.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 400 {
		t.Fatalf("esperava o erro 400, got %v", err)
	}
	if len(calls) != 1 {
		t.Errorf("erro não transitório não deveria repetir, got %v", calls)
	}
}

func TestWithFallback_AllFailReturnsPrimaryError(t *testing.T) {
	noDelays(t)
	var calls []string
	_, err := withFallback(context.Background(), "primary", fakeCalls(map[string][]error{
		"primary":         {overloaded, overloaded, overloaded},
		fallbackModels[0]: {overloaded, overloaded, overloaded},
		fallbackModels[1]: {overloaded, overloaded, overloaded},
	}, &calls))
	var apiErr genai.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 503 {
		t.Fatalf("esperava o 503 do primary, got %v", err)
	}
	if len(calls) != 9 {
		t.Errorf("esperava 9 chamadas, got %d", len(calls))
	}
}

func TestIsTransient(t *testing.T) {
	cases := map[int]bool{429: true, 500: true, 503: true, 400: false, 403: false, 404: false}
	for code, want := range cases {
		if got := isTransient(genai.APIError{Code: code}); got != want {
			t.Errorf("isTransient(%d) = %v, quer %v", code, got, want)
		}
	}
	if isTransient(errors.New("x")) {
		t.Error("erro comum não é transitório")
	}
}

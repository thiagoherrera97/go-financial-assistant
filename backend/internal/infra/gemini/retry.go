package gemini

import (
	"context"
	"errors"
	"net/http"
	"time"

	"google.golang.org/genai"
)

// fallbackModels são tentados, em ordem, quando o modelo pedido continua sobrecarregado ou sem cota. Cada modelo tem
// capacidade e cota próprias no Google, então um pico num deles raramente pega os outros. São aliases "-latest"
// para não envelhecerem; um que não existir mais só é pulado.
var fallbackModels = []string{"gemini-flash-lite-latest", "gemini-flash-latest"}

// retryDelays são as esperas entre as tentativas num mesmo modelo antes de passar ao próximo.
var retryDelays = []time.Duration{time.Second, 2 * time.Second}

type generateFunc func(ctx context.Context, model string) (*genai.GenerateContentResponse, error)

// isTransient diz se o erro é de sobrecarga, cota ou falha do Google, que costuma passar sozinho ou em outro modelo.
func isTransient(err error) bool {
	var apiErr genai.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Code {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// withFallback chama call no modelo pedido, repetindo com espera em erro transitório, e depois nos fallbackModels.
// Um erro que não é transitório no modelo pedido (chave inválida, pedido malformado) volta na hora; num fallback,
// ele só faz pular para o próximo. Se nada der certo, devolve o erro do modelo pedido, que é o que explica o problema.
func withFallback(ctx context.Context, model string, call generateFunc) (*genai.GenerateContentResponse, error) {
	models := []string{model}
	for _, m := range fallbackModels {
		if m != model {
			models = append(models, m)
		}
	}

	var firstErr error
	for i, m := range models {
		for attempt := 0; ; attempt++ {
			resp, err := call(ctx, m)
			if err == nil {
				return resp, nil
			}
			if firstErr == nil {
				firstErr = err
			}
			if !isTransient(err) {
				if i == 0 {
					return nil, err
				}
				break
			}
			if attempt >= len(retryDelays) {
				break
			}
			select {
			case <-ctx.Done():
				return nil, firstErr
			case <-time.After(retryDelays[attempt]):
			}
		}
	}
	return nil, firstErr
}

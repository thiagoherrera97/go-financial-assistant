package pluggy

import (
	"context"
	"net/url"
	"strconv"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// bill é uma fatura fechada de cartão (GET /bills). Só o necessário: id, vencimento e total.
type bill struct {
	ID          string  `json:"id"`
	DueDate     string  `json:"dueDate"`
	TotalAmount float64 `json:"totalAmount"`
	Currency    string  `json:"totalAmountCurrencyCode"`
}

// bills lê as faturas fechadas do cartão. Nem todo conector oferece: quem chama trata o erro como "sem faturas".
func (c *Client) bills(ctx context.Context, accountID string) ([]ports.ExternalBill, error) {
	var out []ports.ExternalBill
	for page := 1; page <= maxPages; page++ {
		var res struct {
			TotalPages int    `json:"totalPages"`
			Results    []bill `json:"results"`
		}
		q := url.Values{"accountId": {accountID}, "page": {strconv.Itoa(page)}}
		if err := c.get(ctx, "/bills?"+q.Encode(), &res); err != nil {
			return nil, err
		}
		for _, b := range res.Results {
			due := parseDay(b.DueDate)
			if b.ID == "" || due == nil || (b.Currency != "" && b.Currency != "BRL") {
				continue
			}
			out = append(out, ports.ExternalBill{ID: b.ID, AccountID: accountID, DueDate: *due, Total: b.TotalAmount})
		}
		if page >= res.TotalPages {
			break
		}
	}
	return out, nil
}

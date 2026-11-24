package chat

import (
	"context"
	"log/slog"
	"sync"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"
)

// webOwner é o destinatário das respostas do painel "Lançar": só existe para o Handler, nada é enviado de fato.
const webOwner = "dashboard"

// Web atende o painel "Lançar" do dashboard com o mesmo Handler do chat. As respostas, em vez de irem a um canal,
// voltam na própria requisição. A conversa é separada da do Telegram: uma pergunta pendente aqui não atrapalha lá.
type Web struct {
	mu      sync.Mutex // uma mensagem por vez: o Handler guarda perguntas pendentes entre mensagens
	handler *Handler
	out     *captureMessenger
}

func NewWeb(analyzeExpense ledger.ExpenseAnalyzer, csvExporter ledger.CSVExporter, logger *slog.Logger) *Web {
	out := &captureMessenger{}
	return &Web{handler: NewHandler(analyzeExpense, csvExporter, out, webOwner, logger), out: out}
}

// Handler é para ligar os mesmos recursos do chat (notas, saldos, áudio...) antes de atender.
func (w *Web) Handler() *Handler { return w.handler }

// Send processa a mensagem e devolve as respostas, na ordem.
func (w *Web) Send(ctx context.Context, msg Message) []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.out.replies = nil
	w.handler.Handle(ctx, msg) //nolint:errcheck // o Handler já avisa o usuário dos erros pelas respostas
	return w.out.replies
}

type captureMessenger struct{ replies []string }

func (c *captureMessenger) SendText(_ context.Context, _ string, text string) (string, error) {
	c.replies = append(c.replies, text)
	return "", nil
}

// SendDocument: o painel não recebe arquivos (é o CSV pedido no chat); avisa onde baixar.
func (c *captureMessenger) SendDocument(_ context.Context, _, filename string, _ []byte, _ string) (string, error) {
	c.replies = append(c.replies, "📎 O arquivo "+filename+" só sai pelo Telegram. Peça por lá.")
	return "", nil
}

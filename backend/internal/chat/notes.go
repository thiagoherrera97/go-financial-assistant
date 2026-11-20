package chat

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// noteChoiceTTL é por quanto tempo a lista de opções de uma nota espera a resposta com o número.
const noteChoiceTTL = 10 * time.Minute

// noteSearchDays é a janela sem dia informado: as transações com o valor dos últimos dias.
const noteSearchDays = 60

const noteHelp = "📝 Para anotar o que foi uma transação, mande o valor, o dia e a nota:\n" +
	"*nota 5400 dia 10 insulfilm, minha mãe está ajudando*\n" +
	"*nota 393 dia 1/11: jantar pizza com os pais da Isa*\n\n" +
	"Sem o dia, procuro pelo valor nos últimos 60 dias. Se a transação ainda não chegou do banco, guardo a nota e coloco quando ela chegar."

// Noter acha a transação pelo valor e pelo dia e grava a nota.
type Noter interface {
	NoteCandidates(ctx context.Context, amount float64, from, to time.Time) ([]domain.NoteCandidate, error)
	SetTransactionNote(ctx context.Context, id uuid.UUID, note string) (bool, error)
	SavePendingNote(ctx context.Context, amount float64, day time.Time, note string) error
}

// SetNoter habilita o comando "nota".
func (h *Handler) SetNoter(n Noter) { h.noter = n }

type noteChoice struct {
	candidates []domain.NoteCandidate
	note       string
	expiresAt  time.Time
}

// noteRequest é o que a pessoa mandou: valor, dia (nil = não disse) e a nota.
type noteRequest struct {
	amount float64
	day    *time.Time
	note   string
}

var (
	notePrefixRE = regexp.MustCompile(`(?i)^/?nota\b[\s:,-]*`)
	// "dia 10", "dia 10/11", "10/11", "10/11/2026" (o primeiro que aparecer).
	noteDayRE = regexp.MustCompile(`(?i)(?:\bdia\s+(\d{1,2})(?:/(\d{1,2})(?:/(\d{2,4}))?)?\b|\b(\d{1,2})/(\d{1,2})(?:/(\d{2,4}))?\b)`)
	// "5400", "5.400", "5.400,00", "279,89", "R$ 99,99".
	noteAmountRE = regexp.MustCompile(`(?i)(?:r\$\s*)?\b(\d{1,3}(?:\.\d{3})+(?:,\d{1,2})?|\d+(?:[.,]\d{1,2})?)\b`)
)

func isNoteCommand(text string) bool { return notePrefixRE.MatchString(strings.TrimSpace(text)) }

// parseNoteRequest lê "nota <valor> [dia <d>[/<m>[/<a>]]] [:] <nota>". A nota é o resto do texto, sem o valor e o dia;
// com ":", valor e dia só são procurados antes dele (números depois do ":" ficam na nota). today é o dia de hoje (para o mês e o
// ano que faltarem: vale a data mais recente até hoje).
func parseNoteRequest(text string, today time.Time) (noteRequest, bool) {
	body := notePrefixRE.ReplaceAllString(strings.TrimSpace(text), "")
	head, note, hasColon := strings.Cut(body, ":")
	if !hasColon {
		head, note = body, ""
	}

	var req noteRequest
	if m := noteDayRE.FindStringSubmatchIndex(head); m != nil {
		g := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return head[m[2*i]:m[2*i+1]]
		}
		d, mo, y := g(1), g(2), g(3)
		if d == "" {
			d, mo, y = g(4), g(5), g(6)
		}
		day, ok := resolveNoteDay(d, mo, y, today)
		if !ok {
			return req, false
		}
		req.day = &day
		head = head[:m[0]] + " " + head[m[1]:]
	}
	m := noteAmountRE.FindStringSubmatchIndex(head)
	if m == nil {
		return req, false
	}
	raw := head[m[2]:m[3]]
	if strings.Contains(raw, ",") {
		raw = strings.ReplaceAll(strings.ReplaceAll(raw, ".", ""), ",", ".")
	} else if strings.Count(raw, ".") >= 1 && len(raw)-strings.LastIndex(raw, ".") == 4 {
		raw = strings.ReplaceAll(raw, ".", "") // "5.400" é milhar
	}
	amount, err := strconv.ParseFloat(raw, 64)
	if err != nil || amount <= 0 {
		return req, false
	}
	req.amount = amount
	// A nota é o que sobrou antes do ":" (sem valor e dia) mais o que veio depois dele.
	note = strings.Trim(strings.TrimSpace(head[:m[0]]+" "+head[m[1]:]), ",-–— ") + " " + note
	clean, err := domain.NormalizeNote(strings.Trim(strings.TrimSpace(note), ":,-–— "))
	if err != nil || clean == "" {
		return req, false
	}
	req.note = clean
	return req, true
}

func resolveNoteDay(d, mo, y string, today time.Time) (time.Time, bool) {
	day, _ := strconv.Atoi(d)
	month, year := int(today.Month()), today.Year()
	if mo != "" {
		month, _ = strconv.Atoi(mo)
	}
	if y != "" {
		year, _ = strconv.Atoi(y)
		if year < 100 {
			year += 2000
		}
	}
	if day < 1 || day > 31 || month < 1 || month > 12 {
		return time.Time{}, false
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Day() != day { // 31/02
		return time.Time{}, false
	}
	todayUTC := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if y == "" && t.After(todayUTC) {
		if mo == "" {
			t = t.AddDate(0, -1, 0)
		} else {
			t = t.AddDate(-1, 0, 0)
		}
	}
	return t, true
}

func (h *Handler) handleNote(ctx context.Context, text string) {
	if h.noter == nil {
		h.sendText(ctx, "ℹ️ As notas não estão disponíveis.")
		return
	}
	today := time.Now()
	req, ok := parseNoteRequest(text, today)
	if !ok {
		h.sendText(ctx, noteHelp)
		return
	}
	todayUTC := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	from, to := todayUTC.AddDate(0, 0, -noteSearchDays), todayUTC
	if req.day != nil {
		from, to = *req.day, *req.day
	}
	found, err := h.noter.NoteCandidates(ctx, req.amount, from, to)
	if err == nil && len(found) == 0 && req.day != nil {
		// O banco às vezes data um dia antes ou depois (compra à noite, Pix agendado).
		found, err = h.noter.NoteCandidates(ctx, req.amount, req.day.AddDate(0, 0, -domain.PendingNoteMatchDays), req.day.AddDate(0, 0, domain.PendingNoteMatchDays))
	}
	if err != nil {
		h.logger.Error("erro ao procurar a transação da nota", "error", err)
		h.sendText(ctx, "Não consegui procurar a transação agora. Tente de novo em instantes.")
		return
	}

	switch len(found) {
	case 0:
		day := todayUTC
		if req.day != nil {
			day = *req.day
		}
		if err := h.noter.SavePendingNote(ctx, req.amount, day, req.note); err != nil {
			h.logger.Error("erro ao guardar nota pendente", "error", err)
			h.sendText(ctx, "Não achei a transação e também não consegui guardar a nota. Tente de novo em instantes.")
			return
		}
		h.sendText(ctx, fmt.Sprintf("🕓 Ainda não achei uma transação de %s em %s. Guardei a nota e coloco quando o banco mandar "+
			"(até %d dias de diferença):\n_%s_", brl(req.amount), day.Format("02/01"), domain.PendingNoteMatchDays, req.note))
	case 1:
		h.applyNote(ctx, found[0], req.note)
	default:
		if len(found) > 5 {
			found = found[:5]
		}
		h.mu.Lock()
		h.noteChoice = &noteChoice{candidates: found, note: req.note, expiresAt: time.Now().Add(noteChoiceTTL)}
		h.mu.Unlock()
		var b strings.Builder
		fmt.Fprintf(&b, "Achei %d transações de %s. Qual é?\n", len(found), brl(req.amount))
		for i, c := range found {
			fmt.Fprintf(&b, "\n*%d)* %s · %s%s", i+1, c.Date.Format("02/01"), c.Description, accountSuffix(c.AccountName))
		}
		b.WriteString("\n\nResponda com o número.")
		h.sendText(ctx, b.String())
	}
}

// tryHandleNoteChoice trata a resposta com o número quando há uma lista de opções de nota esperando.
func (h *Handler) tryHandleNoteChoice(ctx context.Context, text string) bool {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return false
	}
	h.mu.Lock()
	choice := h.noteChoice
	if choice == nil || time.Now().After(choice.expiresAt) {
		h.noteChoice = nil
		h.mu.Unlock()
		return false
	}
	if n < 1 || n > len(choice.candidates) {
		h.mu.Unlock()
		h.sendText(ctx, fmt.Sprintf("Responda com um número de 1 a %d.", len(choice.candidates)))
		return true
	}
	h.noteChoice = nil
	h.mu.Unlock()
	h.applyNote(ctx, choice.candidates[n-1], choice.note)
	return true
}

func (h *Handler) applyNote(ctx context.Context, c domain.NoteCandidate, note string) {
	ok, err := h.noter.SetTransactionNote(ctx, c.PaymentID, note)
	if err != nil || !ok {
		h.logger.Error("erro ao gravar a nota", "error", err)
		h.sendText(ctx, "Não consegui gravar a nota. Tente de novo em instantes.")
		return
	}
	replaced := ""
	if c.Note != "" && c.Note != note {
		replaced = "\n(substituí a nota anterior: " + c.Note + ")"
	}
	h.sendText(ctx, fmt.Sprintf("📝 Nota salva em *%s* (%s, %s)%s:\n_%s_%s", c.Description, brl(abs(c.Amount)), c.Date.Format("02/01"),
		accountSuffix(c.AccountName), note, replaced))
}

func accountSuffix(name string) string {
	if name == "" {
		return ""
	}
	return " · " + name
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// brl formata em reais com vírgula decimal e ponto de milhar.
func brl(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	intPart, dec, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return "R$ " + b.String() + "," + dec
}

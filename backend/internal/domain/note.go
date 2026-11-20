package domain

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// MaxNoteLen é o tamanho máximo da nota de uma transação, em caracteres.
const MaxNoteLen = 300

var ErrNoteTooLong = errors.New("a nota passa de 300 caracteres")

// NormalizeNote limpa a nota: sem caracteres de controle, espaços repetidos nem espaços nas pontas. Vazia apaga a nota.
func NormalizeNote(raw string) (string, error) {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, raw)
	clean = strings.Join(strings.Fields(clean), " ")
	if utf8.RuneCountInString(clean) > MaxNoteLen {
		return "", ErrNoteTooLong
	}
	return clean, nil
}

// NoteCandidate é uma transação que pode ser a que a pessoa descreveu pelo chat (valor e dia).
type NoteCandidate struct {
	PaymentID   uuid.UUID
	Date        time.Time
	Description string
	Amount      float64 // como gravado (estorno é negativo)
	Kind        string
	AccountName string
	Note        string
}

// PendingNoteMatchDays é a folga, em dias, entre o dia dito e o da transação que o banco mandar depois.
const PendingNoteMatchDays = 3

// PendingNoteMaxAge é por quanto tempo a nota pendente espera a transação chegar.
const PendingNoteMaxAge = 60 * 24 * time.Hour

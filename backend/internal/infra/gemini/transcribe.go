package gemini

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// transcribePrompt pede a fala em texto, com valores e datas em algarismos: os comandos do chat (como "nota") procuram números.
const transcribePrompt = "Transcreva a fala deste áudio em português do Brasil, fielmente, como uma mensagem de chat.\n" +
	"Regras:\n" +
	"- Escreva valores em algarismos, com vírgula nos centavos: \"trezentos e noventa e três reais\" vira \"393\"; " +
	"\"doze e cinquenta\" vira \"12,50\"; \"cinco mil e quatrocentos\" vira \"5400\".\n" +
	"- Escreva datas como \"dia D\" ou \"dia D/M\": \"dia primeiro de novembro\" vira \"dia 1/11\"; \"dia dez\" vira \"dia 10\".\n" +
	"- Se a pessoa começar com \"nota\" ou \"anota\", comece o texto com \"nota\".\n" +
	"- Devolva só o texto transcrito, sem aspas nem comentários. Se não houver fala, devolva vazio."

// transcribeConfig é sem o esquema JSON da análise de despesas: a resposta é texto puro.
var transcribeConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0)}

// Transcribe transforma um áudio (voz do Telegram, por exemplo) em texto.
func (c *Client) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
	contents := []*genai.Content{{
		Role: "user",
		Parts: []*genai.Part{
			{Text: transcribePrompt},
			{InlineData: &genai.Blob{MIMEType: mimeType, Data: audio}},
		},
	}}
	resp, err := c.generate(ctx, modelName, contents, transcribeConfig)
	if err != nil {
		return "", fmt.Errorf("erro ao transcrever áudio: %w", err)
	}
	return strings.Trim(strings.TrimSpace(resp.Text()), `"“”`), nil
}

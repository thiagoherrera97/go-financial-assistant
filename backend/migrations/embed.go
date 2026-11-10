// Package migrations embute os scripts SQL no binário: o app os aplica ao subir (internal/infra/db/migrate). O initdb do
// Postgres também lê esta pasta na criação do banco e ignora este arquivo .go.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

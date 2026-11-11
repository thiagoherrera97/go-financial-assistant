package migrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakePgDump(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pg_dump")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

const testURL = "postgres://finassist:s3gredo-xyz@db.local:5433/finassist?sslmode=disable"

func TestPgDump_WritesPrivateFileAndKeepsPasswordOutOfArgs(t *testing.T) {
	dir, rec := t.TempDir(), filepath.Join(t.TempDir(), "rec")
	bin := fakePgDump(t, `echo "$@" > `+rec+`; echo "$PGPASSWORD|$PGHOST|$PGPORT|$PGUSER|$PGDATABASE|$PGSSLMODE" >> `+rec+`; echo DUMPDATA`)
	b := PgDump{Bin: bin, Dir: dir, URL: testURL, Now: func() time.Time { return time.Date(2026, 11, 10, 15, 4, 5, 0, time.UTC) }}

	path, err := b.Run(context.Background(), "abc12345")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "pre-migration-20261110-150405-abc12345.dump"); path != want {
		t.Errorf("caminho: %s, esperado %s", path, want)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("permissão deveria ser 600: %v %v", info, err)
	}
	if data, _ := os.ReadFile(path); strings.TrimSpace(string(data)) != "DUMPDATA" {
		t.Errorf("conteúdo: %q", data)
	}
	got, _ := os.ReadFile(rec)
	args, env, _ := strings.Cut(string(got), "\n")
	if strings.Contains(args, "s3gredo") || strings.Contains(args, "postgres://") {
		t.Errorf("a senha/URL não pode ir na linha de comando: %q", args)
	}
	if !strings.Contains(args, "-Fc") || !strings.Contains(args, "--no-owner") {
		t.Errorf("argumentos: %q", args)
	}
	if strings.TrimSpace(env) != "s3gredo-xyz|db.local|5433|finassist|finassist|disable" {
		t.Errorf("ambiente do pg_dump: %q", env)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".pre-migration-*")); len(left) != 0 {
		t.Errorf("sobrou arquivo temporário: %v", left)
	}
}

func TestPgDump_FailureLeavesNothingAndNeverLeaksPassword(t *testing.T) {
	dir := t.TempDir()
	bin := fakePgDump(t, `echo "falha ao conectar com senha $PGPASSWORD" >&2; echo parcial; exit 1`)
	_, err := PgDump{Bin: bin, Dir: dir, URL: testURL}.Run(context.Background(), "abc12345")
	if err == nil {
		t.Fatal("esperava erro")
	}
	if strings.Contains(err.Error(), "s3gredo") {
		t.Errorf("a senha vazou no erro: %v", err)
	}
	if !strings.Contains(err.Error(), "***") {
		t.Errorf("a senha deveria aparecer mascarada: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("a falha não pode deixar arquivo: %v", entries)
	}
}

func TestPgDump_EmptyOutputIsAFailure(t *testing.T) {
	dir := t.TempDir()
	if _, err := (PgDump{Bin: fakePgDump(t, `true`), Dir: dir, URL: testURL}).Run(context.Background(), "abc12345"); err == nil {
		t.Error("backup vazio não vale")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("sobrou arquivo: %v", entries)
	}
}

func TestPgDump_MissingBinaryAndBadURL(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // sem pg_dump no PATH
	if _, err := (PgDump{Dir: t.TempDir(), URL: testURL}).Run(context.Background(), "abc12345"); err == nil || !strings.Contains(err.Error(), "pg_dump não encontrado") {
		t.Errorf("deveria explicar que falta o pg_dump: %v", err)
	}
	if _, err := (PgDump{Bin: "/bin/true", Dir: t.TempDir(), URL: "::não é url"}).Run(context.Background(), "abc12345"); err == nil {
		t.Error("URL inválida deveria falhar")
	}
}

func dumps(t *testing.T, dir string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(dir, "pre-migration-*.dump"))
	return m
}

func TestPgDump_ReusesTheBackupOfTheSameTagAndMakesANewOneForAnotherTag(t *testing.T) {
	dir := t.TempDir()
	calls := filepath.Join(t.TempDir(), "calls")
	bin := fakePgDump(t, `echo x >> `+calls+`; echo DUMPDATA`)
	clock := time.Now().Truncate(time.Second)
	b := PgDump{Bin: bin, Dir: dir, URL: testURL, Now: func() time.Time { clock = clock.Add(time.Second); return clock }}

	first, err := b.Run(context.Background(), "aaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ { // reinícios em laço: mesma tag
		again, err := b.Run(context.Background(), "aaaaaaaa")
		if err != nil || again != first {
			t.Fatalf("deveria reaproveitar %s, got %s %v", first, again, err)
		}
	}
	if n := len(dumps(t, dir)); n != 1 {
		t.Errorf("5 subidas com o mesmo conjunto devem deixar 1 backup, got %d", n)
	}
	if data, _ := os.ReadFile(calls); strings.Count(string(data), "x") != 1 {
		t.Errorf("o pg_dump deveria rodar uma vez: %q", data)
	}

	other, err := b.Run(context.Background(), "bbbbbbbb") // migration corrigida: outro checksum, outra tag
	if err != nil || other == first || len(dumps(t, dir)) != 2 {
		t.Errorf("tag nova deveria gerar backup novo: %s %v %v", other, err, dumps(t, dir))
	}
}

func TestPgDump_InterruptedTmpIsNeverReused(t *testing.T) {
	dir := t.TempDir()
	// Sobra de uma execução interrompida (e um .dump vazio, que também não vale).
	os.WriteFile(filepath.Join(dir, ".pre-migration-20261110-100000-aaaaaaaa.tmp"), []byte("parcial"), 0o600)
	os.WriteFile(filepath.Join(dir, "pre-migration-20261110-100100-aaaaaaaa.dump"), nil, 0o600)
	bin := fakePgDump(t, `echo DUMPDATA`)

	path, err := PgDump{Bin: bin, Dir: dir, URL: testURL}.Run(context.Background(), "aaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); strings.TrimSpace(string(data)) != "DUMPDATA" {
		t.Errorf("deveria ter feito um backup novo, conteúdo: %q", data)
	}
	if strings.Contains(path, "100100") {
		t.Errorf("o .dump vazio foi reutilizado: %s", path)
	}
}

func TestPgDump_InvalidTag(t *testing.T) {
	if _, err := (PgDump{Bin: "/bin/true", Dir: t.TempDir(), URL: testURL}).Run(context.Background(), "../x"); err == nil {
		t.Error("tag com caminho deveria ser recusada")
	}
}

func touch(t *testing.T, path string, age time.Duration) {
	t.Helper()
	mt := time.Now().Add(-age)
	if err := os.Chtimes(path, mt, mt); err != nil {
		t.Fatal(err)
	}
}

func TestPgDump_ReuseOnlyWhenRecentAndRemovesOldOfTheSameTag(t *testing.T) {
	dir := t.TempDir()
	bin := fakePgDump(t, `echo NEWDUMP`)
	write := func(name string, age time.Duration) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("OLD"), 0o600); err != nil {
			t.Fatal(err)
		}
		touch(t, p, age)
		return p
	}
	b := PgDump{Bin: bin, Dir: dir, URL: testURL, Logger: nil}

	// Mtime de 10 minutos: reaproveitado, nada novo.
	recent := write("pre-migration-20261110-100000-aaaaaaaa.dump", 10*time.Minute)
	got, err := b.Run(context.Background(), "aaaaaaaa")
	if err != nil || got != recent || len(dumps(t, dir)) != 1 {
		t.Fatalf("dump de 10 min deveria ser reaproveitado: %s %v %v", got, err, dumps(t, dir))
	}

	// Mtime de 2 horas da mesma tag: backup novo, o antigo sai; as outras tags ficam.
	touch(t, recent, 2*time.Hour)
	otherTag := write("pre-migration-20261101-100000-bbbbbbbb.dump", 48*time.Hour)
	second := write("pre-migration-20261105-100000-aaaaaaaa.dump", 5*time.Hour)
	got, err = b.Run(context.Background(), "aaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if got == recent || got == second {
		t.Fatalf("deveria ter feito um backup novo: %s", got)
	}
	if data, _ := os.ReadFile(got); strings.TrimSpace(string(data)) != "NEWDUMP" {
		t.Errorf("conteúdo do novo: %q", data)
	}
	for _, gone := range []string{recent, second} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("o dump antigo da mesma tag deveria ter sido apagado: %s", gone)
		}
	}
	if _, err := os.Stat(otherTag); err != nil {
		t.Errorf("dump de outra tag nunca pode ser apagado: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "pre-migration-*-aaaaaaaa.dump")); len(left) != 1 || left[0] != got {
		t.Errorf("deveria sobrar só o novo da tag: %v", left)
	}
}

func TestPgDump_FailedNewBackupKeepsTheOldOne(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "pre-migration-20261105-100000-aaaaaaaa.dump")
	os.WriteFile(old, []byte("OLD"), 0o600)
	touch(t, old, 5*time.Hour)
	if _, err := (PgDump{Bin: fakePgDump(t, `exit 1`), Dir: dir, URL: testURL}).Run(context.Background(), "aaaaaaaa"); err == nil {
		t.Fatal("esperava erro")
	}
	if _, err := os.Stat(old); err != nil {
		t.Errorf("se o backup novo falha, o antigo fica: %v", err)
	}
}

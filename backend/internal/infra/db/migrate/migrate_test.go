package migrate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// newDB cria um banco novo e vazio no Postgres de teste (TEST_DATABASE_URL) e o apaga no fim.
func newDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL não definida")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("h5_test_%d", rand.Int63())
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+name); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(adminURL)
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		admin.Exec(ctx, `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`)
		admin.Close()
	})
	return pool, u.String()
}

func realFiles(t *testing.T) []Migration {
	t.Helper()
	files, err := Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// initdbLike faz o que o initdb do Postgres faz: executa os scripts em ordem, sem tabela de controle.
func initdbLike(t *testing.T, pool *pgxpool.Pool, files []Migration) {
	t.Helper()
	for _, m := range files {
		if _, err := pool.Exec(context.Background(), m.SQL); err != nil {
			t.Fatalf("%s: %v", m.Version, err)
		}
	}
}

func applied(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		rows.Scan(&v)
		out = append(out, v)
	}
	return out
}

// schemaSnapshot descreve o esquema (colunas, constraints e índices) para comparar antes e depois.
func schemaSnapshot(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var lines []string
	collect := func(query string) {
		rows, err := pool.Query(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var s string
			rows.Scan(&s)
			lines = append(lines, s)
		}
	}
	collect(`SELECT 'col ' || table_name || '.' || column_name || ' ' || data_type || ' null=' || is_nullable || ' def=' || COALESCE(column_default, '')
	         FROM information_schema.columns WHERE table_schema = 'public' AND table_name <> 'schema_migrations'`)
	collect(`SELECT 'con ' || conrelid::regclass || ' ' || conname || ' ' || pg_get_constraintdef(oid)
	         FROM pg_constraint WHERE connamespace = 'public'::regnamespace AND conrelid::regclass::text <> 'schema_migrations'`)
	collect(`SELECT 'idx ' || indexdef FROM pg_indexes WHERE schemaname = 'public' AND tablename <> 'schema_migrations'`)
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// dataSnapshot devolve todas as linhas de todas as tabelas (menos schema_migrations), em JSON ordenado.
func dataSnapshot(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'schema_migrations' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		tables = append(tables, n)
	}
	rows.Close()
	var b strings.Builder
	for _, name := range tables {
		var dump string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text), '[]')::text FROM %q x`, name)).Scan(&dump); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s: %s\n", name, dump)
	}
	return b.String()
}

func seed(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	stmts := []string{
		`INSERT INTO purchases (id, description, category, type, total_amount, raw_input, kind, transfer_direction) VALUES
			('00000000-0000-0000-0000-000000000001', 'Mercado', 'MARKET', 'SINGLE', 42.50, 'mercado 42,50', 'EXPENSE', NULL),
			('00000000-0000-0000-0000-000000000002', 'Salário', 'SALARY', 'RECURRING', 6000, 'salário', 'INCOME', NULL),
			('00000000-0000-0000-0000-000000000003', 'Aplicação', 'INVESTMENT', 'SINGLE', 500, 'aplicação', 'TRANSFER', 'OUT')`,
		`INSERT INTO accounts (id, external_id, item_id, type, name, last4, balance) VALUES
			('00000000-0000-0000-0000-0000000000a1', 'acc-1', 'item-1', 'BANK', 'Conta', '1234', 100.10)`,
		`INSERT INTO payments (id, purchase_id, amount, status, due_date, external_id, account_id) VALUES
			('00000000-0000-0000-0000-0000000000b1', '00000000-0000-0000-0000-000000000001', 42.50, 'PAID', '2026-11-03', 'ext-1', '00000000-0000-0000-0000-0000000000a1'),
			('00000000-0000-0000-0000-0000000000b2', '00000000-0000-0000-0000-000000000002', 6000, 'PAID', NULL, NULL, NULL)`,
		`INSERT INTO category_rules (key, category) VALUES ('mercado', 'MARKET'), ('pix fulano', 'OTHER')`,
		`INSERT INTO expense_rules (key, class) VALUES ('netflix', 'FIXED')`,
		`INSERT INTO settings (key, value, secret) VALUES ('DIGEST_HOUR', '9', FALSE)`,
		`INSERT INTO settings_audit (action, key, sensitive, ip) VALUES ('set', 'DIGEST_HOUR', FALSE, '127.0.0.1'), ('setup', 'SETUP', TRUE, '')`,
		`INSERT INTO goals (id, kind, name, target_amount, target_date) VALUES ('00000000-0000-0000-0000-0000000000c1', 'SAVE', 'Reserva', 1000, '2027-01-01')`,
		`INSERT INTO dashboard_owner (id, password_hash, setup_completed_at) VALUES (1, '$argon2id$v=19$x', NOW())`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(context.Background(), s); err != nil {
			t.Fatalf("seed: %v\n%s", err, s)
		}
	}
}

func TestLoad_EmbeddedMigrations(t *testing.T) {
	files := realFiles(t)
	if len(files) == 0 || !strings.HasPrefix(files[0].Version, "001_") {
		t.Fatalf("a primeira migration deveria ser a 001_..., got %d arquivos", len(files))
	}
	for i := 1; i < len(files); i++ {
		if files[i-1].Version >= files[i].Version {
			t.Errorf("fora de ordem: %s antes de %s", files[i-1].Version, files[i].Version)
		}
	}
	for _, m := range files {
		if m.Checksum == "" || m.SQL == "" {
			t.Errorf("%s sem conteúdo ou checksum", m.Version)
		}
	}
}

func TestLoad_RejectsConcurrentlyButAllowsItInComments(t *testing.T) {
	if _, err := Load(fstest.MapFS{"001_x.sql": {Data: []byte("CREATE INDEX CONCURRENTLY i ON t(a);")}}); err == nil || !strings.Contains(err.Error(), "001_x.sql") {
		t.Errorf("CONCURRENTLY deveria ser recusado com o nome do arquivo: %v", err)
	}
	ok := fstest.MapFS{"001_x.sql": {Data: []byte("-- nada de CONCURRENTLY aqui\n/* nem CONCURRENTLY aqui */\nSELECT 1;")}}
	if _, err := Load(ok); err != nil {
		t.Errorf("comentário não deveria contar: %v", err)
	}
}

func TestRun_EmptyDatabaseAppliesAllAndSecondRunAppliesNone(t *testing.T) {
	pool, _ := newDB(t)
	backups := 0
	files := realFiles(t)
	r := &Runner{Pool: pool, Files: files, Logger: quiet(), Backup: func(context.Context, string) (string, error) { backups++; return "x", nil }}

	res, err := r.Run(context.Background())
	if err != nil || len(res.Applied) != len(files) {
		t.Fatalf("banco vazio: %+v %v", res, err)
	}
	if backups != 0 {
		t.Error("banco vazio não precisa de backup")
	}
	if got := applied(t, pool); len(got) != len(files) {
		t.Errorf("deveria registrar as %d versões, got %d", len(files), len(got))
	}
	res, err = r.Run(context.Background())
	if err != nil || len(res.Applied) != 0 || backups != 0 {
		t.Errorf("segunda subida não aplica nada nem faz backup: %+v %v backups=%d", res, err, backups)
	}
}

func TestRun_InitdbDatabaseWithDataIsRecognizedAndDataIsUntouched(t *testing.T) {
	pool, _ := newDB(t)
	files := realFiles(t)
	initdbLike(t, pool, files) // o que o initdb faz: sem schema_migrations
	seed(t, pool)

	dataBefore, schemaBefore := dataSnapshot(t, pool), schemaSnapshot(t, pool)
	var order []string
	r := &Runner{Pool: pool, Files: files, Logger: quiet(), Backup: func(context.Context, string) (string, error) {
		order = append(order, "backup")
		return "x", nil
	}}
	res, err := r.Run(context.Background())
	if err != nil || len(res.Applied) != len(files) {
		t.Fatalf("primeira subida num banco do initdb: %+v %v", res, err)
	}
	if len(order) != 1 || res.Backup != "x" {
		t.Error("deveria fazer um backup antes de aplicar")
	}
	if dataAfter := dataSnapshot(t, pool); dataAfter != dataBefore {
		t.Errorf("os dados mudaram:\nantes:\n%s\ndepois:\n%s", dataBefore, dataAfter)
	}
	if schemaAfter := schemaSnapshot(t, pool); schemaAfter != schemaBefore {
		t.Error("o esquema mudou na primeira subida")
	}
	if got := applied(t, pool); len(got) != len(files) {
		t.Errorf("deveria registrar as %d versões, got %d", len(files), len(got))
	}
}

func TestMigrations_AreIdempotent(t *testing.T) {
	pool, _ := newDB(t)
	files := realFiles(t)
	initdbLike(t, pool, files)
	once := schemaSnapshot(t, pool)
	initdbLike(t, pool, files)
	initdbLike(t, pool, files)
	if twice := schemaSnapshot(t, pool); twice != once {
		t.Error("reaplicar mudou o esquema")
	}
}

// Só roda com Docker e um container Postgres de teste (TEST_PG_CONTAINER): compara o `pg_dump --data-only` antes e depois.
func TestRun_PgDumpDataOnlyIdenticalBeforeAndAfterFirstBoot(t *testing.T) {
	container := os.Getenv("TEST_PG_CONTAINER")
	if container == "" {
		t.Skip("TEST_PG_CONTAINER não definida")
	}
	pool, dbURL := newDB(t)
	u, _ := url.Parse(dbURL)
	dbName := strings.TrimPrefix(u.Path, "/")
	dump := func() string {
		out, err := exec.Command("docker", "exec", container, "pg_dump", "-U", "t", "-d", dbName, "--data-only", "--exclude-table=schema_migrations").CombinedOutput()
		if err != nil {
			t.Fatalf("pg_dump: %v\n%s", err, out)
		}
		// O pg_dump recente escreve um token aleatório em \restrict/\unrestrict a cada execução; não é dado.
		var kept []string
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.HasPrefix(line, `\restrict `) && !strings.HasPrefix(line, `\unrestrict `) {
				kept = append(kept, line)
			}
		}
		return strings.Join(kept, "\n")
	}
	files := realFiles(t)
	initdbLike(t, pool, files)
	seed(t, pool)
	before := dump()
	if _, err := (&Runner{Pool: pool, Files: files, Logger: quiet()}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if after := dump(); after != before {
		t.Errorf("o pg_dump --data-only mudou depois da primeira subida:\n%s", firstDiff(before, after))
	}
	if !strings.Contains(before, "Mercado") {
		t.Error("o dump deveria conter os dados de exemplo")
	}
}

func TestRun_FailureRollsBackAndStops(t *testing.T) {
	pool, _ := newDB(t)
	files, err := Load(fstest.MapFS{
		"001_ok.sql":   {Data: []byte("CREATE TABLE a (id INT);")},
		"002_bad.sql":  {Data: []byte("CREATE TABLE b (id INT); SELECT 1/0;")},
		"003_next.sql": {Data: []byte("CREATE TABLE c (id INT);")},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := (&Runner{Pool: pool, Files: files, Logger: quiet()}).Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "002_bad") || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("erro deveria citar o arquivo e a mensagem do Postgres: %v", err)
	}
	if len(res.Applied) != 1 {
		t.Errorf("só a 001 deveria ter sido aplicada: %+v", res)
	}
	if got := applied(t, pool); len(got) != 1 || got[0] != "001_ok" {
		t.Errorf("só a 001 registrada, got %v", got)
	}
	var b, c *string
	pool.QueryRow(context.Background(), `SELECT to_regclass('b')::text, to_regclass('c')::text`).Scan(&b, &c)
	if b != nil || c != nil {
		t.Error("a 002 deveria ter feito rollback inteiro e a 003 não deveria ter rodado")
	}
}

func TestRun_ConcurrentRunnersApplyEachMigrationOnce(t *testing.T) {
	pool, _ := newDB(t)
	// CREATE TABLE sem IF NOT EXISTS: aplicar duas vezes falharia.
	mfs := fstest.MapFS{}
	for i := 1; i <= 5; i++ {
		mfs[fmt.Sprintf("%03d_t.sql", i)] = &fstest.MapFile{Data: []byte(fmt.Sprintf("CREATE TABLE t%d (id INT); SELECT pg_sleep(0.05);", i))}
	}
	files, _ := Load(mfs)

	var wg sync.WaitGroup
	results := make([]Result, 4)
	errs := make([]error, 4)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = (&Runner{Pool: pool, Files: files, Logger: quiet()}).Run(context.Background())
		}()
	}
	wg.Wait()
	total := 0
	for i := range results {
		if errs[i] != nil {
			t.Fatalf("runner %d: %v", i, errs[i])
		}
		total += len(results[i].Applied)
	}
	if total != 5 {
		t.Errorf("as 5 migrations deveriam ser aplicadas uma única vez no total, got %d", total)
	}
}

func TestRun_BackupFailureAppliesNothing(t *testing.T) {
	pool, _ := newDB(t)
	if _, err := pool.Exec(context.Background(), `CREATE TABLE existing (id INT)`); err != nil { // banco com dados: exige backup
		t.Fatal(err)
	}
	files, _ := Load(fstest.MapFS{"001_t.sql": {Data: []byte("CREATE TABLE t1 (id INT);")}})
	r := &Runner{Pool: pool, Files: files, Logger: quiet(), Backup: func(context.Context, string) (string, error) { return "", errors.New("sem espaço") }}

	res, err := r.Run(context.Background())
	if !errors.Is(err, ErrBackup) || len(res.Applied) != 0 {
		t.Fatalf("deveria falhar com ErrBackup sem aplicar nada: %+v %v", res, err)
	}
	var t1 *string
	pool.QueryRow(context.Background(), `SELECT to_regclass('t1')::text`).Scan(&t1)
	if t1 != nil || len(applied(t, pool)) != 0 {
		t.Error("nada deveria ter sido aplicado nem registrado")
	}
}

func TestRun_BackupHappensBeforeAnyMigration(t *testing.T) {
	pool, _ := newDB(t)
	pool.Exec(context.Background(), `CREATE TABLE existing (id INT)`)
	files, _ := Load(fstest.MapFS{"001_t.sql": {Data: []byte("CREATE TABLE t1 (id INT);")}})
	var existedAtBackup *string
	r := &Runner{Pool: pool, Files: files, Logger: quiet(), Backup: func(ctx context.Context, _ string) (string, error) {
		pool.QueryRow(ctx, `SELECT to_regclass('t1')::text`).Scan(&existedAtBackup)
		return "b.dump", nil
	}}
	res, err := r.Run(context.Background())
	if err != nil || len(res.Applied) != 1 || res.Backup != "b.dump" {
		t.Fatalf("%+v %v", res, err)
	}
	if existedAtBackup != nil {
		t.Error("o backup deveria rodar antes de aplicar a migration")
	}
}

func TestRun_ChangedAppliedMigrationOnlyWarns(t *testing.T) {
	pool, _ := newDB(t)
	v1, _ := Load(fstest.MapFS{"001_t.sql": {Data: []byte("CREATE TABLE t1 (id INT);")}})
	if _, err := (&Runner{Pool: pool, Files: v1, Logger: quiet()}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	v2, _ := Load(fstest.MapFS{"001_t.sql": {Data: []byte("CREATE TABLE t1 (id INT); CREATE TABLE t2 (id INT);")}})
	res, err := (&Runner{Pool: pool, Files: v2, Logger: quiet()}).Run(context.Background())
	if err != nil || len(res.Applied) != 0 {
		t.Errorf("checksum diferente só avisa, sem reaplicar: %+v %v", res, err)
	}
}

var _ fs.FS = migrations.FS

func firstDiff(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(al) && i < len(bl); i++ {
		if al[i] != bl[i] {
			return fmt.Sprintf("linha %d:\n- %s\n+ %s", i+1, al[i], bl[i])
		}
	}
	return fmt.Sprintf("tamanhos diferentes: %d x %d linhas", len(al), len(bl))
}

func TestPendingTag_ChangesWithTheChecksumAndIgnoresOrder(t *testing.T) {
	a, _ := Load(fstest.MapFS{"001_a.sql": {Data: []byte("SELECT 1;")}, "002_b.sql": {Data: []byte("SELECT 2;")}})
	b, _ := Load(fstest.MapFS{"001_a.sql": {Data: []byte("SELECT 1;")}, "002_b.sql": {Data: []byte("SELECT 22;")}})
	if len(PendingTag(a)) != 8 || PendingTag(a) != PendingTag(a) {
		t.Fatal("tag deveria ter 8 caracteres e ser estável")
	}
	if PendingTag(a) == PendingTag(b) {
		t.Error("migration corrigida (outro checksum) deveria mudar a tag")
	}
	if PendingTag([]Migration{a[1], a[0]}) != PendingTag(a) {
		t.Error("a ordem de entrada não deveria importar")
	}
}

// Migration que sempre falha + app que reinicia: 5 subidas seguidas geram 1 só backup; corrigir a migration gera um novo.
func TestRun_RestartLoopWithFailingMigrationMakesOneBackup(t *testing.T) {
	pool, dbURL := newDB(t)
	pool.Exec(context.Background(), `CREATE TABLE existing (id INT)`) // banco com dados: exige backup
	dir := t.TempDir()
	bin := fakePgDump(t, `echo DUMPDATA`)
	run := func(sql string) error {
		files, _ := Load(fstest.MapFS{"001_bad.sql": {Data: []byte(sql)}})
		r := &Runner{Pool: pool, Files: files, Logger: quiet(), Backup: PgDump{Bin: bin, Dir: dir, URL: dbURL}.Run}
		_, err := r.Run(context.Background())
		return err
	}
	for i := 0; i < 5; i++ {
		if err := run("SELECT 1/0;"); err == nil || errors.Is(err, ErrBackup) {
			t.Fatalf("subida %d: esperava erro da migration, got %v", i, err)
		}
		time.Sleep(1100 * time.Millisecond) // o nome leva segundos: sem o reaproveitamento cada subida criaria um arquivo
	}
	if n := len(dumps(t, dir)); n != 1 {
		t.Fatalf("5 subidas falhando deveriam gerar 1 backup, got %d", n)
	}
	if err := run("SELECT 1/0; -- corrigida? não: outro checksum"); err == nil {
		t.Fatal("ainda falha")
	}
	if n := len(dumps(t, dir)); n != 2 {
		t.Errorf("outro checksum deveria gerar um backup novo, got %d", n)
	}
	if err := run("SELECT 1;"); err != nil {
		t.Fatalf("migration corrigida deveria aplicar: %v", err)
	}
}

// Se o unlock falha numa conexão viva, a conexão é fechada em vez de voltar ao pool com o lock da sessão.
func TestRelease_UnlockFailureClosesTheConnection(t *testing.T) {
	pool, _ := newDB(t)
	ctx := context.Background()
	r := &Runner{Pool: pool, Logger: quiet()}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext($1))`, lockName); err != nil {
		t.Fatal(err)
	}
	// O unlock falha de verdade: a transação em aberto aborta e o Exec seguinte devolve erro numa conexão ainda viva.
	if _, err := conn.Exec(ctx, `BEGIN`); err != nil {
		t.Fatal(err)
	}
	conn.Exec(ctx, `SELECT 1/0`) // deixa a transação abortada
	before := pool.Stat().TotalConns()
	r.release(ctx, conn, true)

	if pool.Stat().AcquiredConns() != 0 {
		t.Error("a conexão não deveria continuar emprestada")
	}
	if got := pool.Stat().TotalConns(); got >= before {
		t.Errorf("a conexão deveria ter sido fechada, não devolvida ao pool: %d -> %d", before, got)
	}
	// O lock foi solto junto com a sessão: outro runner consegue tomá-lo.
	other, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Release()
	var got bool
	if err := other.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, lockName).Scan(&got); err != nil || !got {
		t.Errorf("o lock deveria estar livre: %v %v", got, err)
	}
}

package repositories_test

// DB-09 (Lote 39) — integração (INTEGRATION=1 + DB_*; senão t.Skip) da migração
// sql/27: senha_historico.created_at passa a NOT NULL sem tocar nos dados.
//
// O teste troca a definição da coluna, então só roda em banco no loopback (o
// descartável): no CasaOS ele pula, porque o levantamento das linhas NULL e a
// migração de lá são decisão do responsável pelo banco. O SQL vem dos próprios
// arquivos sql/27_*, nunca de uma cópia aqui. Dados: um usuário temporário
// (e-mail zz-test-db09-<n>@teste.local); o Cleanup apaga as linhas e deixa a
// coluna NOT NULL. Nenhum literal deste arquivo usa o relógio do servidor
// (TST-23).

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/testkit/integracao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	db09Abortou      = "DB-09: migracao 27 abortada"
	db09ErroSignal   = uint16(1644)
	db09TempoCliente = 30 * time.Second
	db09ClienteBin   = "mariadb"
	db09ArquivoAlter = "27_alter_senha_historico_created_at_not_null.sql"
	db09ArquivoVolta = "27_revert_senha_historico_created_at_not_null.sql"
)

// db09Criado é o created_at da linha preenchida, conferido depois do aborto.
var db09Criado = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

// db09ColunaAnulavel lê o IS_NULLABLE de senha_historico.created_at.
func db09ColunaAnulavel(t *testing.T, db *sql.DB) bool {
	t.Helper()
	var anulavel string
	require.NoError(t, db.QueryRow(`SELECT IS_NULLABLE FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'senha_historico' AND COLUMN_NAME = 'created_at'`).Scan(&anulavel))
	return anulavel == "YES"
}

func db09LerSQL(t *testing.T, arquivo string) string {
	t.Helper()
	conteudo, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "sql", arquivo))
	require.NoError(t, err)
	return string(conteudo)
}

// db09Migracao devolve o BEGIN NOT ATOMIC do sql/27, sem o DELIMITER do
// cliente: o servidor recebe o mesmo comando que o cliente mariadb envia.
func db09Migracao(t *testing.T) string {
	t.Helper()
	_, depois, achou := strings.Cut(db09LerSQL(t, db09ArquivoAlter), "DELIMITER $$\n")
	require.True(t, achou, "sql/27 sem DELIMITER $$")
	bloco, _, achou := strings.Cut(depois, "END$$")
	require.True(t, achou, "sql/27 sem END$$")
	return bloco + "END"
}

// db09Reversao devolve o ALTER do revert do sql/27.
func db09Reversao(t *testing.T) string {
	t.Helper()
	conteudo := db09LerSQL(t, db09ArquivoVolta)
	inicio := strings.LastIndex(conteudo, "ALTER TABLE")
	require.GreaterOrEqual(t, inicio, 0, "revert do sql/27 sem ALTER TABLE")
	return strings.TrimRight(strings.TrimSpace(conteudo[inicio:]), ";")
}

// db09ExigirLoopback pula o teste fora do loopback (ver cabeçalho).
func db09ExigirLoopback(t *testing.T, host string) {
	t.Helper()
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		t.Skip("DB-09: o teste troca a definição da coluna; só roda no banco descartável do loopback")
	}
}

type db09IT struct {
	t         *testing.T
	db        *sql.DB
	usuarioID int64
	nullID    int64
	cheiaID   int64
}

// novoDB09IT abre o banco, confere que não há linha NULL de outra origem e
// liga o Cleanup que apaga as linhas e deixa a coluna NOT NULL.
func novoDB09IT(t *testing.T) *db09IT {
	t.Helper()
	cfg := integracao.Config(t)
	db09ExigirLoopback(t, cfg.DBHost)
	db := integracao.Abrir(t)
	var alheias int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM senha_historico WHERE created_at IS NULL`).Scan(&alheias))
	require.Zero(t, alheias, "banco com linhas de created_at NULL de outra origem: use um descartável limpo")

	email := fmt.Sprintf("zz-test-db09-%d@teste.local", time.Now().UnixNano())
	res, err := db.Exec(`INSERT INTO usuarios (nome, email, password_hash, role, ativo) VALUES ('ZZ-DB09', ?, 'x', 'normal', 1)`, email)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	c := &db09IT{t: t, db: db, usuarioID: id}
	t.Cleanup(c.restaurar)
	return c
}

func (c *db09IT) restaurar() {
	// ON DELETE CASCADE leva o histórico junto, inclusive as linhas NULL.
	_, err := c.db.Exec(`DELETE FROM usuarios WHERE id = ?`, c.usuarioID)
	assert.NoError(c.t, err, "apagar o usuário temporário")
	_, err = c.db.Exec(db09Migracao(c.t))
	assert.NoError(c.t, err, "restaurar created_at NOT NULL")
	assert.False(c.t, db09ColunaAnulavel(c.t, c.db), "o Cleanup deve deixar created_at NOT NULL")
}

// semearColunaAnulavel volta a coluna a anulável e grava uma linha NULL e uma
// preenchida, o cenário de um banco criado antes do DB-09.
func (c *db09IT) semearColunaAnulavel() {
	c.t.Helper()
	_, err := c.db.Exec(db09Reversao(c.t))
	require.NoError(c.t, err)
	require.True(c.t, db09ColunaAnulavel(c.t, c.db), "o revert do sql/27 deve deixar created_at anulável")

	res, err := c.db.Exec(`INSERT INTO senha_historico (usuario_id, senha_hash_anterior, ip_origem, tipo_reset, created_at)
		VALUES (?, 'x', '10.0.0.9', 'usuario', NULL)`, c.usuarioID)
	require.NoError(c.t, err)
	c.nullID, err = res.LastInsertId()
	require.NoError(c.t, err)

	res, err = c.db.Exec(`INSERT INTO senha_historico (usuario_id, senha_hash_anterior, tipo_reset, created_at)
		VALUES (?, 'y', 'usuario', ?)`, c.usuarioID, db09Criado)
	require.NoError(c.t, err)
	c.cheiaID, err = res.LastInsertId()
	require.NoError(c.t, err)
}

// conferirIntacto: a coluna continua anulável, a linha NULL continua NULL e a
// preenchida não mudou.
func (c *db09IT) conferirIntacto() {
	c.t.Helper()
	assert.True(c.t, db09ColunaAnulavel(c.t, c.db), "o ALTER não pode rodar depois do SIGNAL")

	var criado sql.NullTime
	var ip sql.NullString
	require.NoError(c.t, c.db.QueryRow(`SELECT created_at, ip_origem FROM senha_historico WHERE id = ?`, c.nullID).Scan(&criado, &ip))
	assert.False(c.t, criado.Valid, "a linha NULL foi preenchida: %v", criado.Time)
	assert.Equal(c.t, "10.0.0.9", ip.String, "ip_origem da linha NULL")

	var cheia time.Time
	require.NoError(c.t, c.db.QueryRow(`SELECT created_at FROM senha_historico WHERE id = ?`, c.cheiaID).Scan(&cheia))
	assert.True(c.t, db09Criado.Equal(cheia), "a linha preenchida mudou: %v", cheia)
}

func TestIntegracaoDB09_Migracao27_Idempotente(t *testing.T) {
	c := novoDB09IT(t)
	migracao := db09Migracao(t)

	_, err := c.db.Exec(db09Reversao(t))
	require.NoError(t, err)
	require.True(t, db09ColunaAnulavel(t, c.db))

	for rodada := 1; rodada <= 2; rodada++ {
		_, err := c.db.Exec(migracao)
		require.NoError(t, err, "rodada %d", rodada)
		assert.False(t, db09ColunaAnulavel(t, c.db), "rodada %d", rodada)
	}
}

func TestIntegracaoDB09_Migracao27_AbortaComLinhaNull(t *testing.T) {
	c := novoDB09IT(t)
	c.semearColunaAnulavel()

	_, err := c.db.Exec(db09Migracao(t))

	var erroMySQL *mysql.MySQLError
	require.ErrorAs(t, err, &erroMySQL, "a migração deve abortar com SIGNAL")
	assert.Equal(t, db09ErroSignal, erroMySQL.Number)
	assert.Contains(t, erroMySQL.Message, db09Abortou)
	c.conferirIntacto()
}

// Regressão do --force: com o ALTER fora do bloco, o cliente com --force
// seguiria depois do SIGNAL e o MODIFY preencheria o NULL com a hora da migração.
func TestIntegracaoDB09_Migracao27_ForceNaoPreencheNull(t *testing.T) {
	c := novoDB09IT(t)
	if _, err := exec.LookPath(db09ClienteBin); err != nil {
		t.Skipf("cliente %s fora do PATH: regressão do --force não exercitada", db09ClienteBin)
	}
	c.semearColunaAnulavel()

	saida := db09RodarClienteForce(t, integracao.Config(t))

	// O código de saída com --force não é o critério: a prova é o SIGNAL ter
	// chegado (a conexão funcionou) e os dados continuarem iguais.
	assert.Contains(t, saida, db09Abortou, "o cliente deve mostrar o SIGNAL da migração")
	c.conferirIntacto()
}

// db09RodarClienteForce roda o sql/27 inteiro no cliente com --force e devolve
// a saída. O ambiente do filho é só a senha (TST-19: sem os.Environ; o caminho
// do executável já foi resolvido no processo do teste e --no-defaults dispensa
// o HOME).
func db09RodarClienteForce(t *testing.T, cfg *config.Config) string {
	t.Helper()
	args := []string{
		"--no-defaults", "--local-infile=0", "--force",
		"-u", cfg.DBUsuario, "-h", cfg.DBHost, "-P", cfg.DBPort,
		"--default-character-set=utf8mb4",
	}
	if cfg.DBTLS.Ativo() {
		args = append(args, "--ssl-ca="+cfg.DBTLS.CAFile, "--ssl-verify-server-cert")
	}
	ctx, cancelar := context.WithTimeout(context.Background(), db09TempoCliente)
	defer cancelar()
	cmd := exec.CommandContext(ctx, db09ClienteBin, append(args, cfg.DBName)...)
	cmd.Env = []string{"MYSQL_PWD=" + cfg.DBSenha}
	cmd.Stdin = strings.NewReader(db09LerSQL(t, db09ArquivoAlter))
	var saida bytes.Buffer
	cmd.Stdout = &saida
	cmd.Stderr = &saida
	if err := cmd.Run(); err != nil {
		t.Logf("cliente %s com --force terminou com: %s", db09ClienteBin, err.Error())
	}
	return saida.String()
}

// db09Definicao devolve o COLUMN_DEFAULT e o EXTRA de senha_historico.created_at.
func db09Definicao(t *testing.T, db *sql.DB) (padrao, extra string) {
	t.Helper()
	var p sql.NullString
	require.NoError(t, db.QueryRow(`SELECT COLUMN_DEFAULT, EXTRA FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'senha_historico' AND COLUMN_NAME = 'created_at'`).Scan(&p, &extra))
	return strings.ToLower(p.String), strings.ToLower(extra)
}

// O remédio do cabeçalho do sql/27 (UPDATE com a sentinela, na mesma sessão com
// time_zone -03:00) precisa destravar a migração, e a sentinela precisa ficar.
func TestIntegracaoDB09_Migracao27_RemedioDoCabecalho(t *testing.T) {
	c := novoDB09IT(t)
	c.semearColunaAnulavel()

	cabecalho := db09LerSQL(t, db09ArquivoAlter)
	remedio := `UPDATE senha_historico SET created_at = '1970-01-01 00:00:01' WHERE created_at IS NULL;`
	require.Contains(t, cabecalho, "--   SET time_zone = '-03:00';\n--   "+remedio, "o remédio documentado mudou: atualize o teste")

	ctx := context.Background()
	conn, err := c.db.Conn(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `SET time_zone = '-03:00'`)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, strings.TrimSuffix(remedio, ";"))
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	_, err = c.db.Exec(db09Migracao(t))
	require.NoError(t, err)
	assert.False(t, db09ColunaAnulavel(t, c.db))

	// A sessão do DSN está em -03:00 (config.DSN), a mesma do UPDATE do remédio.
	var sentinela string
	require.NoError(t, c.db.QueryRow(`SELECT CAST(created_at AS CHAR) FROM senha_historico WHERE id = ?`, c.nullID).Scan(&sentinela))
	assert.Equal(t, "1970-01-01 00:00:01", sentinela, "a sentinela precisa ficar")
	var cheia time.Time
	require.NoError(t, c.db.QueryRow(`SELECT created_at FROM senha_historico WHERE id = ?`, c.cheiaID).Scan(&cheia))
	assert.True(t, db09Criado.Equal(cheia), "a linha preenchida não pode mudar: %v", cheia)
}

// A migração não pode ganhar ON UPDATE: created_at é a trilha da troca de senha
// e um UPDATE posterior na linha reescreveria a data.
func TestIntegracaoDB09_Migracao27_SemOnUpdate(t *testing.T) {
	c := novoDB09IT(t)
	_, err := c.db.Exec(db09Reversao(t))
	require.NoError(t, err)
	_, err = c.db.Exec(db09Migracao(t))
	require.NoError(t, err)

	padrao, extra := db09Definicao(t, c.db)
	assert.Contains(t, padrao, "current_"+"timestamp", "DEFAULT da coluna")
	assert.NotContains(t, extra, "on update")

	res, err := c.db.Exec(`INSERT INTO senha_historico (usuario_id, senha_hash_anterior, tipo_reset, created_at)
		VALUES (?, 'y', 'usuario', ?)`, c.usuarioID, db09Criado)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	_, err = c.db.Exec(`UPDATE senha_historico SET ip_origem = '10.0.0.8' WHERE id = ?`, id)
	require.NoError(t, err)
	var criado time.Time
	require.NoError(t, c.db.QueryRow(`SELECT created_at FROM senha_historico WHERE id = ?`, id).Scan(&criado))
	assert.True(t, db09Criado.Equal(criado), "o UPDATE reescreveu created_at: %v", criado)
}

// O revert roda duas vezes sem erro e não preenche nem apaga nada.
func TestIntegracaoDB09_Revert_IdempotenteSemTocarDados(t *testing.T) {
	c := novoDB09IT(t)
	c.semearColunaAnulavel()
	_, err := c.db.Exec(db09Reversao(t))
	require.NoError(t, err)
	c.conferirIntacto()
}

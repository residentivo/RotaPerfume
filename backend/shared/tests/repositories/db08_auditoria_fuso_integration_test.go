package repositories_test

// DB-08 (Lote 38) — integração (INTEGRATION=1 + DB_*; senão t.Skip) do fuso
// da SESSÃO fixado em '-03:00' pelo DSN (config.DSN, tz.OffsetSQL).
//
// Aceite: no banco descartável com servidor em UTC ou em -03, os TIMESTAMP
// gravados pelo DEFAULT/ON UPDATE CURRENT_TIMESTAMP e os gravados pelo Go
// (loc=Local, -03) voltam como o mesmo instante, a menos de 2 min do relógio
// do processo. Sem o time_zone da sessão, num servidor UTC, o DEFAULT voltaria
// deslocado em 3 h.
//
// Dados: um usuário temporário (e-mail zz-test-db08-<n>@teste.local), o
// histórico de senha e o refresh token dele, apagados no t.Cleanup. Nenhum
// SQL deste arquivo usa o relógio do servidor (TST-23).

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/testkit/integracao"
	"github.com/rotaperfumes/shared/tz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// db08Janela é a tolerância entre o instante lido do banco e o relógio do
// processo (bem abaixo das 3 h do erro que o DB-08 corrige).
const db08Janela = 2 * time.Minute

type db08IT struct {
	t      *testing.T
	db     *sql.DB
	ctx    context.Context
	uid    int64
	prefix string
}

func novoDB08IT(t *testing.T) *db08IT {
	t.Helper()
	conn := integracao.Abrir(t)
	n := time.Now().UnixNano()
	c := &db08IT{t: t, db: conn, ctx: context.Background(), prefix: fmt.Sprintf("zz-test-db08-%d", n)}
	u := &identidade.Usuario{
		Nome:  "ZZ-TEST-DB08",
		Email: c.prefix + "@teste.local",
		Role:  kernel.RoleNormal,
		Hash:  identidade.NovoHashSenha("x"),
		Ativo: true,
	}
	require.NoError(t, identidademysql.NovoUsuarios(conn).Inserir(c.ctx, u))
	c.uid = u.ID
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM senha_historico WHERE usuario_id = ?`,
			`DELETE FROM refresh_tokens WHERE usuario_id = ?`,
			`DELETE FROM usuarios WHERE id = ?`,
		} {
			_, err := conn.Exec(q, c.uid)
			assert.NoError(t, err, q)
		}
	})
	return c
}

// db08NaJanela confere que o instante lido está a menos de db08Janela do
// relógio do processo.
func db08NaJanela(t *testing.T, rotulo string, lido time.Time) {
	t.Helper()
	diff := time.Since(lido)
	if diff < 0 {
		diff = -diff
	}
	assert.Less(t, diff, db08Janela, "%s: lido %s, processo %s (diferença %s)", rotulo, lido, time.Now(), diff)
}

func TestIntegracaoDB08_SessaoEmMenos3(t *testing.T) {
	c := novoDB08IT(t)
	var sessao string
	require.NoError(t, c.db.QueryRow(`SELECT @@session.time_zone`).Scan(&sessao))
	assert.Equal(t, tz.OffsetSQL, sessao, "o DSN fixa o time_zone da sessão (DB-08)")
}

func TestIntegracaoDB08_DefaultCurrentTimestamp(t *testing.T) {
	c := novoDB08IT(t)

	t.Run("usuarios.created_at e updated_at pelo DEFAULT", func(t *testing.T) {
		u, err := identidademysql.NovoUsuarios(c.db).PorID(c.ctx, c.uid)
		require.NoError(t, err)
		db08NaJanela(t, "usuarios.created_at", u.CriadoEm)
		db08NaJanela(t, "usuarios.updated_at", u.AtualizadoEm)
	})

	t.Run("senha_historico.created_at pelo DEFAULT", func(t *testing.T) {
		repo := identidademysql.NovoHistoricoSenhas(c.db)
		h := &identidade.RegistroSenha{UsuarioID: c.uid, HashAnterior: identidade.NovoHashSenha("y"), IP: "127.0.0.1", UserAgent: "go-test", Tipo: identidade.TipoResetUsuario}
		require.NoError(t, repo.Registrar(c.ctx, h))
		res, err := repo.DoUsuario(c.ctx, c.uid, identidade.OrdenacaoHistoricoPadrao(), kernel.NovaPagina(1, 10))
		require.NoError(t, err)
		require.Len(t, res.Itens, 1)
		db08NaJanela(t, "senha_historico.created_at", res.Itens[0].CriadoEm)
	})

	t.Run("refresh_tokens.created_at pelo DEFAULT", func(t *testing.T) {
		rt := &identidade.RefreshToken{UsuarioID: c.uid, HashToken: c.prefix + "-rt", ExpiraEm: time.Now().Add(time.Hour).Truncate(time.Second), IP: "127.0.0.1", UserAgent: "go-test"}
		require.NoError(t, identidademysql.NovoRefreshTokens(c.db).Inserir(c.ctx, rt))
		var criado time.Time
		require.NoError(t, c.db.QueryRow(`SELECT created_at FROM refresh_tokens WHERE id = ?`, rt.ID).Scan(&criado))
		db08NaJanela(t, "refresh_tokens.created_at", criado)
	})
}

func TestIntegracaoDB08_OnUpdateCurrentTimestamp(t *testing.T) {
	c := novoDB08IT(t)
	// Recua updated_at para um instante conhecido (parâmetro do Go), para o
	// ON UPDATE ter de gravar um valor novo.
	antigo := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	_, err := c.db.Exec(`UPDATE usuarios SET updated_at = ? WHERE id = ?`, antigo, c.uid)
	require.NoError(t, err)

	repo := identidademysql.NovoUsuarios(c.db)
	require.NoError(t, repo.Atualizar(c.ctx, c.uid, identidade.UsuarioValidado{Nome: "ZZ-TEST-DB08-EDITADO", Role: kernel.RoleNormal}))
	u, err := repo.PorID(c.ctx, c.uid)
	require.NoError(t, err)
	assert.Equal(t, "ZZ-TEST-DB08-EDITADO", u.Nome)
	db08NaJanela(t, "usuarios.updated_at pelo ON UPDATE", u.AtualizadoEm)
}

func TestIntegracaoDB08_UltimoLoginIdaEVolta(t *testing.T) {
	c := novoDB08IT(t)
	repo := identidademysql.NovoUsuarios(c.db)
	// Instante de madrugada UTC (dia anterior em -03), para pegar desvio de fuso.
	quando := time.Date(2026, 11, 6, 1, 30, 15, 0, time.UTC)
	require.NoError(t, repo.RegistrarLogin(c.ctx, c.uid, quando))
	u, err := repo.PorID(c.ctx, c.uid)
	require.NoError(t, err)
	require.NotNil(t, u.UltimoLoginEm)
	assert.Equal(t, quando.Unix(), u.UltimoLoginEm.Unix(), "ultimo_login_at volta como o mesmo instante")

	// Lido na sessão -03, o TIMESTAMP aparece na parede -03.
	var parede string
	require.NoError(t, c.db.QueryRow(`SELECT DATE_FORMAT(ultimo_login_at, '%Y-%m-%d %H:%i:%s') FROM usuarios WHERE id = ?`, c.uid).Scan(&parede))
	assert.Equal(t, "2026-11-05 22:30:15", parede)
}

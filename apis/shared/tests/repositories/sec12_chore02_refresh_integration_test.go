package repositories_test

// Integração (INTEGRATION=1 + DB_USUARIO/DB_SENHA; senão t.Skip) do
// SEC-12/CHORE-02 no RefreshTokenRepository contra o MySQL local.
//
// Dados: um usuário temporário (e-mail zz-test-sec12-<n>@teste.local) e os
// refresh tokens dele (token_hash "zz-test-sec12-<n>-..."), apagados no
// t.Cleanup. O DeleteExpired usa cortes no ano 2001, então só alcança as
// linhas deste teste (há uma guarda que pula o teste se houver outras).

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

type sec12IT struct {
	t      *testing.T
	db     *sql.DB
	uid    int64
	prefix string
	seq    int
}

func novoSEC12IT(t *testing.T) *sec12IT {
	t.Helper()
	db := abrirDBSEC08(t)
	n := time.Now().UnixNano()
	res, err := db.Exec(`INSERT INTO usuarios (nome, email, password_hash, role, ativo) VALUES ('ZZ-TEST-SEC12', ?, 'x', 'normal', 1)`,
		fmt.Sprintf("zz-test-sec12-%d@teste.local", n))
	require.NoError(t, err)
	uid, err := res.LastInsertId()
	require.NoError(t, err)
	c := &sec12IT{t: t, db: db, uid: uid, prefix: fmt.Sprintf("zz-test-sec12-%d-", n)}
	t.Cleanup(func() {
		_, err := db.Exec(`DELETE FROM refresh_tokens WHERE usuario_id = ?`, uid)
		assert.NoError(t, err)
		_, err = db.Exec(`DELETE FROM usuarios WHERE id = ?`, uid)
		assert.NoError(t, err)
		var resto int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE token_hash LIKE ?`, c.prefix+"%").Scan(&resto))
		assert.Zero(t, resto, "tokens de teste remanescentes")
	})
	return c
}

// token insere um refresh token do usuário de teste. motivo "" = ativo.
func (c *sec12IT) token(expira time.Time, motivo string) int64 {
	c.t.Helper()
	c.seq++
	rt := &repositories.RefreshToken{UsuarioID: c.uid, TokenHash: fmt.Sprintf("%s%d", c.prefix, c.seq), ExpiresAt: expira, IPOrigem: "127.0.0.1", UserAgent: "go-test"}
	require.NoError(c.t, repositories.NewRefreshTokenRepository().Create(context.Background(), c.db, rt))
	if motivo != "" {
		_, err := c.db.Exec(`UPDATE refresh_tokens SET revoked_at = NOW() - INTERVAL 5 MINUTE, revoked_reason = ? WHERE id = ?`, motivo, rt.ID)
		require.NoError(c.t, err)
	}
	return rt.ID
}

func (c *sec12IT) marca(id int64) *time.Time {
	c.t.Helper()
	var v *time.Time
	require.NoError(c.t, c.db.QueryRow(`SELECT reuso_detectado_em FROM refresh_tokens WHERE id = ?`, id).Scan(&v))
	return v
}

func TestIntegracaoSEC12_MarcarReusoDetectado(t *testing.T) {
	c := novoSEC12IT(t)
	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	janela := 30 * time.Minute
	agora := time.Now().Truncate(time.Second)
	id := c.token(time.Now().Add(7*24*time.Hour), "rotacao")

	t.Run("concorrência: 20 marcações simultâneas, só uma corta", func(t *testing.T) {
		const n = 20
		var wg sync.WaitGroup
		var mu sync.Mutex
		marcou, erros := 0, 0
		inicio := make(chan struct{})
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-inicio
				ok, err := repo.MarcarReusoDetectado(ctx, c.db, id, agora, agora.Add(-janela))
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					erros++
				}
				if ok {
					marcou++
				}
			}()
		}
		close(inicio)
		wg.Wait()
		assert.Zero(t, erros)
		assert.Equal(t, 1, marcou, "exatamente uma requisição deve cortar")
		m := c.marca(id)
		require.NotNil(t, m)
		assert.True(t, m.Equal(agora), "gravado=%s agora=%s (sem deslocamento de fuso)", m, agora)
	})

	casos := []struct {
		nome   string
		agora  time.Time
		want   bool
		marcaF time.Time // valor esperado na coluna depois
	}{
		{"dentro da janela (10 min depois) → suprimido", agora.Add(10 * time.Minute), false, agora},
		{"no limite exato (30 min depois) → corta de novo", agora.Add(janela), true, agora.Add(janela)},
		{"logo depois (1 min) do novo corte → suprimido", agora.Add(janela + time.Minute), false, agora.Add(janela)},
		{"31 min depois do último corte → corta", agora.Add(2*janela + time.Minute), true, agora.Add(2*janela + time.Minute)},
	}
	for _, cs := range casos {
		t.Run(cs.nome, func(t *testing.T) {
			ok, err := repo.MarcarReusoDetectado(ctx, c.db, id, cs.agora, cs.agora.Add(-janela))
			require.NoError(t, err)
			assert.Equal(t, cs.want, ok)
			m := c.marca(id)
			require.NotNil(t, m)
			assert.True(t, m.Equal(cs.marcaF), "coluna=%s esperado=%s", m, cs.marcaF)
		})
	}

	t.Run("token não rotacionado nunca é marcado", func(t *testing.T) {
		for _, motivo := range []string{"", "logout", "senha", "inativacao", "revogacao_massa"} {
			outro := c.token(time.Now().Add(time.Hour), motivo)
			ok, err := repo.MarcarReusoDetectado(ctx, c.db, outro, agora, agora.Add(-janela))
			require.NoError(t, err)
			assert.False(t, ok, "motivo=%q", motivo)
			assert.Nil(t, c.marca(outro), "motivo=%q", motivo)
		}
	})

	t.Run("token inexistente → false sem erro", func(t *testing.T) {
		ok, err := repo.MarcarReusoDetectado(ctx, c.db, -1, agora, agora.Add(-janela))
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestIntegracaoSEC12_DesfazerReusoDetectado(t *testing.T) {
	c := novoSEC12IT(t)
	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	agora := time.Now().Truncate(time.Second)
	id := c.token(time.Now().Add(time.Hour), "rotacao")

	ok, err := repo.MarcarReusoDetectado(ctx, c.db, id, agora, agora.Add(-30*time.Minute))
	require.NoError(t, err)
	require.True(t, ok)

	// Marca de outra chamada: não mexe.
	require.NoError(t, repo.DesfazerReusoDetectado(ctx, c.db, id, agora.Add(-time.Second)))
	require.NotNil(t, c.marca(id), "marca de outra chamada é preservada")

	require.NoError(t, repo.DesfazerReusoDetectado(ctx, c.db, id, agora))
	assert.Nil(t, c.marca(id), "a marca desta chamada é limpa")

	// Depois de desfeita, o próximo reuso volta a cortar mesmo dentro da janela.
	ok, err = repo.MarcarReusoDetectado(ctx, c.db, id, agora.Add(time.Minute), agora.Add(time.Minute-30*time.Minute))
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestIntegracaoCHORE02_DeleteExpired(t *testing.T) {
	c := novoSEC12IT(t)
	repo := repositories.NewRefreshTokenRepository()
	ctx := context.Background()
	corte := time.Date(2001, 1, 10, 12, 0, 0, 0, time.Local)

	var outros int
	require.NoError(t, c.db.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE expires_at < ? AND usuario_id <> ?`, corte.Add(24*time.Hour), c.uid).Scan(&outros))
	if outros > 0 {
		t.Skipf("banco de dev tem %d tokens de outros usuários com expires_at < %s; o teste não os apaga", outros, corte)
	}

	velhos := []int64{
		c.token(corte.Add(-72*time.Hour), ""),
		c.token(corte.Add(-48*time.Hour), "rotacao"),
		c.token(corte.Add(-time.Second), "logout"),
	}
	noCorte := c.token(corte, "")                          // expires_at == corte: fica (estrito <)
	depois := c.token(corte.Add(time.Hour), "revogacao_massa") // depois do corte: fica
	valido := c.token(time.Now().Add(time.Hour), "")       // ainda válido: fica

	n, err := repo.DeleteExpired(ctx, c.db, corte, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "respeita o LIMIT do lote")
	n, err = repo.DeleteExpired(ctx, c.db, corte, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "lote incompleto: fim")
	n, err = repo.DeleteExpired(ctx, c.db, corte, 2)
	require.NoError(t, err)
	assert.Zero(t, n)

	existe := func(id int64) bool {
		var k int
		require.NoError(t, c.db.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE id = ?`, id).Scan(&k))
		return k == 1
	}
	for _, id := range velhos {
		assert.False(t, existe(id), "id=%d expirado antes do corte deveria ter sido apagado", id)
	}
	for _, id := range []int64{noCorte, depois, valido} {
		assert.True(t, existe(id), "id=%d não pode ser apagado", id)
	}
}

package services_test

// Integração (INTEGRATION=1 + DB_USUARIO/DB_SENHA; senão t.Skip) do SEC-12 e
// do CHORE-02 no RefreshTokenService contra o MySQL local.
//
// Dados: um usuário temporário (e-mail zz-test-l12-<n>@teste.local) e os
// refresh tokens dele, apagados no t.Cleanup.
//
// CHORE-02 usa o relógio real (corte = agora - 720h) e apaga QUALQUER token
// do banco expirado há mais de 30 dias — o mesmo que a API faz ao subir.
// Para não tocar em dados alheios, o teste é pulado se houver tokens de
// outros usuários nessa situação.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

type l12IT struct {
	t   *testing.T
	db  *sql.DB
	uid int64
}

func novoL12IT(t *testing.T) *l12IT {
	t.Helper()
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("teste de integração: defina INTEGRATION=1 (requer MySQL local)")
	}
	db := abrirDBIntegracao(t)
	res, err := db.Exec(`INSERT INTO usuarios (nome, email, password_hash, role, ativo) VALUES ('ZZ-TEST-L12', ?, 'x', 'normal', 1)`,
		fmt.Sprintf("zz-test-l12-%d@teste.local", time.Now().UnixNano()))
	require.NoError(t, err)
	uid, err := res.LastInsertId()
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.Exec(`DELETE FROM refresh_tokens WHERE usuario_id = ?`, uid)
		assert.NoError(t, err)
		_, err = db.Exec(`DELETE FROM usuarios WHERE id = ?`, uid)
		assert.NoError(t, err)
	})
	return &l12IT{t: t, db: db, uid: uid}
}

// token cria um refresh token (texto puro devolvido) e ajusta expires_at e a
// revogação direto no banco. motivo "" = ativo.
func (c *l12IT) token(expira time.Time, motivo string, revogadoHa time.Duration) (string, int64) {
	c.t.Helper()
	svc := services.NewRefreshTokenService()
	tok, err := svc.GenerateRefreshToken(context.Background(), c.db, c.uid, "127.0.0.1", "go-test")
	require.NoError(c.t, err)
	var id int64
	require.NoError(c.t, c.db.QueryRow(`SELECT id FROM refresh_tokens WHERE token_hash = ?`, hashOf(tok)).Scan(&id))
	_, err = c.db.Exec(`UPDATE refresh_tokens SET expires_at = ? WHERE id = ?`, expira, id)
	require.NoError(c.t, err)
	if motivo != "" {
		_, err = c.db.Exec(`UPDATE refresh_tokens SET revoked_at = ?, revoked_reason = ? WHERE id = ?`, time.Now().Add(-revogadoHa), motivo, id)
		require.NoError(c.t, err)
	}
	return tok, id
}

func (c *l12IT) existe(id int64) bool {
	c.t.Helper()
	var n int
	require.NoError(c.t, c.db.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE id = ?`, id).Scan(&n))
	return n == 1
}

// SEC-12: 20 RegistrarReuso concorrentes no mesmo token → só um corta; o
// relógio avançado além da janela volta a cortar; DesfazerReuso limpa a marca.
func TestIntegracaoSEC12_RegistrarReuso_Concorrencia(t *testing.T) {
	c := novoL12IT(t)
	ctx := context.Background()
	_, id := c.token(time.Now().Add(24*time.Hour), "rotacao", 5*time.Minute)

	agora := time.Now()
	svc := services.NewRefreshTokenServiceWithClock(func() time.Time { return agora })

	const n = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	cortes, erros := 0, 0
	var marca time.Time
	inicio := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-inicio
			cortar, m, err := svc.RegistrarReuso(ctx, c.db, id)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				erros++
			}
			if cortar {
				cortes++
				marca = m
			}
		}()
	}
	close(inicio)
	wg.Wait()
	require.Zero(t, erros)
	require.Equal(t, 1, cortes, "exatamente um corte")
	assert.True(t, marca.Equal(agora.Truncate(time.Second)))

	casos := []struct {
		nome   string
		depois time.Duration
		want   bool
	}{
		{"29 min depois → suprimido", 29 * time.Minute, false},
		{"31 min depois → corta de novo", 31 * time.Minute, true},
		{"32 min depois (1 min após o novo corte) → suprimido", 32 * time.Minute, false},
	}
	for _, cs := range casos {
		t.Run(cs.nome, func(t *testing.T) {
			s := services.NewRefreshTokenServiceWithClock(func() time.Time { return agora.Add(cs.depois) })
			cortar, _, err := s.RegistrarReuso(ctx, c.db, id)
			require.NoError(t, err)
			assert.Equal(t, cs.want, cortar)
		})
	}

	t.Run("DesfazerReuso reabre o corte dentro da janela", func(t *testing.T) {
		s := services.NewRefreshTokenServiceWithClock(func() time.Time { return agora.Add(31 * time.Minute) })
		require.NoError(t, s.DesfazerReuso(ctx, c.db, id, agora.Add(31*time.Minute).Truncate(time.Second)))
		var v *time.Time
		require.NoError(t, c.db.QueryRow(`SELECT reuso_detectado_em FROM refresh_tokens WHERE id = ?`, id).Scan(&v))
		assert.Nil(t, v)
		cortar, _, err := s.RegistrarReuso(ctx, c.db, id)
		require.NoError(t, err)
		assert.True(t, cortar)
	})
}

// CHORE-02: now-1h e now-29d sobrevivem; now-31d é apagado; token
// rotacionado não expirado continua gerando detecção de reuso depois.
func TestIntegracaoCHORE02_CleanupExpired(t *testing.T) {
	c := novoL12IT(t)
	ctx := context.Background()
	agora := time.Now()
	corte := agora.Add(-services.RefreshTokenRetencaoPadrao)

	var outros int
	require.NoError(t, c.db.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE expires_at < ? AND usuario_id <> ?`,
		corte.Add(time.Hour), c.uid).Scan(&outros))
	if outros > 0 {
		t.Skipf("banco de dev tem %d tokens de outros usuários expirados há mais de 30 dias; o cleanup os apagaria", outros)
	}

	_, expirou1h := c.token(agora.Add(-time.Hour), "", 0)
	_, expirou29d := c.token(agora.Add(-29*24*time.Hour), "logout", 30*24*time.Hour)
	_, expirou31d := c.token(agora.Add(-31*24*time.Hour), "rotacao", 32*24*time.Hour)
	_, expirou40d := c.token(agora.Add(-40*24*time.Hour), "", 0)
	rotacionado, rotacionadoID := c.token(agora.Add(6*24*time.Hour), "rotacao", 10*time.Minute)
	_, ativo := c.token(agora.Add(7*24*time.Hour), "", 0)

	svc := services.NewRefreshTokenService()
	n, err := svc.CleanupExpired(ctx, c.db)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "só os expirados há mais de 30 dias")

	casos := []struct {
		nome string
		id   int64
		fica bool
	}{
		{"expires_at = now-1h sobrevive", expirou1h, true},
		{"expires_at = now-29d sobrevive", expirou29d, true},
		{"expires_at = now-31d é apagado", expirou31d, false},
		{"expires_at = now-40d é apagado", expirou40d, false},
		{"rotacionado e não expirado sobrevive", rotacionadoID, true},
		{"ativo sobrevive", ativo, true},
	}
	for _, cs := range casos {
		t.Run(cs.nome, func(t *testing.T) {
			assert.Equal(t, cs.fica, c.existe(cs.id))
		})
	}

	t.Run("token rotacionado continua gerando detecção de reuso", func(t *testing.T) {
		_, err := svc.ValidateRefreshToken(ctx, c.db, rotacionado)
		var rev *services.RevokedTokenError
		require.True(t, errors.As(err, &rev), "err=%v", err)
		assert.True(t, rev.IsRotationReuse())
		assert.Equal(t, rotacionadoID, rev.TokenID)
		cortar, _, err := svc.RegistrarReuso(ctx, c.db, rev.TokenID)
		require.NoError(t, err)
		assert.True(t, cortar)
	})

	t.Run("segunda execução não apaga mais nada", func(t *testing.T) {
		n, err := svc.CleanupExpired(ctx, c.db)
		require.NoError(t, err)
		assert.Zero(t, n)
	})
}

// CHORE-02 pelo agendador: a execução imediata apaga o token velho.
func TestIntegracaoCHORE02_Agendador(t *testing.T) {
	c := novoL12IT(t)
	agora := time.Now()
	var outros int
	require.NoError(t, c.db.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE expires_at < ? AND usuario_id <> ?`,
		agora.Add(-services.RefreshTokenRetencaoPadrao+time.Hour), c.uid).Scan(&outros))
	if outros > 0 {
		t.Skipf("banco de dev tem %d tokens de outros usuários expirados há mais de 30 dias", outros)
	}
	_, velho := c.token(agora.Add(-60*24*time.Hour), "", 0)
	_, novo := c.token(agora.Add(time.Hour), "", 0)

	ctx, cancel := context.WithCancel(context.Background())
	done := services.IniciarLimpezaRefreshTokens(ctx, c.db, services.NewRefreshTokenService(), time.Hour)
	esperarAte(t, 5*time.Second, func() bool { return !c.existe(velho) }, "execução imediata do agendador")
	cancel()
	require.True(t, fechaEm(done, time.Second))
	assert.True(t, c.existe(novo))
}


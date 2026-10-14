// Package middleware_test contém testes unitários para o LoginRateLimiter.
package middleware_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/rotaperfumes/shared/adapters/ratelimit"
	"github.com/stretchr/testify/assert"
)

// ------------------------------------------------------------------
// LoginRateLimiter — chave nunca registrada não está bloqueada
// ------------------------------------------------------------------

func TestLoginRateLimiter_ChaveNaoRegistrada(t *testing.T) {
	l := ratelimit.Nova(3, time.Minute, time.Minute)

	blocked, wait := l.Bloqueado("1.2.3.4")
	assert.False(t, blocked)
	assert.Zero(t, wait)
}

// ------------------------------------------------------------------
// LoginRateLimiter — bloqueia após N falhas consecutivas
// ------------------------------------------------------------------

func TestLoginRateLimiter_BloqueiaAposNFalhas(t *testing.T) {
	maxFailures := 3
	l := ratelimit.Nova(maxFailures, time.Minute, 5*time.Minute)
	key := "1.2.3.4"

	for i := 0; i < maxFailures-1; i++ {
		l.RegistrarFalha(key)
		blocked, _ := l.Bloqueado(key)
		assert.False(t, blocked, "não deveria bloquear antes de atingir maxFailures (tentativa %d)", i+1)
	}

	l.RegistrarFalha(key)
	blocked, wait := l.Bloqueado(key)
	assert.True(t, blocked, "deveria bloquear após atingir maxFailures")
	assert.Greater(t, wait, time.Duration(0))
}

// ------------------------------------------------------------------
// LoginRateLimiter — parametrizado: limites diferentes
// ------------------------------------------------------------------

func TestLoginRateLimiter_BloqueiaAposNFalhas_Parametrizado(t *testing.T) {
	cases := []int{1, 2, 5, 10}

	for _, maxFailures := range cases {
		t.Run(
			"maxFailures="+strconv.Itoa(maxFailures),
			func(t *testing.T) {
				l := ratelimit.Nova(maxFailures, time.Minute, time.Minute)
				key := "chave-teste"

				for i := 0; i < maxFailures; i++ {
					blocked, _ := l.Bloqueado(key)
					assert.False(t, blocked, "não deveria estar bloqueado antes da falha %d/%d", i+1, maxFailures)
					l.RegistrarFalha(key)
				}

				blocked, _ := l.Bloqueado(key)
				assert.True(t, blocked, "deveria bloquear exatamente após %d falhas", maxFailures)
			},
		)
	}
}

// ------------------------------------------------------------------
// LoginRateLimiter — RegisterSuccess reseta o contador de falhas
// ------------------------------------------------------------------

func TestLoginRateLimiter_SucessoResetaFalhas(t *testing.T) {
	maxFailures := 3
	l := ratelimit.Nova(maxFailures, time.Minute, time.Minute)
	key := "1.2.3.4"

	l.RegistrarFalha(key)
	l.RegistrarFalha(key)
	// Ainda não atingiu o limite.
	blocked, _ := l.Bloqueado(key)
	assert.False(t, blocked)

	l.RegistrarSucesso(key)

	// Depois do sucesso, um novo ciclo de falhas deve precisar de maxFailures
	// falhas novamente (não deve "herdar" as 2 falhas anteriores).
	for i := 0; i < maxFailures-1; i++ {
		l.RegistrarFalha(key)
		blocked, _ = l.Bloqueado(key)
		assert.False(t, blocked, "contador deveria ter sido resetado pelo RegisterSuccess")
	}
	l.RegistrarFalha(key)
	blocked, _ = l.Bloqueado(key)
	assert.True(t, blocked)
}

// ------------------------------------------------------------------
// LoginRateLimiter — bloqueio expira após blockFor
// ------------------------------------------------------------------

func TestLoginRateLimiter_BloqueioExpiraAposBlockFor(t *testing.T) {
	l := ratelimit.Nova(1, time.Minute, 50*time.Millisecond)
	key := "1.2.3.4"

	l.RegistrarFalha(key)
	blocked, _ := l.Bloqueado(key)
	assert.True(t, blocked, "deveria estar bloqueado imediatamente após atingir maxFailures")

	time.Sleep(100 * time.Millisecond)

	blocked, wait := l.Bloqueado(key)
	assert.False(t, blocked, "bloqueio deveria ter expirado após blockFor")
	assert.Zero(t, wait)
}

// ------------------------------------------------------------------
// LoginRateLimiter — janela de tempo expira e reseta contagem de falhas
// ------------------------------------------------------------------

func TestLoginRateLimiter_JanelaExpiraResetaFalhas(t *testing.T) {
	// Janela curta: falhas fora da janela não devem se acumular para o bloqueio.
	l := ratelimit.Nova(2, 30*time.Millisecond, time.Minute)
	key := "1.2.3.4"

	l.RegistrarFalha(key)
	blocked, _ := l.Bloqueado(key)
	assert.False(t, blocked)

	// Espera a janela expirar antes da segunda falha.
	time.Sleep(60 * time.Millisecond)

	l.RegistrarFalha(key)
	blocked, _ = l.Bloqueado(key)
	assert.False(t, blocked, "falha após expiração da janela deveria reiniciar a contagem, não bloquear")
}

// ------------------------------------------------------------------
// LoginRateLimiter — chave vazia é sempre ignorada (não bloqueia, não registra)
// ------------------------------------------------------------------

func TestLoginRateLimiter_ChaveVazia(t *testing.T) {
	l := ratelimit.Nova(1, time.Minute, time.Minute)

	l.RegistrarFalha("")
	blocked, wait := l.Bloqueado("")
	assert.False(t, blocked)
	assert.Zero(t, wait)

	// Não deve entrar em pânico nem afetar outras chaves.
	l.RegistrarSucesso("")
}

// ------------------------------------------------------------------
// LoginRateLimiter — chaves diferentes são independentes
// ------------------------------------------------------------------

func TestLoginRateLimiter_ChavesIndependentes(t *testing.T) {
	l := ratelimit.Nova(2, time.Minute, time.Minute)

	l.RegistrarFalha("ip-a")
	l.RegistrarFalha("ip-a")
	blockedA, _ := l.Bloqueado("ip-a")
	blockedB, _ := l.Bloqueado("ip-b")

	assert.True(t, blockedA, "ip-a deveria estar bloqueado")
	assert.False(t, blockedB, "ip-b não deveria ser afetado pelas falhas de ip-a")
}

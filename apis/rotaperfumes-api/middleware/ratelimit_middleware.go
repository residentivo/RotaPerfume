// Package middleware contém middlewares HTTP compartilhados (CORS, auth, log).
package middleware

import (
	"sync"
	"time"

	"github.com/rotaperfumes/shared/vlog"
)

// LoginRateLimiter é um limiter em memória, por chave arbitrária (ex.: IP,
// ou IP+email), que bloqueia temporariamente uma chave após um número de
// falhas consecutivas dentro de uma janela de tempo curta. Usado para mitigar
// brute force / credential stuffing em endpoints de autenticação.
//
// Implementação simples (mutex + mapa) — suficiente para uma única instância
// de API; não é compartilhado entre réplicas.
type LoginRateLimiter struct {
	mu          sync.Mutex
	entries     map[string]*rateLimitEntry
	maxFailures int
	window      time.Duration
	blockFor    time.Duration
}

type rateLimitEntry struct {
	failures     int
	windowStart  time.Time
	blockedUntil time.Time
}

// NewLoginRateLimiter cria um limiter que bloqueia uma chave por blockFor
// após maxFailures falhas registradas dentro de window.
func NewLoginRateLimiter(maxFailures int, window, blockFor time.Duration) *LoginRateLimiter {
	vlog.Printf("ratelimit_middleware.go", "NewLoginRateLimiter", "montando &LoginRateLimiter e declarando l")
	l := &LoginRateLimiter{
		entries:     make(map[string]*rateLimitEntry),
		maxFailures: maxFailures,
		window:      window,
		blockFor:    blockFor,
	}
	vlog.Printf("ratelimit_middleware.go", "NewLoginRateLimiter", "disparando goroutine: l.cleanupLoop")
	go l.cleanupLoop()
	return l
}

// Blocked retorna se a chave está bloqueada no momento e por quanto tempo
// ainda (0 se não estiver bloqueada).
func (l *LoginRateLimiter) Blocked(key string) (bool, time.Duration) {
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.Blocked", "verificando condição key == \"\"")
	if key == "" {
		return false, 0
	}
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.Blocked", "chamando l.mu.Lock")
	l.mu.Lock()
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.Blocked", "agendando defer: l.mu.Unlock")
	defer l.mu.Unlock()

	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.Blocked", "declarando entry, ok com l.entries[key]")
	entry, ok := l.entries[key]
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.Blocked", "verificando condição !ok")
	if !ok {
		return false, 0
	}
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.Blocked", "chamando time.Now e declarando now")
	now := time.Now()
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.Blocked", "verificando condição now.Before(entry.blockedUntil)")
	if now.Before(entry.blockedUntil) {
		return true, entry.blockedUntil.Sub(now)
	}
	return false, 0
}

// RegisterFailure registra uma tentativa falha para a chave. Se o número de
// falhas na janela atual atingir maxFailures, bloqueia a chave por blockFor.
func (l *LoginRateLimiter) RegisterFailure(key string) {
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterFailure", "verificando condição key == \"\"")
	if key == "" {
		return
	}
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterFailure", "chamando l.mu.Lock")
	l.mu.Lock()
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterFailure", "agendando defer: l.mu.Unlock")
	defer l.mu.Unlock()

	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterFailure", "chamando time.Now e declarando now")
	now := time.Now()
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterFailure", "declarando entry, ok com l.entries[key]")
	entry, ok := l.entries[key]
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterFailure", "verificando condição !ok || now.Sub(entry.windowStart) > l.window")
	if !ok || now.Sub(entry.windowStart) > l.window {
		vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterFailure", "montando &rateLimitEntry e atribuindo a entry")
		entry = &rateLimitEntry{windowStart: now}
		vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterFailure", "atribuindo entry a l.entries[key]")
		l.entries[key] = entry
	}
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterFailure", "incrementando entry.failures")
	entry.failures++
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterFailure", "verificando condição entry.failures >= l.maxFailures")
	if entry.failures >= l.maxFailures {
		vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterFailure", "chamando now.Add e atribuindo a entry.blockedUntil")
		entry.blockedUntil = now.Add(l.blockFor)
	}
}

// RegisterSuccess limpa o registro de falhas da chave (ex.: login bem-sucedido).
func (l *LoginRateLimiter) RegisterSuccess(key string) {
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterSuccess", "verificando condição key == \"\"")
	if key == "" {
		return
	}
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterSuccess", "chamando l.mu.Lock")
	l.mu.Lock()
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterSuccess", "agendando defer: l.mu.Unlock")
	defer l.mu.Unlock()
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.RegisterSuccess", "chamando delete")
	delete(l.entries, key)
}

// cleanupLoop remove periodicamente entradas expiradas para evitar
// crescimento ilimitado do mapa em memória em processos de longa duração.
func (l *LoginRateLimiter) cleanupLoop() {
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.cleanupLoop", "chamando time.NewTicker e declarando ticker")
	ticker := time.NewTicker(10 * time.Minute)
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.cleanupLoop", "agendando defer: ticker.Stop")
	defer ticker.Stop()
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.cleanupLoop", "iniciando loop range sobre ticker.C")
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for k, entry := range l.entries {
			expired := now.Sub(entry.windowStart) > l.window && now.After(entry.blockedUntil)
			if expired {
				delete(l.entries, k)
			}
		}
		l.mu.Unlock()
	}
	vlog.Printf("ratelimit_middleware.go", "LoginRateLimiter.cleanupLoop", "loop range concluído sobre ticker.C")
}

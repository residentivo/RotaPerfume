// Package middleware contém middlewares HTTP compartilhados (CORS, auth, log).
package middleware

import (
	"sync"
	"time"
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
	l := &LoginRateLimiter{
		entries:     make(map[string]*rateLimitEntry),
		maxFailures: maxFailures,
		window:      window,
		blockFor:    blockFor,
	}
	go l.cleanupLoop()
	return l
}

// Blocked retorna se a chave está bloqueada no momento e por quanto tempo
// ainda (0 se não estiver bloqueada).
func (l *LoginRateLimiter) Blocked(key string) (bool, time.Duration) {
	if key == "" {
		return false, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok {
		return false, 0
	}
	now := time.Now()
	if now.Before(entry.blockedUntil) {
		return true, entry.blockedUntil.Sub(now)
	}
	return false, 0
}

// RegisterFailure registra uma tentativa falha para a chave. Se o número de
// falhas na janela atual atingir maxFailures, bloqueia a chave por blockFor.
func (l *LoginRateLimiter) RegisterFailure(key string) {
	if key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, ok := l.entries[key]
	if !ok || now.Sub(entry.windowStart) > l.window {
		entry = &rateLimitEntry{windowStart: now}
		l.entries[key] = entry
	}
	entry.failures++
	if entry.failures >= l.maxFailures {
		entry.blockedUntil = now.Add(l.blockFor)
	}
}

// RegisterSuccess limpa o registro de falhas da chave (ex.: login bem-sucedido).
func (l *LoginRateLimiter) RegisterSuccess(key string) {
	if key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// cleanupLoop remove periodicamente entradas expiradas para evitar
// crescimento ilimitado do mapa em memória em processos de longa duração.
func (l *LoginRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
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
}

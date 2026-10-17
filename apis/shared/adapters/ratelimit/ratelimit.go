// Package ratelimit é o limitador em memória por chave (Adapter da porta
// identidadeapp.Limitador, ARQ-03; ex-middleware.LoginRateLimiter): após
// maxFalhas falhas dentro de janela, bloqueia a chave por bloqueio (I-14).
// Uma instância por processo, criada no bootstrap; não é compartilhado
// entre réplicas.
package ratelimit

import (
	"sync"
	"time"
)

// intervaloLimpeza: de quanto em quanto tempo entradas vencidas saem do mapa.
const intervaloLimpeza = 10 * time.Minute

// Memoria é o limitador (mutex + mapa).
type Memoria struct {
	mu        sync.Mutex
	entradas  map[string]*entrada
	maxFalhas int
	janela    time.Duration
	bloqueio  time.Duration
}

type entrada struct {
	falhas       int
	inicioJanela time.Time
	bloqueadaAte time.Time
}

// Nova cria o limitador e sobe a limpeza periódica do mapa.
func Nova(maxFalhas int, janela, bloqueio time.Duration) *Memoria {
	m := &Memoria{entradas: make(map[string]*entrada), maxFalhas: maxFalhas, janela: janela, bloqueio: bloqueio}
	go m.limparPeriodicamente()
	return m
}

// Bloqueado informa se a chave está bloqueada e por quanto tempo ainda.
func (m *Memoria) Bloqueado(chave string) (bool, time.Duration) {
	if chave == "" {
		return false, 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entradas[chave]
	if !ok {
		return false, 0
	}
	if agora := time.Now(); agora.Before(e.bloqueadaAte) {
		return true, e.bloqueadaAte.Sub(agora)
	}
	return false, 0
}

// RegistrarFalha conta uma falha; ao atingir maxFalhas na janela, bloqueia.
func (m *Memoria) RegistrarFalha(chave string) {
	if chave == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	agora := time.Now()
	e, ok := m.entradas[chave]
	if !ok || agora.Sub(e.inicioJanela) > m.janela {
		e = &entrada{inicioJanela: agora}
		m.entradas[chave] = e
	}
	e.falhas++
	if e.falhas >= m.maxFalhas {
		e.bloqueadaAte = agora.Add(m.bloqueio)
	}
}

// RegistrarSucesso zera a chave (ex.: login bem-sucedido).
func (m *Memoria) RegistrarSucesso(chave string) {
	if chave == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entradas, chave)
}

func (m *Memoria) limparPeriodicamente() {
	ticker := time.NewTicker(intervaloLimpeza)
	defer ticker.Stop()
	for agora := range ticker.C {
		m.LimparVencidas(agora)
	}
}

// LimparVencidas remove as entradas com janela e bloqueio já vencidos em
// agora (a limpeza periódica chama a cada 10 min; o mapa não cresce sem
// limite) e devolve quantas restaram.
func (m *Memoria) LimparVencidas(agora time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, e := range m.entradas {
		if agora.Sub(e.inicioJanela) > m.janela && agora.After(e.bloqueadaAte) {
			delete(m.entradas, k)
		}
	}
	return len(m.entradas)
}

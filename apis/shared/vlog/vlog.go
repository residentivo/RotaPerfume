// Package vlog emite logs verbose de rastreamento ("linha a linha") no
// formato "[arquivo.go] [Funcao] descrição do passo".
//
// Só fica ativo quando VERBOSE=true (ou LOG_LEVEL=debug), via SetEnabled no
// startup de cada binário. NÃO use VERBOSE em produção.
//
// Regras-chave (spec de segurança LOG-02):
//   - Descreva O QUE o código faz, não o valor dos dados. Permitido: IDs
//     numéricos, contagens/len(), booleanos, status HTTP, papel, durações.
//   - Proibido %v/%+v/%#v em struct/map/slice de modelo, r.Body, headers,
//     cookies, query string, senhas, hashes, tokens, secrets, DSN e CNPJ.
//   - E-mail sempre via MaskEmail; strings do usuário no máximo len().
//   - SQL: só o nome da operação, nunca valores/args.
//   - Nunca logar por iteração em loops (rows.Next, importers): logue antes e
//     depois com contagem.
package vlog

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
)

var enabled atomic.Bool

// SetEnabled liga/desliga os logs verbose (chamar no startup).
func SetEnabled(on bool) { enabled.Store(on) }

// Enabled informa se os logs verbose estão ligados.
func Enabled() bool { return enabled.Load() }

// Printf emite "[arquivo] [funcao] <mensagem>" via log.Printf quando ativo.
// A flag é checada antes de qualquer formatação.
func Printf(arquivo, funcao, format string, args ...any) {
	if !enabled.Load() {
		return
	}
	log.Printf("[%s] [%s] %s", arquivo, funcao, fmt.Sprintf(format, args...))
}

// MaskEmail mascara parcialmente um e-mail para uso em logs, preservando
// apenas o primeiro caractere do usuário e o domínio completo
// (ex.: "ana.silva@empresa.com" -> "a***@empresa.com").
func MaskEmail(email string) string {
	at := strings.Index(email, "@")
	if at <= 0 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}

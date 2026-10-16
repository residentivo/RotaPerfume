// Package vlog emite logs verbose de rastreamento no formato
// "[arquivo.go] [Funcao] descrição".
//
// Só fica ativo quando VERBOSE=true (ou LOG_LEVEL=debug), via SetEnabled no
// startup de cada binário. NÃO use VERBOSE em produção.
//
// Quatro níveis (docs/arquitetura-ddd.md §6, decisão P1). Código novo emite
// só estes; o "linha a linha" do legado some à medida que cada contexto é
// migrado:
//  1. Rota: automático, pelo httpx.Trace do router ("início GET" /
//     "fim status=200"). Pode: método, status, rótulo. Nunca: path com query,
//     headers, cookies, corpo.
//  2. Caso de uso: `defer vlog.Trace(...)(&err)` na 1ª linha de cada método
//     da aplicação. Pode: IDs numéricos, contagens, page/limit, enums,
//     booleanos, Kind do erro. Nunca: %v de comando/entidade/DTO/filtro;
//     texto digitado no máximo como len() ou busca=sim/não.
//  3. Decisão: uma linha em cada ramo com significado de negócio ou
//     segurança. Pode: IDs, papel, booleanos, retry em segundos, id da linha
//     do token, e-mail só via MaskEmail/Email.Mascarado. Nunca: chave do rate
//     limit, token Turnstile/refresh/JWT, hash, senha (nem mascarados), CNPJ,
//     IP, User-Agent.
//  4. Query: uma linha por operação de repositório, com o nome da operação e
//     contagens (linhas=20 total=37). Nunca: texto do SQL, args, valores.
//
// Regras gerais (LOG-02, O7; verificadas por TestARQ_VlogSemConteudoSensivel):
//   - Proibido %v/%+v/%#v no formato.
//   - Nunca logar por iteração em loops (rows.Next, importers): antes e
//     depois, com contagem.
//   - Proibido vlog de sintaxe ("declarando variável", "verificando err") e
//     `if cfg.Verbose { log.Printf }` (vira nível 3).
//   - vlog não entra no kernel nem no domínio (puros).
package vlog

import (
	"fmt"
	"log"
	"regexp"
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

// OcultarEmail devolve o texto de err com cada e-mail trocado por
// "[email-usuario]". Servidores SMTP costumam ecoar o destinatário nas
// respostas de erro (ex.: 550 <a@b>), e o log nunca deve levar esse e-mail
// (SEC-09, NB-1). E-mail vazio é ignorado (sem nenhum, o texto volta igual).
//
// ARQ-07 NB-A: cada e-mail é aparado e a troca ignora maiúsculas (o servidor
// pode ecoar o endereço normalizado em minúsculas). NB-B: aceita vários
// (usuário e destino do envio).
func OcultarEmail(err error, emails ...string) string {
	msg := err.Error()
	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(email))
		msg = re.ReplaceAllLiteralString(msg, "[email-usuario]")
	}
	return msg
}

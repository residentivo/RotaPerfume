// Package kernel é o shared kernel do domínio (ARQ-01): tipos de valor e
// erros usados por todos os bounded contexts e pela camada HTTP.
//
// Regras (docs/arquitetura-ddd.md §3.2 e §4.1): o pacote é puro. Importa só
// a stdlib, sem database/sql, net/http, log nem pacotes do projeto (nem
// vlog). Quem loga é a aplicação.
package kernel

import "time"

// Kind classifica um erro de domínio. O mapper HTTP (httpx.ResponderErro)
// decide o status só pelo Kind, com switch explícito e default 500.
type Kind uint8

// Kinds conhecidos. KindInterno é o zero value: um erro sem classificação
// nunca vira 4xx por engano.
const (
	KindInterno Kind = iota
	KindInvalido
	KindNaoEncontrado
	KindConflito
	KindProibido
	KindNaoAutenticado
	KindBloqueado // 429; o Retry-After vem de *Bloqueio
)

// String devolve o nome estável do Kind, usado em log (vlog.Trace).
func (k Kind) String() string {
	switch k {
	case KindInterno:
		return "interno"
	case KindInvalido:
		return "invalido"
	case KindNaoEncontrado:
		return "nao_encontrado"
	case KindConflito:
		return "conflito"
	case KindProibido:
		return "proibido"
	case KindNaoAutenticado:
		return "nao_autenticado"
	case KindBloqueado:
		return "bloqueado"
	default:
		return "desconhecido"
	}
}

// Erro é um erro de domínio com mensagem pública FIXA.
//
// Só pode ser criado em declaração `var` de pacote, com Kind constante e
// mensagem literal string (TestARQ_NovoErro_SentinelaLiteral). A mensagem vai
// para a resposta HTTP; por isso nunca carrega valores.
type Erro struct {
	kind Kind
	msg  string
}

// NovoErro cria um sentinela de domínio. Use só em `var ErrX = ...`.
func NovoErro(k Kind, msg string) *Erro { return &Erro{kind: k, msg: msg} }

// Error devolve a mensagem pública fixa.
func (e *Erro) Error() string { return e.msg }

// Kind devolve a classificação do erro.
func (e *Erro) Kind() Kind { return e.kind }

// KindNome devolve o nome do Kind. Satisfaz a interface que o vlog usa para
// classificar erros sem depender deste pacote.
func (e *Erro) KindNome() string { return e.kind.String() }

// ErrInterno é o sentinela genérico de falha interna (500 "erro interno").
var ErrInterno = NovoErro(KindInterno, "erro interno")

// Bloqueio carrega o tempo de espera de um rate limit sem criar mensagem
// dinâmica: Error e Unwrap devolvem o sentinela (KindBloqueado).
type Bloqueio struct {
	Sentinela  *Erro
	RetryAfter time.Duration
}

// Error devolve a mensagem do sentinela (ou "erro interno" se ausente).
func (b *Bloqueio) Error() string {
	if b.Sentinela == nil {
		return ErrInterno.Error()
	}
	return b.Sentinela.Error()
}

// Unwrap expõe o sentinela para errors.Is/errors.As.
func (b *Bloqueio) Unwrap() error {
	if b.Sentinela == nil {
		return nil
	}
	return b.Sentinela
}

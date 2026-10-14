package httpx

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/rotaperfumes/shared/kernel"
)

const msgErroInterno = "erro interno"

// ResponderErro escreve o envelope de erro de toda a API (F-05, O6).
//
// Decide só pelo sentinela (*kernel.Erro via errors.As), com switch explícito
// sobre o Kind e default fail-closed: um Kind novo esquecido cai em 500,
// nunca em status 0 ou panic. A mensagem pública é a constante do sentinela,
// nunca err.Error() da cadeia embrulhada. tag identifica o handler no log.
func ResponderErro(w http.ResponseWriter, tag string, err error) {
	var de *kernel.Erro
	if !errors.As(err, &de) {
		interno(w, tag, err)
		return
	}
	switch de.Kind() {
	case kernel.KindInvalido:
		JSON(w, http.StatusBadRequest, nil, de.Error())
	case kernel.KindNaoEncontrado:
		JSON(w, http.StatusNotFound, nil, de.Error())
	case kernel.KindConflito:
		JSON(w, http.StatusConflict, nil, de.Error())
	case kernel.KindProibido:
		JSON(w, http.StatusForbidden, nil, de.Error())
	case kernel.KindNaoAutenticado:
		JSON(w, http.StatusUnauthorized, nil, de.Error())
	case kernel.KindBloqueado:
		ResponderBloqueado(w, err)
	default: // KindInterno, ErrEscopoInvalido, ErrPrincipalInvalido, Kind desconhecido
		interno(w, tag, err)
	}
}

// ResponderBloqueado escreve 429 com Retry-After ≥ 1 s (I-14). O tempo vem
// de *kernel.Bloqueio; a mensagem, do sentinela. Sem Bloqueio na cadeia, o
// Retry-After é 1.
func ResponderBloqueado(w http.ResponseWriter, err error) {
	segundos := 0
	var b *kernel.Bloqueio
	if errors.As(err, &b) {
		segundos = int(b.RetryAfter.Seconds()) // truncado, como o legado
	}
	if segundos < 1 {
		segundos = 1
	}
	msg := msgErroInterno
	var de *kernel.Erro
	if errors.As(err, &de) {
		msg = de.Error()
	}
	w.Header().Set("Retry-After", strconv.Itoa(segundos))
	JSON(w, http.StatusTooManyRequests, nil, msg)
}

// interno loga com tag fixa e responde 500 "erro interno" (R3).
func interno(w http.ResponseWriter, tag string, err error) {
	switch {
	case errors.Is(err, kernel.ErrEscopoInvalido), errors.Is(err, kernel.ErrEscopoSemAcesso):
		log.Printf("[escopo] invalido %s", tag) // R-e: tag fixa para alertas; sem detalhes
	case errors.Is(err, kernel.ErrPrincipalInvalido):
		log.Printf("[auth] principal invalido %s", tag)
	default:
		log.Printf("%s: %v", tag, err) // só log, nunca resposta
	}
	JSON(w, http.StatusInternalServerError, nil, msgErroInterno)
}

package vlog

import (
	"errors"
	"time"
)

// classificavel é satisfeita por kernel.Erro (e por *kernel.Bloqueio via
// Unwrap). Declarada aqui para o vlog não depender do kernel.
type classificavel interface {
	KindNome() string
}

// Trace emite o log de nível 2 (caso de uso): "início <format>" agora e
// "fim ok dur=…" ou "fim erro=<kind> dur=…" no defer. O erro é logado só pela
// classificação (Kind), nunca por err.Error().
//
// Uso, na primeira linha do caso de uso com retorno nomeado err:
//
//	defer vlog.Trace("produtos.go", "Catalogo.ListarProdutos", "page=%d limit=%d", p.Numero(), p.Limite())(&err)
func Trace(arquivo, funcao, format string, args ...any) func(errp *error) {
	if !Enabled() {
		return func(*error) {}
	}
	inicio := time.Now()
	Printf(arquivo, funcao, "início "+format, args...)
	return func(errp *error) {
		if errp != nil && *errp != nil {
			Printf(arquivo, funcao, "fim erro=%s dur=%s", classificar(*errp), time.Since(inicio))
			return
		}
		Printf(arquivo, funcao, "fim ok dur=%s", time.Since(inicio))
	}
}

// classificar devolve o Kind do erro de domínio ou "interno".
func classificar(err error) string {
	var c classificavel
	if errors.As(err, &c) {
		return c.KindNome()
	}
	return "interno"
}

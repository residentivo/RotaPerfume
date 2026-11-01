package vlog_test

// ARQ-07 (SEC-09, NB-1): o texto do erro SMTP sai do log sem o e-mail.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/vlog"
)

func TestARQ07_OcultarEmail(t *testing.T) {
	err := errors.New("550 <ana@x.com>: mailbox ana@x.com unavailable")
	assert.Equal(t, "550 <[email-usuario]>: mailbox [email-usuario] unavailable", vlog.OcultarEmail(err, "ana@x.com"))
	assert.Equal(t, err.Error(), vlog.OcultarEmail(err, ""), "sem e-mail conhecido, nada a ocultar")
	assert.Equal(t, err.Error(), vlog.OcultarEmail(err, "outra@x.com"))
}

// ARQ-07 NB-A: e-mail cadastrado com maiúsculas/espaço e servidor ecoando em
// minúsculas também é ocultado; e-mail só de espaços não troca nada.
func TestARQ07_NBA_OcultarEmailIgnoraCaixaEEspacos(t *testing.T) {
	err := errors.New("550 <ana.silva@x.com>: user ANA.SILVA@X.COM unknown")
	assert.Equal(t, "550 <[email-usuario]>: user [email-usuario] unknown", vlog.OcultarEmail(err, "Ana.Silva@X.com "))
	assert.Equal(t, err.Error(), vlog.OcultarEmail(err, "   "), "e-mail vazio após aparar: nada a ocultar")

	meta := errors.New("550 a+b.c@x.com rejeitado; aXb+c@x.com intacto")
	assert.Equal(t, "550 [email-usuario] rejeitado; aXb+c@x.com intacto", vlog.OcultarEmail(meta, "a+b.c@x.com"),
		"metacaracteres do e-mail são literais")
}

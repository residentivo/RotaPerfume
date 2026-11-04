package deploy_test

// DEPLOY-08 (2026-11-04): no servidor, o "openssl s_client" na porta
// publicada morria sem handshake (o mariadbd manda o erro 1130 ao próprio
// servidor, que não tem conta). O stub antigo do timeout sempre respondia
// "Verify return code: 0 (ok)" e mascarava isso. Agora o script lê os
// primeiros bytes da porta (greeting ou erro 1130 = mariadbd; resto = die),
// confere o certificado offline (-verify_ip) e o certificado em uso.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDEPLOY08_PortaPublicada(t *testing.T) {
	casos := []struct {
		nome     string
		resposta *[]byte // nil = sem bytes (timeout)
		passa    bool
		msg      string
	}{
		{"erro 1130 passa", ptr(pacoteErro1130("192.168.168.106")), true, "mariadbd respondeu (erro 1130"},
		{"greeting passa", ptr(pacoteGreeting()), true, "greeting do mariadbd: OK."},
		{"timeout sem bytes aborta", nil, false, "inesperada:nada"},
		{"outro servico aborta", ptr([]byte("HTTP/1.1 400 Bad Request\r\n\r\n")), false, "esperado greeting ou erro 1130"},
		{"outro erro do mariadbd aborta", ptr([]byte{0x05, 0, 0, 0, 0xff, 0x10, 0x04, 'x', 'y'}), false, "esperado greeting ou erro 1130"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaio(t)
			if c.resposta == nil {
				require.NoError(t, removerState(e, "porta_resposta"))
			} else {
				e.setState("porta_resposta", string(*c.resposta))
			}
			r := e.rodar("--nova-ca", "--dev-ip", "192.168.168.105")
			assert.Contains(t, r.out, c.msg)
			if c.passa {
				require.NoError(t, r.err, r.out)
				assert.Contains(t, r.out, "\nPronto. MariaDB")
				return
			}
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, "a porta publicada 192.168.168.106:3306 nao respondeu como o mariadbd")
			assert.NotContains(t, r.out, "\nPronto. MariaDB")
		})
	}
}

func TestDEPLOY08_CertificadoEmUsoDiferenteAborta(t *testing.T) {
	e := novoEnsaio(t)
	e.setState("cert_em_uso", "Jan  1 00:00:00 2030 GMT|Jan  1 00:00:00 2028 GMT\n")
	r := e.rodar("--nova-ca")
	require.Error(t, r.err, r.out)
	assert.Contains(t, r.out, "nao e o de ")
	assert.Contains(t, r.out, "--novo-cert")
	assert.NotContains(t, r.out, "\nPronto. MariaDB")
}

func TestDEPLOY08_HealthcheckDaImagem(t *testing.T) {
	t.Run("so USAGE: sem aviso", func(t *testing.T) {
		e := novoEnsaio(t)
		r := e.rodar("--nova-ca", "--dev-ip", "192.168.168.105")
		require.NoError(t, r.err, r.out)
		assert.NotContains(t, r.out, "contas fora do esperado")
		assert.NotContains(t, r.out, "healthcheck locais com privilegios")
		// A consulta das "outras" exclui só os healthcheck locais.
		assert.Contains(t, e.state("sql_outras"), "NOT (User = 'healthcheck' AND Host IN ('localhost', '127.0.0.1', '::1'))")
		hc := e.state("sql_healthcheck")
		for _, tab := range []string{"$.access", "mysql.db", "mysql.tables_priv", "mysql.columns_priv", "mysql.procs_priv", "mysql.roles_mapping", "mysql.proxies_priv"} {
			assert.Contains(t, hc, tab)
		}
		assert.NotContains(t, e.state("sql"), "healthcheck", "nunca remove as contas healthcheck")
	})
	t.Run("alem de USAGE: aviso", func(t *testing.T) {
		e := novoEnsaio(t)
		e.setState("healthcheck_alem_usage", "healthcheck@127.0.0.1\n")
		r := e.rodar("--nova-ca", "--dev-ip", "192.168.168.105")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "contas healthcheck locais com privilegios ALEM de USAGE: healthcheck@127.0.0.1")
		assert.NotContains(t, e.state("sql"), "DROP USER IF EXISTS 'healthcheck'")
	})
	t.Run("healthcheck remoto continua como conta fora do esperado", func(t *testing.T) {
		e := novoEnsaio(t)
		e.setState("outras", "healthcheck@%\n")
		r := e.rodar("--nova-ca")
		require.NoError(t, r.err, r.out)
		assert.True(t, strings.Contains(r.out, "contas fora do esperado") && strings.Contains(r.out, "healthcheck@%"), r.out)
	})
}

func ptr(b []byte) *[]byte { return &b }

func removerState(e *ensaio, nome string) error {
	return os.Remove(filepath.Join(e.dir, "state", nome))
}

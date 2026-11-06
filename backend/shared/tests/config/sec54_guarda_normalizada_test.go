package config_test

// SEC-54 (Lote 35): a guarda de produção normaliza o DB_HOST do mesmo jeito
// no Makefile (NORMALIZA_HOST/PRODUCAO_DETECTA) e no Go
// (testkit/integracao), sem DNS, e os testes de integração usam uma
// allowlist (INTEGRACAO_PERMITIDOS / integracao.Permitidos): loopback em
// qualquer porta e o CasaOS de dev (192.168.168.106) só na 3307.
// Uma tabela única roda contra o make (cópia do Makefile, sem .env) e contra
// ValidarNaoProducao/ValidarPermitidoIntegracao. Nada conecta em banco.

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/testkit/integracao"
)

type casoSEC54 struct {
	host, porta string
	producao    bool // db-guard-producao recusa / ValidarNaoProducao erra
	integracao  bool // db-guard-integracao aceita / as duas validações passam
}

var casosSEC54 = []casoSEC54{
	// Aliases do servidor de produção (o achado do SEC-54).
	{"IVO-INSPIRON-15-3530.local.", "3306", true, false},
	{"ivo-inspiron-15-3530.local", "", true, false},
	{"ivo-inspiron-15-3530.local", "3307", false, false}, // .local nunca na allowlist
	{"ivo-inspiron-15-3530..", "3306", true, false},
	{"[::ffff:192.168.168.106]", "3306", true, false},
	{"::FFFF:192.168.168.106", "", true, false},
	{"0:0:0:0:0:ffff:192.168.168.106", "3306", true, false},
	{"[::ffff:192.168.168.106]", "3307", false, true}, // é o CasaOS
	{"::ffff:c0a8:a86a", "3307", true, false},         // IPv6 não mapeado textualmente: fail-closed
	{"fe80::1", "3307", true, false},
	{"[2001:db8::1]", "3307", true, false},
	{"ROTAPERFUMES-MARIADB", "3307", true, false},
	{"172.30.42.2.", "1", true, false},
	// Porta: número com "+"/zeros; não numérica é recusada em qualquer host.
	{"192.168.168.106", "+03306", true, false},
	{"192.168.168.106", "abc", true, false},
	{"localhost", "abc", true, false},
	{"localhost", "+", true, false},
	{"127.0.0.1", "-1", true, false},
	{"192.168.168.106", "3307", false, true},
	{"192.168.168.106", "03307", false, true},
	{"192.168.168.106", "+3307", false, true},
	{"192.168.168.106.", "3307", false, true},
	{"192.168.168.106", "33070", false, false},
	{"192.168.168.106", "3308", false, false},
	// Loopback em qualquer porta.
	{"127.0.0.1", "3306", false, true},
	{"127.10.20.30", "13306", false, true},
	{"localhost", "000", false, true}, // zeros à esquerda: porta 0
	{"LOCALHOST.", "", false, true},
	{"::1", "3306", false, true},
	{"[::1]", "", false, true},
	{"::ffff:127.0.0.1", "3306", false, true},
	{"localhost.local", "3306", false, false},
	// Fora das duas listas: db-* passam (blocklist), integração não.
	{"192.168.168.107", "3306", false, false},
	{"outro-host", "3307", false, false},
	{"127.0.0.1.nip.io", "3306", false, false},
	{"", "", false, false},
	// TestBrain (Lote 35): bordas extras da normalização.
	{"[::1].", "3306", true, false},     // "]" não é o fim: literal com ":" fail-closed
	{"127.0.0.1:3306", "", true, false}, // porta embutida no host
	{"ivo-inspiron-15-3530.LOCAL", "3306", true, false},
	{"192.168.168.106.local", "3307", false, false}, // .local nunca na allowlist
	{"::ffff:c0a8:a86a", "3306", true, false},
	{"0:0:0:0:0:FFFF:127.0.0.1", "", false, true},
	{"localhost..", "3306", false, true},
	{"127.0.0.1", "0003307", false, true},
	// SEC-56 (Lote 37): IPv4 numérico não canônico é recusado, sem válvula.
	{"3232278634", "3307", true, false},
	{"3232278634", "", true, false},
	{"0xc0a8a86a", "3307", true, false},
	{"0XC0A8A86A", "3307", true, false},
	{"192.168.43114", "3307", true, false},
	{"0300.0250.0250.0152", "3307", true, false},
	{"192.168.168.0106", "3307", true, false},
	{"192.0250.168.106", "3307", true, false},
	{"2130706433", "3307", true, false},
	{"127.1", "3307", true, false},
	{"127.000.000.001", "3307", true, false},
	{"0x7f.1", "3307", true, false},
	{"0", "3307", true, false},
	{"0x", "3307", true, false},
	{"192.168.168.256", "3307", true, false},
	{"1.2.3.4.5", "3307", true, false},
	{"[::ffff:010.1.1.1]", "3307", true, false},
	// SEC-56: nomes que parecem hex continuam nomes (fora das duas listas).
	{"cafe", "3307", false, false},
	{"db1.lan", "3307", false, false},
	{"0xg", "3307", false, false},
	// SEC-57 (Lote 37): espaço na porta é porta inválida (sem TrimSpace).
	// Espaço À ESQUERDA (" 3307", "\t3307", "  ") não entra na paridade: o
	// próprio make tira o branco inicial de "DB_PORT= 3307" na linha de
	// comando (o cliente recebe 3307); no Go é recusado (TestSEC56_SEC57_Go).
	{"127.0.0.1", "3307 ", true, false},
	{"127.0.0.1", "33 07", true, false},
	{"127.0.0.1", "3307\t", true, false},
	{"192.168.168.106", "3307 ", true, false},
	{"localhost", "3307  ", true, false},
}

func TestSEC54_Paridade_MakefileEGo(t *testing.T) {
	for _, c := range casosSEC54 {
		t.Run(c.host+":"+c.porta, func(t *testing.T) {
			args := []string{"DB_HOST=" + c.host, "DB_PORT=" + c.porta, "DB_NAME=rotaperfumes"}
			outProd, errProd := makeAlvo(t, copiaDoMakefile(t), "db-guard-producao", nil, args...)
			outInt, errInt := makeAlvo(t, copiaDoMakefile(t), "db-guard-integracao", nil, args...)

			goProd := integracao.ValidarNaoProducao(c.host, c.porta)
			goInt := integracao.ValidarPermitidoIntegracao(c.host, c.porta)

			assert.Equal(t, c.producao, errProd != nil, "Makefile db-guard-producao: %s", outProd)
			assert.Equal(t, c.producao, goProd != nil, "Go ValidarNaoProducao: %v", goProd)
			assert.Equal(t, c.integracao, errInt == nil, "Makefile db-guard-integracao: %s", outInt)
			assert.Equal(t, c.integracao, goProd == nil && goInt == nil, "Go integração: %v / %v", goProd, goInt)
			if errInt != nil {
				assert.Contains(t, outInt, "SEC-54: DB_HOST=")
				assert.Contains(t, outInt, "fora da allowlist da integracao")
			}
		})
	}
}

func TestSEC54_ConfigDoAmbiente_Allowlist(t *testing.T) {
	casos := []struct {
		nome string
		vars map[string]string
		erro error
	}{
		{"host fora da allowlist", credenciais(map[string]string{"DB_HOST": "192.168.168.107", "DB_PORT": "3306", "DB_TLS_CA": "/inexistente-sec54.pem"}), integracao.ErrHostNaoPermitido},
		{"casaos em outra porta", credenciais(map[string]string{"DB_HOST": "192.168.168.106", "DB_PORT": "3308"}), integracao.ErrHostNaoPermitido},
		{".local no loopback", credenciais(map[string]string{"DB_HOST": "localhost.local"}), integracao.ErrHostNaoPermitido},
		{"alias .local da produção", credenciais(map[string]string{"DB_HOST": "ivo-inspiron-15-3530.local."}), integracao.ErrBancoProducao},
		{"IPv4 mapeado da produção", credenciais(map[string]string{"DB_HOST": "[::ffff:192.168.168.106]", "DB_PORT": "3306"}), integracao.ErrBancoProducao},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			ambienteIntegracao(t, c.vars)
			cfg, err := integracao.ConfigDoAmbiente()
			require.ErrorIs(t, err, c.erro)
			assert.Nil(t, cfg)
			assert.NotContains(t, err.Error(), "s-sec50", "a senha não aparece no erro")
		})
	}
	t.Run("ipv6 loopback passa", func(t *testing.T) {
		ambienteIntegracao(t, credenciais(map[string]string{"DB_HOST": "::1", "DB_PORT": "3310"}))
		cfg, err := integracao.ConfigDoAmbiente()
		require.NoError(t, err)
		assert.Equal(t, "::1", cfg.DBHost)
	})
	t.Run("fora da allowlist: Config falha (t.Fatal), não pula", func(t *testing.T) {
		ambienteIntegracao(t, credenciais(map[string]string{"DB_HOST": "db.exemplo.lan", "DB_PORT": "3306"}))
		tb := &tbFalso{}
		assert.Nil(t, integracao.Abrir(tb))
		assert.True(t, tb.falhou)
		assert.False(t, tb.pulou)
		assert.Contains(t, tb.msg, "SEC-54")
		assert.Empty(t, tb.limpezas, "nada aberto")
	})
}

func TestSEC54_PortaNaoNumericaEmQualquerHost(t *testing.T) {
	for _, h := range []string{"localhost", "127.0.0.1", "192.168.168.106", "outro-host"} {
		err := integracao.ValidarNaoProducao(h, "33o6")
		require.Error(t, err, h)
		assert.False(t, errors.Is(err, integracao.ErrBancoProducao), h)
		assert.Contains(t, err.Error(), "não é numérico")
		assert.ErrorIs(t, integracao.ValidarPermitidoIntegracao(h, "33o6"), integracao.ErrHostNaoPermitido, h)
	}
}

func TestSEC54_ListaPermitidosIgualAoMakefile(t *testing.T) {
	mk := lerMakefile(t)
	permitidos := strings.Fields(strings.SplitN(mk.linhaDaVariavel(t, "INTEGRACAO_PERMITIDOS"), "=", 2)[1])
	assert.Equal(t, permitidos, integracao.Permitidos, "INTEGRACAO_PERMITIDOS do Makefile == integracao.Permitidos")
	assert.Equal(t, []string{"localhost", "127.0.0.0/8", "::1", "192.168.168.106:3307"}, integracao.Permitidos)

	assert.True(t, mk.phony["db-guard-integracao"], "db-guard-integracao no .PHONY")
	assert.Equal(t, []string{"db-guard-producao", "db-guard-integracao"}, mk.regras["test-integration"].prereqs)
	assert.Contains(t, mk.linhaDaVariavel(t, "GUARDA_INTEGRACAO"), "db-guard-producao db-guard-integracao")
	// A mesma normalização nas três guardas e no db-check-env.
	assert.Contains(t, mk.linhaDaVariavel(t, "PRODUCAO_DETECTA"), "$(NORMALIZA_HOST)")
	for _, a := range []string{"db-guard-drop", "db-check-env"} {
		receita := strings.Join(mk.regras[a].receita, "\n")
		assert.Contains(t, receita, "$(NORMALIZA_HOST)", a)
		assert.Contains(t, receita, "$(HOST_LOOPBACK)", a)
	}
	assert.Contains(t, strings.Join(mk.regras["db-guard-integracao"].receita, "\n"), "$(PRODUCAO_DETECTA)")
	// Os db-* continuam só com a blocklist.
	for _, a := range []string{"db-down", "db-seed", "db-reset", "db-rebuild", "db-up"} {
		assert.False(t, mk.dependeDe(a, "db-guard-integracao"), a)
	}
}

// Loopback normalizado no db-guard-drop: "[::1]", "localhost." e IPv4
// mapeado passam calados; ".local" exige CONFIRMAR.
func TestSEC54_GuardaDropLoopbackNormalizado(t *testing.T) {
	casos := []struct {
		host   string
		recusa bool
	}{
		{"[::1]", false},
		{"LOCALHOST.", false},
		{"::ffff:127.0.0.1", false},
		{"[127.0.0.1]", false},
		{"localhost.local", true},
		{"::ffff:192.168.168.106", true},
		{"fe80::1", true},
	}
	for _, c := range casos {
		t.Run(c.host, func(t *testing.T) {
			out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-drop", nil, "DB_HOST="+c.host, "DB_PORT=3307", "DB_NAME=rotaperfumes")
			if c.recusa {
				require.Error(t, err, out)
				assert.Contains(t, out, "(fora do loopback): repita com CONFIRMAR=")
				return
			}
			assert.NoError(t, err, out)
			assert.Empty(t, strings.TrimSpace(out))
		})
	}
}

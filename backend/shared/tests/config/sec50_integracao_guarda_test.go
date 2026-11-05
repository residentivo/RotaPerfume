package config_test

// SEC-50 (Lote 34): guarda de produção dos testes de integração Go
// (shared/testkit/integracao), sem banco:
//   - ValidarNaoProducao com a MESMA tabela do db-guard-producao (DEPLOY-07);
//   - paridade das listas de hosts com o Makefile;
//   - a recusa vem antes de qualquer conexão e antes de ler a CA (TLS);
//   - Config/Abrir: pula sem INTEGRATION=1/credenciais, falha na produção;
//   - catraca: nenhum _test.go de backend/*/tests lê INTEGRATION direto.
// Nunca conecta em banco: o único Abrir "de verdade" vai para 127.0.0.1:1.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/testkit/integracao"
)

func TestSEC50_ValidarNaoProducao_MesmaTabelaDoMakefile(t *testing.T) {
	for _, c := range casosGuardaProducao {
		t.Run(c.host+":"+c.porta, func(t *testing.T) {
			err := integracao.ValidarNaoProducao(c.host, c.porta)
			if c.recusa {
				require.ErrorIs(t, err, integracao.ErrBancoProducao)
				assert.Contains(t, err.Error(), "é o banco de PRODUÇÃO; teste de integração recusado antes de conectar (SEC-50)")
				assert.Contains(t, err.Error(), fmt.Sprintf("DB_HOST=%q DB_PORT=%q", c.host, c.porta))
			} else {
				assert.NoError(t, err)
			}
		})
	}
	extras := []struct {
		host, porta string
		recusa      bool
	}{
		{" 192.168.168.106 ", "3306", true},
		{"192.168.168.106", " 3306 ", true}, // espaços nas pontas
		{"192.168.168.106.", "3306", true},  // "." final
		{"ivo-inspiron-15-3530", "", true},
		{"rotaperfumes-mariadb", "", true},
	}
	for _, c := range extras {
		t.Run("extra/"+c.host+":"+c.porta, func(t *testing.T) {
			err := integracao.ValidarNaoProducao(c.host, c.porta)
			assert.Equal(t, c.recusa, errors.Is(err, integracao.ErrBancoProducao), "%v", err)
		})
	}
	t.Run("porta não numérica no host da LAN", func(t *testing.T) {
		err := integracao.ValidarNaoProducao("192.168.168.106", "abc")
		require.Error(t, err)
		assert.NotErrorIs(t, err, integracao.ErrBancoProducao)
		assert.Contains(t, err.Error(), "não é numérico")
	})
}

func TestSEC50_ListasIguaisAoMakefile(t *testing.T) {
	mk := lerMakefile(t)
	lan := strings.Fields(strings.SplitN(mk.linhaDaVariavel(t, "PRODUCAO_HOSTS_LAN"), "=", 2)[1])
	rede := strings.Fields(strings.SplitN(mk.linhaDaVariavel(t, "PRODUCAO_HOSTS_REDE"), "=", 2)[1])
	assert.Equal(t, lan, integracao.ProducaoHostsLAN, "PRODUCAO_HOSTS_LAN do Makefile == integracao.ProducaoHostsLAN")
	assert.Equal(t, rede, integracao.ProducaoHostsRede, "PRODUCAO_HOSTS_REDE do Makefile == integracao.ProducaoHostsRede")
	// SEC-54: a allowlist da integração também é a mesma nos dois lados.
	permitidos := strings.Fields(strings.SplitN(mk.linhaDaVariavel(t, "INTEGRACAO_PERMITIDOS"), "=", 2)[1])
	assert.Equal(t, permitidos, integracao.Permitidos, "INTEGRACAO_PERMITIDOS do Makefile == integracao.Permitidos")
}

// ambienteIntegracao limpa as variáveis lidas pelo helper e aplica vars.
func ambienteIntegracao(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, k := range []string{"INTEGRATION", "DB_HOST", "DB_PORT", "DB_NAME", "DB_USUARIO", "DB_SENHA",
		"DB_TLS_CA", "DB_TLS_SERVER_NAME", "DB_TLS_REQUIRED"} {
		t.Setenv(k, vars[k])
	}
}

func credenciais(extra map[string]string) map[string]string {
	m := map[string]string{"INTEGRATION": "1", "DB_USUARIO": "u-sec50", "DB_SENHA": "s-sec50"}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestSEC50_ConfigDoAmbiente(t *testing.T) {
	casos := []struct {
		nome string
		vars map[string]string
		erro error // nil = ok
	}{
		{"sem INTEGRATION", map[string]string{"DB_USUARIO": "u", "DB_SENHA": "s"}, integracao.ErrSemIntegracao},
		{"INTEGRATION=true não vale", map[string]string{"INTEGRATION": "true", "DB_USUARIO": "u", "DB_SENHA": "s"}, integracao.ErrSemIntegracao},
		{"sem usuário", map[string]string{"INTEGRATION": "1", "DB_SENHA": "s"}, integracao.ErrSemCredenciais},
		{"sem senha", map[string]string{"INTEGRATION": "1", "DB_USUARIO": "u"}, integracao.ErrSemCredenciais},
		// A recusa da produção vem antes do LoadDBTLS: a CA inexistente nem é lida.
		{"produção antes da CA", credenciais(map[string]string{"DB_HOST": "192.168.168.106", "DB_PORT": "3306", "DB_TLS_CA": "/inexistente-sec50.pem"}), integracao.ErrBancoProducao},
		{"produção sem porta (padrão 3306)", credenciais(map[string]string{"DB_HOST": "192.168.168.106"}), integracao.ErrBancoProducao},
		{"rede docker", credenciais(map[string]string{"DB_HOST": "rotaperfumes-mariadb", "DB_PORT": "3307"}), integracao.ErrBancoProducao},
		{"casaos chega à CA", credenciais(map[string]string{"DB_HOST": "192.168.168.106", "DB_PORT": "3307", "DB_TLS_CA": "/inexistente-sec50.pem"}), config.ErrDBTLSCA},
		{"casaos sem TLS (SEC-42)", credenciais(map[string]string{"DB_HOST": "192.168.168.106", "DB_PORT": "3307"}), config.ErrDBTLSObrigatorio},
		{"DB_TLS_REQUIRED inválido", credenciais(map[string]string{"DB_HOST": "127.0.0.1", "DB_TLS_REQUIRED": "talvez"}), config.ErrDBTLSRequired},
		{"loopback sem CA", credenciais(map[string]string{"DB_HOST": "127.0.0.1"}), nil},
		{"padrão localhost", credenciais(nil), nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			ambienteIntegracao(t, c.vars)
			cfg, err := integracao.ConfigDoAmbiente()
			if c.erro != nil {
				require.ErrorIs(t, err, c.erro)
				assert.Nil(t, cfg)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, "u-sec50", cfg.DBUsuario)
			assert.Equal(t, "rotaperfumes", cfg.DBName)
			assert.Equal(t, "3306", cfg.DBPort)
			assert.False(t, cfg.DBTLS.Ativo())
		})
	}
	t.Run("padrões de host e porta", func(t *testing.T) {
		ambienteIntegracao(t, credenciais(nil))
		cfg, err := integracao.ConfigDoAmbiente()
		require.NoError(t, err)
		assert.Equal(t, "localhost", cfg.DBHost)
	})
}

// tbFalso registra Skip/Fatalf sem encerrar a goroutine (o helper devolve
// nil logo depois), para cobrir os ramos de Config/Abrir.
type tbFalso struct {
	testing.TB
	pulou, falhou bool
	msg           string
	limpezas      []func()
}

func (f *tbFalso) Helper()          {}
func (f *tbFalso) Skip(args ...any) { f.pulou, f.msg = true, fmt.Sprint(args...) }
func (f *tbFalso) Fatalf(format string, args ...any) {
	f.falhou, f.msg = true, fmt.Sprintf(format, args...)
}
func (f *tbFalso) Cleanup(fn func()) { f.limpezas = append(f.limpezas, fn) }

func TestSEC50_ConfigEAbrir(t *testing.T) {
	t.Run("sem INTEGRATION pula", func(t *testing.T) {
		ambienteIntegracao(t, nil)
		tb := &tbFalso{}
		assert.Nil(t, integracao.Config(tb))
		assert.True(t, tb.pulou)
		assert.Contains(t, tb.msg, "defina INTEGRATION=1")
		tb = &tbFalso{}
		assert.Nil(t, integracao.Abrir(tb))
		assert.True(t, tb.pulou)
	})
	t.Run("sem credenciais pula", func(t *testing.T) {
		ambienteIntegracao(t, map[string]string{"INTEGRATION": "1"})
		tb := &tbFalso{}
		assert.Nil(t, integracao.Config(tb))
		assert.True(t, tb.pulou)
		assert.Contains(t, tb.msg, "DB_USUARIO/DB_SENHA")
	})
	t.Run("produção falha", func(t *testing.T) {
		ambienteIntegracao(t, credenciais(map[string]string{"DB_HOST": "192.168.168.106", "DB_PORT": "3306"}))
		tb := &tbFalso{}
		assert.Nil(t, integracao.Abrir(tb))
		assert.True(t, tb.falhou)
		assert.False(t, tb.pulou)
		assert.Contains(t, tb.msg, "PRODUÇÃO")
		assert.NotContains(t, tb.msg, "s-sec50")
		assert.Empty(t, tb.limpezas, "nada aberto")
	})
	t.Run("loopback sem servidor falha no Open", func(t *testing.T) {
		ambienteIntegracao(t, credenciais(map[string]string{"DB_HOST": "127.0.0.1", "DB_PORT": "1"}))
		tb := &tbFalso{}
		require.NotNil(t, integracao.Config(tb))
		assert.False(t, tb.pulou || tb.falhou)
		assert.Nil(t, integracao.Abrir(tb))
		assert.True(t, tb.falhou)
		assert.Contains(t, tb.msg, "MariaDB de integração indisponível")
		assert.NotContains(t, tb.msg, "s-sec50", "a senha não aparece na falha")
	})
}

// Catraca: só o testkit lê INTEGRATION. Um _test.go novo que volte a ler a
// variável direto escaparia da guarda de produção.
func TestSEC50_NenhumTesteLeIntegrationDireto(t *testing.T) {
	backend := filepath.Join(raizProjeto(t), "backend")
	proibido := "Getenv(" + `"INTEGRATION"` + ")"
	proibido2 := "LookupEnv(" + `"INTEGRATION"` + ")"
	var vistos int
	var achados []string
	for _, mod := range []string{"shared", "rotaperfumes-api"} {
		raiz := filepath.Join(backend, mod, "tests")
		err := filepath.WalkDir(raiz, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(p, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			vistos++
			if s := string(b); strings.Contains(s, proibido) || strings.Contains(s, proibido2) {
				achados = append(achados, p)
			}
			return nil
		})
		require.NoError(t, err)
	}
	assert.Greater(t, vistos, 50, "a varredura encontrou os testes")
	assert.Empty(t, achados, "use integracao.Config/Abrir (SEC-50)")
}

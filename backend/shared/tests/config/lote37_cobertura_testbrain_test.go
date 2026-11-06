package config_test

// Lote 37 (TestBrain): cobertura complementar de BUG-29, SEC-56 e SEC-57
// contra os "casos de teste mínimos" do spec do SecBrain, além do que o
// BackBrain já cobriu em lote37_makefile_bug29_sec56_test.go:
//   - BUG-29: CONFIRMO_PRODUCAO só no ambiente, "make -e" com ENV_ARQUIVO
//     do ambiente, MAKEFLAGS vazio na linha de comando, test-integration com
//     ENV_ARQUIVO do ambiente e com -j2, sentinela coerente aceita;
//   - SEC-56: todos os hosts numéricos não canônicos do spec, com e sem
//     CONFIRMO_PRODUCAO, nas três guardas e no Go; loopback só canônico no
//     db-guard-drop e no db-check-env;
//   - SEC-57: portas com "+" mal posto, sinal, hex e tab no fim; formas
//     válidas ("+3307", "03307", vazio) seguem aceitas; DB_PORT do .env com
//     espaço no fim é strip-ado (comportamento documentado);
//     ConfigDoAmbiente devolve ErrPortaInvalida.
// Nada conecta em banco (cliente mariadb e go falsos do bug27Dir).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/testkit/integracao"
)

func TestLote37_BUG29_RecusasComplementares(t *testing.T) {
	casos := []struct {
		nome, alvo string
		env, args  []string
		msg        string
	}{
		{"MAKEFLAGS vazio na linha de comando", "db-reset", nil, []string{"MAKEFLAGS=", "ENV_ARQUIVO=a.env"}, "MAKEFLAGS/MAKEOVERRIDES/GNUMAKEFLAGS na linha de comando"},
		{"MAKEOVERRIDES= com db-reset -k", "db-reset", nil, []string{"-k", "MAKEOVERRIDES=", "ENV_ARQUIVO=a.env"}, "MAKEFLAGS/MAKEOVERRIDES/GNUMAKEFLAGS na linha de comando"},
		{"test-integration -j2 MAKEFLAGS=-k", "test-integration", nil, []string{"-j2", "MAKEFLAGS=-k", "ENV_ARQUIVO=a.env"}, "MAKEFLAGS/MAKEOVERRIDES/GNUMAKEFLAGS na linha de comando"},
		{"test-integration com ENV_ARQUIVO do ambiente", "test-integration", []string{"ENV_ARQUIVO=a.env"}, []string{"-k"}, "ENV_ARQUIVO veio do ambiente"},
		{"make -e com ENV_ARQUIVO do ambiente", "db-reset", []string{"ENV_ARQUIVO=a.env"}, []string{"-e", "-k"}, "ENV_ARQUIVO veio do ambiente"},
		{"MAKEFLAGS=-e no ambiente com ENV_ARQUIVO do ambiente", "db-down", []string{"MAKEFLAGS=-e", "ENV_ARQUIVO=a.env"}, nil, "ENV_ARQUIVO veio do ambiente"},
		{"sentinela diverge do ENV_ARQUIVO", "db-down", nil, []string{"ENV_ARQUIVO=a.env", "RP_ENV_PAI=.env"}, "o make pai leu .env e este sub-make leria a.env"},
		{"ENV_ARQUIVO com $", "db-down", nil, []string{"ENV_ARQUIVO=a$$b.env"}, "invalido: um caminho so com"},
		{"ENV_ARQUIVO com ;", "db-down", nil, []string{"ENV_ARQUIVO=a;b.env"}, "invalido: um caminho so com"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir, path, cliente, log := bug29Dir(t)
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "backend", "rotaperfumes-api"), 0o755))
			out, err := makeAlvo(t, dir, c.alvo, append([]string{path}, c.env...), bug29Args(cliente, c.args...)...)
			require.Error(t, err, out)
			assert.Contains(t, out, "BUG-29: ")
			assert.Contains(t, out, c.msg)
			assert.Empty(t, lerSeExiste(t, log), "nenhum cliente/go test antes da recusa")
		})
	}
}

func TestLote37_BUG29_PositivosComplementares(t *testing.T) {
	t.Run("sentinela igual ao ENV_ARQUIVO é aceita", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		out, err := makeAlvo(t, dir, "db-down", []string{path}, bug29Args(cliente, "ENV_ARQUIVO=a.env", "RP_ENV_PAI=a.env")...)
		require.NoError(t, err, out)
		texto := lerSeExiste(t, log)
		assert.Contains(t, texto, "-h 127.0.0.1 ")
		assert.Contains(t, texto, "DROP DATABASE IF EXISTS rp_a;")
		assert.NotContains(t, texto, "rp_b")
	})
	t.Run("GNUMAKEFLAGS no ambiente é aceito", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		out, err := makeAlvo(t, dir, "db-reset", []string{path, "GNUMAKEFLAGS=-k"}, bug29Args(cliente, "ENV_ARQUIVO=a.env")...)
		require.NoError(t, err, out)
		texto := lerSeExiste(t, log)
		assert.Contains(t, texto, "rp_a")
		assert.NotContains(t, texto, "rp_b")
		assert.NotContains(t, texto, "127.0.0.2")
	})
	t.Run("test-integration -k -j2 lê o a.env no sub-make", func(t *testing.T) {
		dir, path, cliente, log := bug29Dir(t)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "backend", "rotaperfumes-api"), 0o755))
		out, err := makeAlvo(t, dir, "test-integration", []string{path}, bug29Args(cliente, "-k", "-j2", "ENV_ARQUIVO=a.env")...)
		require.NoError(t, err, out)
		assert.Contains(t, lerSeExiste(t, log), "go test ./...")
	})
}

// CONFIRMO_PRODUCAO só no ambiente nunca abre a válvula (nem com -k, nem no
// sub-make do db-seed).
func TestLote37_BUG29_ConfirmoProducaoSoNoAmbiente(t *testing.T) {
	for _, alvo := range []string{"db-guard-producao", "db-create", "db-up"} {
		t.Run(alvo, func(t *testing.T) {
			dir, path, cliente, log := bug27Dir(t, false)
			out, err := makeAlvo(t, dir, alvo, []string{path, "CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes"},
				bug27Args(cliente, "-k", "DB_HOST=192.168.168.106", "DB_PORT=3306")...)
			require.Error(t, err, out)
			assert.Contains(t, out, "DEPLOY-07: DB_HOST=192.168.168.106 DB_PORT=3306 e o banco de PRODUCAO")
			assert.NotContains(t, out, "AVISO (DEPLOY-07)")
			assert.Empty(t, lerSeExiste(t, log))
		})
	}
}

// hostsSEC56 são os recusados novos do spec (todos fora da forma a.b.c.d).
var hostsSEC56 = []string{
	"3232278634", "0xc0a8a86a", "0XC0A8A86A", "192.168.43114", "0300.0250.0250.0152",
	"192.168.168.0106", "192.0250.168.106", "2130706433", "127.1", "127.000.000.001",
	"0x7f.1", "0", "0x", "192.168.168.256", "1.2.3.4.5", "[::ffff:010.1.1.1]",
	"0x7f000001.", "127.0.0.01", // "  127.1  " (espaços): recusado antes, SEC-63 (lote40)
}

func TestLote37_SEC56_TodosOsHostsSemValvula(t *testing.T) {
	for _, h := range hostsSEC56 {
		for _, porta := range []string{"3307", ""} {
			t.Run(h+":"+porta, func(t *testing.T) {
				// Makefile: db-guard-producao recusa com a linha SEC-56, mesmo
				// com CONFIRMO_PRODUCAO exato na linha de comando.
				for _, confirma := range []bool{false, true} {
					args := []string{"DB_HOST=" + h, "DB_PORT=" + porta, "DB_NAME=rotaperfumes"}
					if confirma {
						args = append(args, "CONFIRMO_PRODUCAO="+h+"/rotaperfumes")
					}
					out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-producao", nil, args...)
					require.Error(t, err, out)
					assert.Contains(t, out, "SEC-56: DB_HOST=")
					assert.Contains(t, out, "sem valvula")
					assert.NotContains(t, out, "AVISO (DEPLOY-07)")
					assert.NotContains(t, out, "e o banco de PRODUCAO")
				}
				// As outras duas guardas imprimem a SEC-56 e mantêm a própria recusa.
				args := []string{"DB_HOST=" + h, "DB_PORT=" + porta}
				out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-sem-producao", nil, args...)
				require.Error(t, err, out)
				assert.Contains(t, out, "SEC-56: DB_HOST=")
				assert.Contains(t, out, "SEC-46: DB_HOST=")
				out, err = makeAlvo(t, copiaDoMakefile(t), "db-guard-integracao", nil, args...)
				require.Error(t, err, out)
				assert.Contains(t, out, "SEC-56: DB_HOST=")
				assert.Contains(t, out, "SEC-54: DB_HOST=")

				// Go: ErrBancoProducao com o mesmo texto; a integração recusa.
				errGo := integracao.ValidarNaoProducao(h, porta)
				require.ErrorIs(t, errGo, integracao.ErrBancoProducao)
				assert.NotErrorIs(t, errGo, integracao.ErrPortaInvalida)
				assert.Contains(t, errGo.Error(), "SEC-56: DB_HOST=")
				assert.Contains(t, errGo.Error(), "IPv4 em forma numerica nao canonica (decimal/hex/octal/curta): use a.b.c.d")
			})
		}
	}
}

// Canônicos e nomes seguem fora da SEC-56 (nenhuma recusa nova).
func TestLote37_SEC56_CanonicosENomesNaoSaoSEC56(t *testing.T) {
	for _, h := range []string{"127.0.0.1", "10.0.0.0", "255.255.255.255", "0.0.0.0", "192.168.168.107", "cafe", "db1.lan", "0xg", "a0x1", "1e3"} {
		t.Run(h, func(t *testing.T) {
			out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-producao", nil, "DB_HOST="+h, "DB_PORT=3307", "DB_NAME=rotaperfumes")
			require.NoError(t, err, out)
			assert.NotContains(t, out, "SEC-56")
			assert.NoError(t, integracao.ValidarNaoProducao(h, "3307"))
		})
	}
}

// SEC-56 (recomendado no spec): loopback textual só com octeto canônico no
// db-guard-drop e no db-check-env.
func TestLote37_SEC56_LoopbackSoCanonico(t *testing.T) {
	casos := []struct {
		host     string
		loopback bool
	}{
		{"127.0.0.1", true},
		{"127.255.255.254", true},
		{"[::1]", true},
		{"localhost.", true},
		{"127.000.000.001", false},
		{"127.0.0.01", false},
		{"127.1", false},
		{"127.0.0.256", false},
		{"0x7f.0.0.1", false},
	}
	for _, c := range casos {
		t.Run(c.host, func(t *testing.T) {
			// db-guard-drop: fora do loopback, DROP do "rotaperfumes" exige CONFIRMAR.
			out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-drop", nil, "DB_HOST="+c.host, "DB_NAME=rotaperfumes")
			if c.loopback {
				require.NoError(t, err, out)
			} else {
				require.Error(t, err, out)
				assert.Contains(t, out, "(fora do loopback): repita com CONFIRMAR=rotaperfumes")
			}
			// db-check-env: sem DB_TLS_CA, só loopback passa (SEC-42).
			dir, path, cliente, _ := bug27Dir(t, false)
			out, err = makeAlvo(t, dir, "db-check-env", []string{path}, "DB_HOST="+c.host, "DB_USUARIO=u", "DB_SENHA=s",
				"DB_TLS_CA=", "DB_CLIENTE_BIN="+cliente)
			if c.loopback {
				require.NoError(t, err, out)
			} else {
				require.Error(t, err, out)
				assert.Contains(t, out, "SEC-42: DB_HOST="+c.host+" nao e loopback")
			}
		})
	}
}

func TestLote37_SEC57_PortasInvalidasEValidas(t *testing.T) {
	invalidas := []string{"+", "++3307", "3307+", "-3307", "0x0ceb", "3307\t", "3 307", "٣٣٠٧"}
	for _, p := range invalidas {
		t.Run("invalida/"+p, func(t *testing.T) {
			for _, h := range []string{"127.0.0.1", "192.168.168.106"} {
				for _, confirma := range []bool{false, true} {
					args := []string{"DB_HOST=" + h, "DB_PORT=" + p, "DB_NAME=rotaperfumes"}
					if confirma {
						args = append(args, "CONFIRMO_PRODUCAO="+h+"/rotaperfumes")
					}
					out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-producao", nil, args...)
					require.Error(t, err, out)
					assert.Contains(t, out, "SEC-57: DB_PORT=")
					assert.NotContains(t, out, "AVISO (DEPLOY-07)")
					assert.NotContains(t, out, "e o banco de PRODUCAO")
				}
				err := integracao.ValidarNaoProducao(h, p)
				require.ErrorIs(t, err, integracao.ErrPortaInvalida, "%q", p)
				assert.NotErrorIs(t, err, integracao.ErrBancoProducao)
				errInt := integracao.ValidarPermitidoIntegracao(h, p)
				assert.ErrorIs(t, errInt, integracao.ErrPortaInvalida)
				assert.ErrorIs(t, errInt, integracao.ErrHostNaoPermitido)
			}
		})
	}
	// Quebra de linha na porta: o shell da guarda nem chega à SEC-57 (aspas
	// do echo quebradas), mas a recusa continua (fail-closed) e o Go recusa.
	t.Run("invalida/quebra de linha", func(t *testing.T) {
		for _, confirma := range []bool{false, true} {
			args := []string{"DB_HOST=127.0.0.1", "DB_PORT=3307\n", "DB_NAME=rotaperfumes"}
			if confirma {
				args = append(args, "CONFIRMO_PRODUCAO=127.0.0.1/rotaperfumes")
			}
			out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-producao", nil, args...)
			require.Error(t, err, out)
			assert.NotContains(t, out, "AVISO (DEPLOY-07)")
		}
		require.ErrorIs(t, integracao.ValidarNaoProducao("127.0.0.1", "3307\n"), integracao.ErrPortaInvalida)
	})
	for _, p := range []string{"3307", "+3307", "03307", "+03307", ""} {
		t.Run("valida/"+p, func(t *testing.T) {
			out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-producao", nil, "DB_HOST=127.0.0.1", "DB_PORT="+p, "DB_NAME=rotaperfumes")
			require.NoError(t, err, out)
			assert.NotContains(t, out, "SEC-57")
			assert.NoError(t, integracao.ValidarNaoProducao("127.0.0.1", p))
			assert.NoError(t, integracao.ValidarPermitidoIntegracao("127.0.0.1", p))
		})
	}
}

// SEC-57 (só documentar no spec): o DB_PORT do .env/ENV_ARQUIVO passa pelo
// $(strip) do Makefile; espaço no fim do valor do arquivo não é recusado e o
// cliente recebe a porta limpa.
func TestLote37_SEC57_PortaDoEnvEStripada(t *testing.T) {
	dir, path, cliente, log := bug27Dir(t, false)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "p.env"),
		[]byte("DB_HOST=127.0.0.1\nDB_PORT=3307   \nDB_NAME=rp_p\nDB_USUARIO=u\nDB_SENHA=s\n"), 0o600))
	out, err := makeAlvo(t, dir, "db-create", []string{path}, "ENV_ARQUIVO=p.env", "DB_TLS_CA=", "DB_CLIENTE_BIN="+cliente)
	require.NoError(t, err, out)
	assert.NotContains(t, out, "SEC-57")
	assert.Contains(t, lerSeExiste(t, log), "-P 3307 ")
}

func TestLote37_SEC57_ConfigDoAmbiente(t *testing.T) {
	for _, p := range []string{"3307 ", " 3307", "  ", "+", "33 07"} {
		t.Run(p, func(t *testing.T) {
			ambienteIntegracao(t, credenciais(map[string]string{"DB_HOST": "127.0.0.1", "DB_PORT": p}))
			cfg, err := integracao.ConfigDoAmbiente()
			require.ErrorIs(t, err, integracao.ErrPortaInvalida)
			assert.Nil(t, cfg)
			assert.NotContains(t, err.Error(), "s-sec50")
			tb := &tbFalso{}
			assert.Nil(t, integracao.Abrir(tb))
			assert.True(t, tb.falhou, "porta inválida falha (t.Fatal), não pula")
			assert.False(t, tb.pulou)
			assert.Contains(t, tb.msg, "SEC-57")
		})
	}
	t.Run("IPv4 não canônico no ambiente", func(t *testing.T) {
		ambienteIntegracao(t, credenciais(map[string]string{"DB_HOST": "2130706433", "DB_PORT": "3306"}))
		_, err := integracao.ConfigDoAmbiente()
		require.True(t, errors.Is(err, integracao.ErrBancoProducao), "%v", err)
		assert.True(t, strings.Contains(err.Error(), "SEC-56"))
	})
}

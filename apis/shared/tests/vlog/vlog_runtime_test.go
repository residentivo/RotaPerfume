package vlog_test

import (
	"bytes"
	"log"
	"regexp"
	"strings"
	"testing"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
	"github.com/rotaperfumes/shared/vlog"
)

// LOG-02: valida em runtime que os logs verbose instrumentados seguem o
// formato "[arquivo.go] [Funcao] descrição" e nunca expõem senha, hash ou
// pepper; e que com a flag desligada nada é emitido.

var linhaVlog = regexp.MustCompile(`^\[(password_generator|password_policy|password_hash)\.go\] \[[A-Za-z][A-Za-z0-9_.]*\] \S.*$`)

const pepperTeste = "pepper-de-teste-com-mais-de-32-bytes-0123456789"

var hashParams = config.HashSenha{Pepper: pepperTeste, MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1}

func capturar(t *testing.T, ligado bool, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags, prevEnabled := log.Writer(), log.Flags(), vlog.Enabled()
	log.SetOutput(&buf)
	log.SetFlags(0)
	vlog.SetEnabled(ligado)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		vlog.SetEnabled(prevEnabled)
	})
	fn()
	return buf.String()
}

type caso struct {
	nome    string
	arquivo string
	// exec roda a função instrumentada e devolve os valores sensíveis que
	// NÃO podem aparecer no log.
	exec func(t *testing.T) []string
}

func casos() []caso {
	return []caso{
		{"GerarSenhaAleatoria", "password_generator.go", func(t *testing.T) []string {
			s, err := services.GerarSenhaAleatoria(16)
			if err != nil {
				t.Fatalf("GerarSenhaAleatoria: %v", err)
			}
			return []string{s}
		}},
		{"SenhaAlfanumerica", "password_generator.go", func(t *testing.T) []string {
			s, err := services.SenhaAlfanumerica(20)
			if err != nil {
				t.Fatalf("SenhaAlfanumerica: %v", err)
			}
			return []string{s}
		}},
		{"ValidarForcaSenha forte", "password_policy.go", func(t *testing.T) []string {
			senha := "Zq9!Segredo#Forte77"
			if err := services.ValidarForcaSenha(senha); err != nil {
				t.Fatalf("ValidarForcaSenha: %v", err)
			}
			return []string{senha}
		}},
		{"ValidarForcaSenha fraca", "password_policy.go", func(t *testing.T) []string {
			senha := "abcfraca"
			_ = services.ValidarForcaSenha(senha)
			return []string{senha}
		}},
		{"GerarHashSenha + VerificarSenha", "password_hash.go", func(t *testing.T) []string {
			senha := "Kx7$SenhaParaHash"
			h, err := services.GerarHashSenha(hashParams, senha)
			if err != nil {
				t.Fatalf("GerarHashSenha: %v", err)
			}
			if !services.VerificarSenha(hashParams, h, senha) {
				t.Fatal("VerificarSenha deveria aceitar a senha")
			}
			_ = services.PrecisaRehash(hashParams, h)
			partes := strings.Split(h, "$")
			return []string{senha, h, pepperTeste, partes[len(partes)-1], partes[len(partes)-2]}
		}},
	}
}

func TestVlogRuntimeLigadoFormatoESemVazamento(t *testing.T) {
	for _, c := range casos() {
		t.Run(c.nome, func(t *testing.T) {
			var sensiveis []string
			out := capturar(t, true, func() { sensiveis = c.exec(t) })
			if strings.TrimSpace(out) == "" {
				t.Fatal("esperava logs verbose com a flag ligada")
			}
			viuArquivo := false
			for _, linha := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
				if !linhaVlog.MatchString(linha) {
					t.Errorf("linha fora do formato [arquivo] [funcao] msg: %q", linha)
				}
				if strings.HasPrefix(linha, "["+c.arquivo+"]") {
					viuArquivo = true
				}
			}
			if !viuArquivo {
				t.Errorf("nenhuma linha com [%s]", c.arquivo)
			}
			for _, s := range sensiveis {
				if s != "" && strings.Contains(out, s) {
					t.Errorf("valor sensível vazou no log verbose (len=%d)", len(s))
				}
			}
		})
	}
}

func TestVlogRuntimeDesligadoNaoEmite(t *testing.T) {
	for _, c := range casos() {
		t.Run(c.nome, func(t *testing.T) {
			out := capturar(t, false, func() { _ = c.exec(t) })
			if out != "" {
				t.Fatalf("com a flag desligada nada deveria ser emitido; veio %d bytes", len(out))
			}
		})
	}
}

func TestVlogMaskEmailNaoExpoeUsuario(t *testing.T) {
	casosEmail := []struct{ in, want string }{
		{"ana.silva@empresa.com", "a***@empresa.com"},
		{"x@y.com", "x***@y.com"},
		{"@semusuario.com", "***"},
		{"sem-arroba", "***"},
		{"", "***"},
	}
	for _, c := range casosEmail {
		t.Run(c.in, func(t *testing.T) {
			if got := vlog.MaskEmail(c.in); got != c.want {
				t.Fatalf("MaskEmail(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

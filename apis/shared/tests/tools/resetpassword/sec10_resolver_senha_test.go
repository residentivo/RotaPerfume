package resetpassword_test

// SEC-10: fontes de senha do cmd/resetpassword (ResolverSenha) e DSN sem
// fallback de credenciais. As dependências de terminal (isTerminal,
// readNoEcho) e o stdin são falsos; nada toca o terminal real.

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/tools/resetpassword"
)

var errLeitura = errors.New("falha simulada de leitura")

// leitorProibido falha o teste se o stdin for lido.
type leitorProibido struct{ t *testing.T }

func (l leitorProibido) Read([]byte) (int, error) {
	l.t.Error("stdin não deveria ser lido")
	return 0, io.EOF
}

type leitorComErro struct{}

func (leitorComErro) Read([]byte) (int, error) { return 0, errLeitura }

// terminalFalso devolve as leituras sem eco em sequência e guarda os buffers
// entregues (para conferir que são zerados depois).
type terminalFalso struct {
	ehTerminal bool
	leituras   [][]byte
	erroEm     int // índice (1-based) da leitura que falha; 0 = nenhuma
	chamadas   int
	entregues  [][]byte
}

func (f *terminalFalso) isTerminal() bool { return f.ehTerminal }

func (f *terminalFalso) readNoEcho() ([]byte, error) {
	f.chamadas++
	if f.chamadas == f.erroEm {
		return nil, errLeitura
	}
	if f.chamadas > len(f.leituras) {
		return nil, io.EOF
	}
	b := append([]byte(nil), f.leituras[f.chamadas-1]...)
	f.entregues = append(f.entregues, b)
	return b, nil
}

func proibidoTerminal(t *testing.T) (func() bool, func() ([]byte, error)) {
	return func() bool { t.Error("isTerminal não deveria ser chamado"); return false },
		func() ([]byte, error) { t.Error("readNoEcho não deveria ser chamado"); return nil, nil }
}

func TestSEC10_ResolverSenha_Stdin(t *testing.T) {
	casos := []struct {
		nome    string
		stdin   io.Reader
		want    string
		wantErr string
	}{
		{"CRLF removido", strings.NewReader("abc\r\n"), "abc", ""},
		{"LF removido", strings.NewReader("abc\n"), "abc", ""},
		{"sem quebra final (EOF)", strings.NewReader("abc"), "abc", ""},
		{"só a 1ª linha", strings.NewReader("primeira\r\nsegunda\n"), "primeira", ""},
		{"espaços internos/nas pontas preservados", strings.NewReader(" a b \n"), " a b ", ""},
		{"stdin vazio", strings.NewReader(""), "", "senha vazia"},
		{"1ª linha vazia (LF)", strings.NewReader("\nsenha\n"), "", "senha vazia"},
		{"1ª linha vazia (CRLF)", strings.NewReader("\r\n"), "", "senha vazia"},
		{"erro de leitura", leitorComErro{}, "", "leitura"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			isT, read := proibidoTerminal(t)
			var stderr bytes.Buffer
			got, err := resetpassword.ResolverSenha(resetpassword.Options{PasswordStdin: true}, c.stdin, isT, read, &stderr)
			if c.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErr)
				assert.Contains(t, err.Error(), "-password-stdin")
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
			assert.Empty(t, stderr.String(), "stdin não escreve nada no stderr")
		})
	}
	t.Run("erro de leitura embrulhado", func(t *testing.T) {
		isT, read := proibidoTerminal(t)
		_, err := resetpassword.ResolverSenha(resetpassword.Options{PasswordStdin: true}, leitorComErro{}, isT, read, io.Discard)
		assert.ErrorIs(t, err, errLeitura)
	})
}

func TestSEC10_ResolverSenha_Prompt(t *testing.T) {
	casos := []struct {
		nome       string
		term       *terminalFalso
		want       string
		wantErr    string
		wantErrIs  error
		wantStderr []string
	}{
		{
			nome: "confirmação igual",
			term: &terminalFalso{ehTerminal: true, leituras: [][]byte{[]byte("S3nh@Forte!"), []byte("S3nh@Forte!")}},
			want: "S3nh@Forte!", wantStderr: []string{"Nova senha: ", "Confirme a senha: "},
		},
		{
			nome:    "confirmação diferente",
			term:    &terminalFalso{ehTerminal: true, leituras: [][]byte{[]byte("S3nh@Forte!"), []byte("S3nh@Forte?")}},
			wantErr: "não conferem",
		},
		{
			nome:    "senha vazia",
			term:    &terminalFalso{ehTerminal: true, leituras: [][]byte{{}, {}}},
			wantErr: "senha vazia",
		},
		{
			nome:    "sem terminal: erro com dica",
			term:    &terminalFalso{ehTerminal: false},
			wantErr: "-password-stdin",
		},
		{
			nome:      "erro na 1ª leitura",
			term:      &terminalFalso{ehTerminal: true, erroEm: 1},
			wantErr:   "leitura",
			wantErrIs: errLeitura,
		},
		{
			nome:      "erro na confirmação",
			term:      &terminalFalso{ehTerminal: true, leituras: [][]byte{[]byte("abc")}, erroEm: 2},
			wantErr:   "leitura",
			wantErrIs: errLeitura,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var stderr bytes.Buffer
			got, err := resetpassword.ResolverSenha(resetpassword.Options{PasswordPrompt: true},
				leitorProibido{t}, c.term.isTerminal, c.term.readNoEcho, &stderr)
			if c.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErr)
				if c.wantErrIs != nil {
					assert.ErrorIs(t, err, c.wantErrIs)
				}
				assert.Empty(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, c.want, got)
			}
			for _, s := range c.wantStderr {
				assert.Contains(t, stderr.String(), s)
			}
			assert.NotContains(t, stderr.String(), "S3nh@Forte", "a senha nunca é ecoada")
			// Buffers lidos do terminal são zerados após o uso (inclusive
			// quando a leitura da confirmação falha).
			for _, b := range c.term.entregues {
				assert.True(t, bytes.Equal(make([]byte, len(b)), b), "buffer de senha não foi zerado: %q", b)
			}
		})
	}
	t.Run("sem terminal: dica cita winpty e não lê nada", func(t *testing.T) {
		term := &terminalFalso{ehTerminal: false}
		_, err := resetpassword.ResolverSenha(resetpassword.Options{PasswordPrompt: true}, leitorProibido{t}, term.isTerminal, term.readNoEcho, io.Discard)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "winpty")
		assert.Contains(t, err.Error(), "terminal")
		assert.Zero(t, term.chamadas)
	})
}

func TestSEC10_ResolverSenha_PasswordDepreciada(t *testing.T) {
	isT, read := proibidoTerminal(t)
	var stderr bytes.Buffer
	got, err := resetpassword.ResolverSenha(resetpassword.Options{Password: "Direta@123"}, leitorProibido{t}, isT, read, &stderr)
	require.NoError(t, err)
	assert.Equal(t, "Direta@123", got)
	assert.Equal(t, resetpassword.AvisoPasswordDepreciada+"\n", stderr.String(), "aviso exato no stderr")
	assert.NotContains(t, stderr.String(), "Direta@123")
	assert.Contains(t, resetpassword.AvisoPasswordDepreciada, "-password-prompt")
	assert.Contains(t, resetpassword.AvisoPasswordDepreciada, "-password-stdin")
	assert.Contains(t, resetpassword.AvisoPasswordDepreciada, "depreciada")
}

func TestSEC10_ResolverSenha_Fontes(t *testing.T) {
	casos := []struct {
		nome string
		opts resetpassword.Options
	}{
		{"-password + -password-stdin", resetpassword.Options{Password: "x", PasswordStdin: true}},
		{"-password + -password-prompt", resetpassword.Options{Password: "x", PasswordPrompt: true}},
		{"-password-stdin + -password-prompt", resetpassword.Options{PasswordStdin: true, PasswordPrompt: true}},
		{"as três", resetpassword.Options{Password: "x", PasswordStdin: true, PasswordPrompt: true}},
	}
	for _, c := range casos {
		t.Run("duas ou mais fontes: "+c.nome, func(t *testing.T) {
			isT, read := proibidoTerminal(t)
			var stderr bytes.Buffer
			got, err := resetpassword.ResolverSenha(c.opts, leitorProibido{t}, isT, read, &stderr)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "no máximo uma fonte")
			assert.Empty(t, got)
			assert.Empty(t, stderr.String(), "nem o aviso de depreciação é escrito")
		})
	}
	t.Run("nenhuma fonte: fluxo antigo (vazio, sem ler nada)", func(t *testing.T) {
		isT, read := proibidoTerminal(t)
		var stderr bytes.Buffer
		got, err := resetpassword.ResolverSenha(resetpassword.Options{Email: "a@b.com", CreateAdmin: true}, leitorProibido{t}, isT, read, &stderr)
		require.NoError(t, err)
		assert.Empty(t, got)
		assert.Empty(t, stderr.String())
	})
}

// unsetEnv remove a variável (ausente, não vazia) e a restaura no Cleanup.
func unsetEnv(t *testing.T, k string) {
	t.Helper()
	t.Setenv(k, "") // registra a restauração do valor original
	require.NoError(t, os.Unsetenv(k))
}

func TestSEC10_DSN(t *testing.T) {
	const sufixo = "?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local&clientFoundRows=true"
	casos := []struct {
		nome    string
		env     map[string]string // valor "<unset>" = variável ausente
		want    string
		wantErr bool
	}{
		{"nenhuma variável", map[string]string{"DB_USUARIO": "<unset>", "DB_SENHA": "<unset>"}, "", true},
		{"só DB_USUARIO", map[string]string{"DB_USUARIO": "u", "DB_SENHA": "<unset>"}, "", true},
		{"só DB_SENHA", map[string]string{"DB_USUARIO": "<unset>", "DB_SENHA": "s"}, "", true},
		{"DB_USUARIO vazio", map[string]string{"DB_USUARIO": "", "DB_SENHA": "s"}, "", true},
		{"DB_SENHA vazia", map[string]string{"DB_USUARIO": "u", "DB_SENHA": ""}, "", true},
		{"golang/golang não é mais default", map[string]string{"DB_USUARIO": "<unset>", "DB_SENHA": "<unset>", "DB_HOST": "h"}, "", true},
		{"com credenciais e defaults", map[string]string{"DB_USUARIO": "u", "DB_SENHA": "s"}, "u:s@tcp(localhost:3306)/rotaperfumes" + sufixo, false},
		{"com tudo", map[string]string{"DB_USUARIO": "app", "DB_SENHA": "p@ss", "DB_HOST": "db", "DB_PORT": "3307", "DB_NAME": "rp"}, "app:p@ss@tcp(db:3307)/rp" + sufixo, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			for _, k := range []string{"DB_USUARIO", "DB_SENHA", "DB_HOST", "DB_PORT", "DB_NAME"} {
				unsetEnv(t, k)
			}
			for k, v := range c.env {
				if v != "<unset>" {
					t.Setenv(k, v)
				}
			}
			dsn, err := resetpassword.DSN()
			if c.wantErr {
				assert.ErrorIs(t, err, resetpassword.ErrCredenciaisDB)
				assert.Empty(t, dsn)
				assert.NotContains(t, dsn, "golang")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, dsn)
		})
	}
}

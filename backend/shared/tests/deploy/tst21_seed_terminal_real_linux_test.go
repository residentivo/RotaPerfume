//go:build linux

package deploy_test

// TST-21 (Lote 35): seed-producao.sh com terminal REAL (pty do
// `script -qec` do util-linux) por cima do ensaio do DEPLOY-07 (unshare -rm,
// docker falso). Cobre o que o BUG-26 (sem terminal) não cobre:
//   - pepper digitado sem eco: o valor nunca aparece na saída do terminal
//     nem no argv do docker; chega ao resetpassword só pelo --env-file;
//   - dois peppers diferentes: o script morre antes de qualquer SQL;
//   - --forcar com o nome do banco errado: nada alterado; com o nome certo:
//     prossegue até a conferência;
//   - --admin-senha-prompt: docker run -it (seed-producao.sh, etapa do
//     admin) com o pepper no --env-file, nunca no argv.
// Cada escrita no stdin espera o prompt aparecer (o read -s só desliga o eco
// quando começa; escrever antes ecoaria o pepper). Pula sem `script` do
// util-linux ou sem `unshare`.

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubDockerPromptTST21: embrulha o docker falso do DEPLOY-07; só o
// "run -it ... -create-admin -password-prompt" é tratado aqui (grava argv,
// modo e conteúdo do --env-file); o resto vai para o stub original.
const stubDockerPromptTST21 = `#!/bin/bash
set -u
if [[ " $* " == *" -password-prompt "* ]]; then
  printf '%s\n' "$@" >"$FAKE/state/prompt_argv"
  [[ "${1:-}" == run && "${2:-}" == -it ]] && : >"$FAKE/state/prompt_run_it"
  args=("$@")
  for ((i=0; i<${#args[@]}; i++)); do
    if [[ "${args[i]}" == --env-file ]]; then
      stat -c '%a' "${args[i+1]}" >"$FAKE/state/prompt_env_modo"
      cp "${args[i+1]}" "$FAKE/state/prompt_env_conteudo"
    fi
  done
  [[ -t 0 ]] && : >"$FAKE/state/prompt_stdin_tty"
  exit 0
fi
exec "$FAKE/bin/docker-base" "$@"
`

const (
	pepperTST21  = "pepper-digitado-no-terminal-com-32-bytes-ou-mais"
	pepper2TST21 = "outro-pepper-digitado-no-terminal-com-32-bytes"
)

func precisaScript(t *testing.T) {
	t.Helper()
	caminho, err := exec.LookPath("script")
	if err != nil {
		t.Skip("script (util-linux) não está no PATH")
	}
	out, err := exec.Command(caminho, "--version").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "util-linux") {
		t.Skipf("script não é o do util-linux: %v %s", err, out)
	}
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("unshare não está no PATH")
	}
}

// novoEnsaioTTY: o ensaio do DEPLOY-07 com o docker embrulhado.
func novoEnsaioTTY(t *testing.T) *ensaioSeed {
	t.Helper()
	precisaScript(t)
	e := novoEnsaioSeed(t)
	bin := filepath.Join(e.dir, "bin")
	require.NoError(t, os.Rename(filepath.Join(bin, "docker"), filepath.Join(bin, "docker-base")))
	escrever(t, filepath.Join(bin, "docker"), stubDockerPromptTST21, 0o755)
	return e
}

// sessaoTTY: o script rodando sob `script -qec`, com a saída do pty
// acumulada e o stdin aberto até o fim.
type sessaoTTY struct {
	t     *testing.T
	cmd   *exec.Cmd
	stdin io.WriteCloser
	mu    sync.Mutex
	buf   bytes.Buffer
	fim   chan error
}

type escritorTTY struct{ s *sessaoTTY }

func (w escritorTTY) Write(p []byte) (int, error) {
	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	return w.s.buf.Write(p)
}

func aspasShell(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func (e *ensaioSeed) rodarComTerminal(args ...string) *sessaoTTY {
	e.t.Helper()
	interno := e.comando(args...)
	partes := make([]string, 0, len(interno.Args))
	for _, a := range interno.Args {
		partes = append(partes, aspasShell(a))
	}
	s := &sessaoTTY{t: e.t, fim: make(chan error, 1)}
	s.cmd = exec.Command("script", "-qec", strings.Join(partes, " "), "/dev/null")
	s.cmd.Env = append(interno.Env, "SHELL=/bin/bash", "TERM=dumb")
	s.cmd.Stdout = escritorTTY{s}
	s.cmd.Stderr = escritorTTY{s}
	var err error
	s.stdin, err = s.cmd.StdinPipe()
	require.NoError(e.t, err)
	require.NoError(e.t, s.cmd.Start())
	go func() { s.fim <- s.cmd.Wait() }()
	e.t.Cleanup(func() {
		_ = s.stdin.Close()
		if s.cmd.ProcessState == nil {
			_ = s.cmd.Process.Kill()
		}
	})
	return s
}

func (s *sessaoTTY) saida() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// esperar aguarda o trecho aparecer na saída (o prompt) antes de digitar.
func (s *sessaoTTY) esperar(trecho string) {
	s.t.Helper()
	limite := time.Now().Add(30 * time.Second)
	for time.Now().Before(limite) {
		if strings.Contains(s.saida(), trecho) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("prompt %q não apareceu; saída:\n%s", trecho, s.saida())
}

func (s *sessaoTTY) digitar(prompt, texto string) {
	s.t.Helper()
	s.esperar(prompt)
	_, err := io.WriteString(s.stdin, texto+"\n")
	require.NoError(s.t, err)
}

// terminar espera o fim (o stdin continua aberto: EOF no pty viraria ^D).
func (s *sessaoTTY) terminar() (string, error) {
	s.t.Helper()
	select {
	case err := <-s.fim:
		return s.saida(), err
	case <-time.After(90 * time.Second):
		_ = s.cmd.Process.Kill()
		s.t.Fatalf("seed-producao.sh não terminou; saída:\n%s", s.saida())
		return "", nil
	}
}

const (
	promptPepper      = "PASSWORD_PEPPER (o mesmo do rotaperfumes-api-env; nao aparece na tela): "
	promptConfirma    = "Confirme o PASSWORD_PEPPER: "
	promptNomeDoBanco = "Digite o nome do banco (rotaperfumes) para confirmar: "
)

// semSegredos: pepper digitado, pepper do arquivo e senha do banco nunca na
// saída do terminal nem no argv do docker.
func (e *ensaioSeed) semSegredos(t *testing.T, out string) {
	t.Helper()
	log := e.ler("log") + e.ler("state/prompt_argv")
	for nome, v := range map[string]string{"pepper digitado": pepperTST21, "segundo pepper": pepper2TST21,
		"pepper do arquivo": pepperSeed, "senha do banco": senhaDBSeed} {
		assert.NotContains(t, out, v, "%s na saída do terminal", nome)
		assert.NotContains(t, log, v, "%s no argv do docker", nome)
	}
}

func (e *ensaioSeed) semSQLDeEscrita(t *testing.T) {
	t.Helper()
	for _, c := range e.chamadas() {
		if c.mariadb() {
			assert.True(t, strings.HasPrefix(c.stdin, "SELECT "), "só consultas: %q", c.stdin)
		}
		assert.False(t, c.tem("-create-admin"), "sem -create-admin")
		assert.False(t, c.tem("-ec"), "nem compila")
	}
	assert.NoFileExists(t, filepath.Join(e.dir, "root", "rotaperfumes-mariadb", "admin-app.senha"))
}

func TestTST21_SeedProducao_TerminalReal_PepperSemEco(t *testing.T) {
	casos := []struct {
		nome   string
		args   []string
		prompt bool // --admin-senha-prompt: docker run -it
	}{
		{"senha do admin gerada", nil, false},
		{"senha do admin digitada (run -it)", []string{"--admin-senha-prompt"}, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioTTY(t)
			s := e.rodarComTerminal(c.args...)
			s.digitar(promptPepper, pepperTST21)
			s.digitar(promptConfirma, pepperTST21)
			out, err := s.terminar()
			require.NoError(t, err, out)
			assert.Contains(t, out, "=== [7/7] Conferencia ===")
			assert.Contains(t, out, "pepper lido; nada foi impresso")
			e.semSegredos(t, out)

			envConteudo, envModo := e.ler("state/env_conteudo"), e.ler("state/env_modo")
			if c.prompt {
				assert.FileExists(t, filepath.Join(e.dir, "state", "prompt_run_it"), "docker run -it")
				assert.FileExists(t, filepath.Join(e.dir, "state", "prompt_stdin_tty"), "o -it recebe o terminal")
				assert.Contains(t, e.ler("state/prompt_argv"), "--env-file\n")
				envConteudo, envModo = e.ler("state/prompt_env_conteudo"), e.ler("state/prompt_env_modo")
				assert.Contains(t, out, "Senha inicial do admin: a digitada")
			} else {
				assert.NoFileExists(t, filepath.Join(e.dir, "state", "prompt_argv"))
				assert.Equal(t, senhaAdmSeed+"\n", e.ler("root/rotaperfumes-mariadb/admin-app.senha"))
			}
			assert.Contains(t, envConteudo, "PASSWORD_PEPPER="+pepperTST21+"\n", "pepper só pelo --env-file")
			assert.Equal(t, "600", strings.TrimSpace(envModo))
			trab, _ := filepath.Glob(filepath.Join(e.dir, "root", "rotaperfumes-seed.*"))
			assert.Empty(t, trab, "env-file apagado no fim")
		})
	}
}

func TestTST21_SeedProducao_TerminalReal_PeppersDiferentes(t *testing.T) {
	e := novoEnsaioTTY(t)
	s := e.rodarComTerminal()
	s.digitar(promptPepper, pepperTST21)
	s.digitar(promptConfirma, pepper2TST21)
	out, err := s.terminar()
	require.Error(t, err, out)
	assert.Contains(t, out, "os dois PASSWORD_PEPPER digitados nao conferem")
	assert.NotContains(t, out, "=== [2/7]", "morre nos pré-requisitos")
	for _, c := range e.chamadas() {
		assert.False(t, c.mariadb(), "nenhum cliente mariadb")
	}
	e.semSQLDeEscrita(t)
	e.semSegredos(t, out)
}

func TestTST21_SeedProducao_TerminalReal_Forcar(t *testing.T) {
	casos := []struct {
		nome, resposta string
		prossegue      bool
	}{
		{"nome errado", "rotaperfume", false},
		{"nome em maiúsculas", "ROTAPERFUMES", false},
		{"vazio", "", false},
		{"nome certo", "rotaperfumes", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioTTY(t)
			e.state("linhas", "5\n")
			s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar")
			s.digitar(promptNomeDoBanco, c.resposta)
			out, err := s.terminar()
			assert.Contains(t, out, "--forcar: usuarios tem 5 linha(s)")
			e.semSegredos(t, out)
			if !c.prossegue {
				require.Error(t, err, out)
				assert.Contains(t, out, "confirmacao nao confere; nada foi alterado")
				assert.NotContains(t, out, "=== [3/7]")
				e.semSQLDeEscrita(t)
				return
			}
			require.NoError(t, err, out)
			assert.Contains(t, out, "=== [7/7] Conferencia ===")
			var update, admin bool
			for _, ch := range e.chamadas() {
				update = update || (ch.mariadb() && strings.HasPrefix(ch.stdin, "UPDATE usuarios SET ativo = 0"))
				admin = admin || ch.tem("-create-admin")
			}
			assert.True(t, update, "reaplica e inativa os não-admin")
			assert.True(t, admin, "troca a senha do admin")
			assert.Contains(t, e.ler("state/env_conteudo"), "PASSWORD_PEPPER="+pepperSeed+"\n")
		})
	}
}

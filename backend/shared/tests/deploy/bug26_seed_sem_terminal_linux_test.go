//go:build linux

package deploy_test

// BUG-26 (Lote 34): seed-producao.sh sem terminal de controle. O go test
// herda o terminal de quem o chamou; Setsid tira o terminal de controle do
// script (o open de /dev/tty falha com ENXIO, embora [[ -r /dev/tty ]] passe).
// O script tem de falhar com a mensagem clara, antes do Docker e do banco,
// nunca com o erro cru do bash no read < /dev/tty.

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rodarSemTerminal roda o ensaio numa sessão nova, sem terminal de controle.
func (e *ensaioSeed) rodarSemTerminal(args ...string) (string, error) {
	e.t.Helper()
	cmd := e.comando(args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// semEfeito confere que nada foi alterado: nenhuma chamada ao docker (logo,
// nenhuma DDL, seed, UPDATE, compilação ou -create-admin) e nenhum segredo
// na saída.
func (e *ensaioSeed) semEfeito(t *testing.T, out string) {
	t.Helper()
	assert.Empty(t, e.chamadas(), "falha antes de qualquer chamada ao docker")
	assert.NotContains(t, out, "=== [2/7]", "nem consulta o banco")
	assert.NotContains(t, out, "No such device", "sem o erro cru do bash")
	assert.NotContains(t, out, "/dev/tty:", "sem o erro cru do redirecionamento")
	assert.NoFileExists(t, filepath.Join(e.dir, "root", "rotaperfumes-mariadb", "admin-app.senha"))
	assert.NotContains(t, out, senhaDBSeed)
	assert.NotContains(t, out, pepperSeed)
}

func TestBUG26_SeedProducao_ForcarSemTerminal(t *testing.T) {
	e := novoEnsaioSeed(t)
	e.state("linhas", "5\n")
	out, err := e.rodarSemTerminal("--pepper-file", "/root/pepper.txt", "--forcar")
	require.Error(t, err, out)
	assert.Contains(t, out, "ERRO: --forcar exige um terminal para a confirmacao")
	assert.NotContains(t, out, "Digite o nome do banco", "não chega a pedir a confirmação")
	e.semEfeito(t, out)
}

// --forcar sem terminal falha cedo mesmo com o banco vazio (a checagem vem
// na etapa 1, antes de consultar o banco).
func TestBUG26_SeedProducao_ForcarSemTerminalBancoVazio(t *testing.T) {
	e := novoEnsaioSeed(t)
	out, err := e.rodarSemTerminal("--pepper-file", "/root/pepper.txt", "--forcar")
	require.Error(t, err, out)
	assert.Contains(t, out, "ERRO: --forcar exige um terminal para a confirmacao")
	e.semEfeito(t, out)
}

func TestBUG26_SeedProducao_PepperSemArquivoSemTerminal(t *testing.T) {
	e := novoEnsaioSeed(t)
	out, err := e.rodarSemTerminal()
	require.Error(t, err, out)
	assert.Contains(t, out, "ERRO: sem terminal para pedir o PASSWORD_PEPPER: use --pepper-file ARQ")
	assert.NotContains(t, out, "PASSWORD_PEPPER (o mesmo", "não chega a pedir o pepper")
	e.semEfeito(t, out)
}

// Controle: sem terminal, mas sem precisar dele (--pepper-file, banco vazio,
// sem --forcar), o seed segue até o fim.
func TestBUG26_SeedProducao_SemTerminalSemPrecisarSegue(t *testing.T) {
	e := novoEnsaioSeed(t)
	out, err := e.rodarSemTerminal("--pepper-file", "/root/pepper.txt")
	require.NoError(t, err, out)
	assert.Contains(t, out, "=== [7/7]")
	assert.NotContains(t, out, "exige um terminal")
}

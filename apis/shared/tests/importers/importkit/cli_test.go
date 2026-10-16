package importkit_test

// ARQ-07: o main de cada cmd/import* virou só a composition root; o CLI
// (flags → .env → config → pipeline) mora no pacote do importador e roda
// aqui com o Ambiente injetado.

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/importers/carteiras"
	"github.com/rotaperfumes/shared/importers/clientes"
	"github.com/rotaperfumes/shared/importers/estoque"
	"github.com/rotaperfumes/shared/importers/oportunidades"
	"github.com/rotaperfumes/shared/importers/pagamentos"
	"github.com/rotaperfumes/shared/importers/pedidos"
	"github.com/rotaperfumes/shared/importers/produtos"
	"github.com/rotaperfumes/shared/importers/visitas"
)

// ambienteCLI devolve um Ambiente sem efeitos de processo; abertos conta as
// conexões pedidas (o dry-run não pode abrir nenhuma).
func ambienteCLI(cfgErr error, abertos *int) cmdutil.Ambiente {
	return cmdutil.Ambiente{
		Saida: io.Discard, Erro: io.Discard,
		CarregarEnv: func() {},
		Config:      func() (*config.Config, error) { return &config.Config{}, cfgErr },
		Abrir: func(string) cmdutil.Opener {
			return func() (cmdutil.Conn, error) { *abertos++; return nil, errTeste }
		},
	}
}

func csvSoCabecalho(t *testing.T, nome string, campos int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), nome)
	require.NoError(t, os.WriteFile(p, []byte(strings.Repeat("c,", campos-1)+"c\n"), 0o600))
	return p
}

func TestARQ07_CLIDosImportadores(t *testing.T) {
	type cli struct {
		tag   string
		cmd   cmdutil.Comando
		flags func(t *testing.T) []string
	}
	umCSV := func(campos int) func(t *testing.T) []string {
		return func(t *testing.T) []string { return []string{"-csv", csvSoCabecalho(t, "x.csv", campos)} }
	}
	clis := []cli{
		{carteiras.Tag, carteiras.CLI, umCSV(5)},
		{clientes.Tag, clientes.CLI, umCSV(9)},
		{estoque.Tag, estoque.CLI, umCSV(4)},
		{oportunidades.Tag, oportunidades.CLI, umCSV(11)},
		{produtos.Tag, produtos.CLI, umCSV(10)},
		{visitas.Tag, visitas.CLI, umCSV(6)},
		{pagamentos.Tag, pagamentos.CLI, func(t *testing.T) []string {
			return []string{"-pagamentos-csv", csvSoCabecalho(t, "pagamentos.csv", 10)}
		}},
		{pedidos.Tag, pedidos.CLI, func(t *testing.T) []string {
			return []string{"-pedidos-csv", csvSoCabecalho(t, "pedidos.csv", 7), "-itens-csv", csvSoCabecalho(t, "itens.csv", 7)}
		}},
	}
	for _, c := range clis {
		t.Run(c.tag, func(t *testing.T) {
			logs := capturarLog(t)
			abertos := 0
			args := c.flags(t)

			assert.Equal(t, cmdutil.SaidaOK, cmdutil.Executar(c.tag, c.cmd, append(args, "-dry-run"), ambienteCLI(nil, &abertos)))
			assert.Contains(t, logs.String(), c.tag+": --dry-run informado, nada foi gravado no banco")
			assert.Zero(t, abertos, "dry-run não abre o banco")

			assert.Equal(t, cmdutil.SaidaErro, cmdutil.Executar(c.tag, c.cmd, args, ambienteCLI(nil, &abertos)))
			assert.Equal(t, 1, abertos, "carga real abre a conexão MySQL do Ambiente")
			assert.Contains(t, logs.String(), c.tag+": falhou")

			assert.Equal(t, cmdutil.SaidaErro, cmdutil.Executar(c.tag, c.cmd, args, ambienteCLI(errors.New("DB_SENHA ausente"), &abertos)))
			assert.Contains(t, logs.String(), c.tag+": falha ao carregar config: DB_SENHA ausente")

			assert.Equal(t, cmdutil.SaidaUso, cmdutil.Executar(c.tag, c.cmd, []string{"-nao-existe"}, ambienteCLI(nil, &abertos)))
		})
	}
}

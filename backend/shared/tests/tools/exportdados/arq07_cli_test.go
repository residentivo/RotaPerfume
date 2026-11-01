package exportdados_test

// ARQ-07: o cmd/exportdados virou main fino; o CLI roda aqui com o Ambiente
// injetado.

import (
	"bytes"
	"errors"
	"io"
	"log"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tools/exportdados"
)

func TestARQ07_CLIExportdados(t *testing.T) {
	var logs bytes.Buffer
	anterior := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(anterior) })

	abertos := 0
	amb := func(cfgErr error) cmdutil.Ambiente {
		return cmdutil.Ambiente{
			Saida: io.Discard, Erro: io.Discard, CarregarEnv: func() {},
			Config: func() (*config.Config, error) { return &config.Config{}, cfgErr },
			Abrir: func(string) cmdutil.Opener {
				return func() (cmdutil.Conn, error) { abertos++; return nil, errors.New("sem banco") }
			},
		}
	}
	out := filepath.Join(t.TempDir(), "export")
	assert.Equal(t, cmdutil.SaidaErro, cmdutil.Executar(exportdados.Tag, exportdados.CLI, []string{"-out", out}, amb(nil)))
	assert.Equal(t, 1, abertos)
	assert.Contains(t, logs.String(), "exportdados: exportando para "+out)
	assert.Contains(t, logs.String(), "exportdados: sem banco")

	assert.Equal(t, cmdutil.SaidaErro, cmdutil.Executar(exportdados.Tag, exportdados.CLI, nil, amb(errors.New("DB_SENHA ausente"))))
	assert.Contains(t, logs.String(), "exportdados: falha ao carregar config: DB_SENHA ausente")
	assert.Equal(t, cmdutil.SaidaUso, cmdutil.Executar(exportdados.Tag, exportdados.CLI, []string{"-x"}, amb(nil)))
}

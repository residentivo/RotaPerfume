package cmdutil_test

// ARQ-07: kit dos CLIs (main fino). Executar traduz o resultado do comando
// em código de saída; Configurar carrega .env, config e vlog.

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/vlog"
)

func logsDoTeste(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	anterior, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(anterior); log.SetFlags(flags) })
	return &buf
}

func TestARQ07_Executar_CodigosDeSaida(t *testing.T) {
	logs := logsDoTeste(t)
	amb := cmdutil.Ambiente{Erro: io.Discard}
	comando := func(err error) cmdutil.Comando {
		return func([]string, cmdutil.Ambiente) error { return err }
	}
	assert.Equal(t, cmdutil.SaidaOK, cmdutil.Executar("x", comando(nil), nil, amb))
	assert.Equal(t, cmdutil.SaidaOK, cmdutil.Executar("x", comando(flag.ErrHelp), nil, amb), "-h não é erro")
	assert.Equal(t, cmdutil.SaidaUso, cmdutil.Executar("x", comando(&cmdutil.ErroDeUso{Err: errors.New("flag ruim")}), nil, amb))
	assert.Empty(t, logs.String(), "ok, -h e uso não logam")

	assert.Equal(t, cmdutil.SaidaErro, cmdutil.Executar("importx", comando(errors.New("ping no banco falhou")), nil, amb))
	assert.Equal(t, "importx: ping no banco falhou\n", logs.String(), "o mesmo texto do log.Fatalf antigo")

	logs.Reset()
	assert.Equal(t, cmdutil.SaidaErro, cmdutil.Executar("", comando(errors.New("resetpassword: ping: x")), nil, amb))
	assert.Equal(t, "resetpassword: ping: x\n", logs.String(), "tag vazia: o erro já traz o prefixo")

	logs.Reset()
	assert.Equal(t, cmdutil.SaidaErro, cmdutil.Executar("x", comando(cmdutil.ErrJaRelatado), nil, amb))
	assert.Empty(t, logs.String(), "falha já relatada não é logada de novo")
}

func TestARQ07_LerFlags(t *testing.T) {
	var uso bytes.Buffer
	amb := cmdutil.Ambiente{Erro: &uso}
	fs := cmdutil.NovasFlags("cmd", amb)
	v := fs.Bool("v", false, "verbose")
	require.NoError(t, cmdutil.LerFlags(fs, []string{"-v"}))
	assert.True(t, *v)

	err := cmdutil.LerFlags(cmdutil.NovasFlags("cmd", amb), []string{"-nao-existe"})
	var erroDeUso *cmdutil.ErroDeUso
	require.ErrorAs(t, err, &erroDeUso)
	assert.Contains(t, erroDeUso.Error(), "nao-existe")
	assert.Contains(t, uso.String(), "Usage of cmd", "o uso vai para amb.Erro")
	assert.ErrorIs(t, cmdutil.LerFlags(cmdutil.NovasFlags("cmd", amb), []string{"-h"}), flag.ErrHelp)
}

func TestARQ07_Configurar(t *testing.T) {
	t.Cleanup(func() { vlog.SetEnabled(false) })
	chamadas := []string{}
	amb := cmdutil.Ambiente{
		CarregarEnv: func() { chamadas = append(chamadas, "env") },
		Config: func() (*config.Config, error) {
			chamadas = append(chamadas, "load")
			return &config.Config{Verbose: true}, nil
		},
		ConfigSemDB: func() (*config.Config, error) { chamadas = append(chamadas, "semdb"); return &config.Config{}, nil },
	}
	cfg, err := cmdutil.Configurar(amb, false)
	require.NoError(t, err)
	assert.True(t, cfg.Verbose)
	assert.True(t, vlog.Enabled(), "VERBOSE liga o vlog")
	_, err = cmdutil.Configurar(amb, true)
	require.NoError(t, err)
	assert.False(t, vlog.Enabled())
	assert.Equal(t, []string{"env", "load", "env", "semdb"}, chamadas, "sem credenciais do banco só quando pedido (M-03)")

	amb.Config = func() (*config.Config, error) { return nil, errors.New("DB_SENHA ausente") }
	_, err = cmdutil.Configurar(amb, false)
	assert.EqualError(t, err, "falha ao carregar config: DB_SENHA ausente")
}

func TestARQ07_AmbientePadrao(t *testing.T) {
	a := cmdutil.AmbientePadrao()
	for nome, ok := range map[string]bool{
		"Entrada": a.Entrada != nil, "Saida": a.Saida != nil, "Erro": a.Erro != nil, "CarregarEnv": a.CarregarEnv != nil,
		"Config": a.Config != nil, "ConfigSemDB": a.ConfigSemDB != nil, "Abrir": a.Abrir != nil,
	} {
		assert.True(t, ok, nome)
	}
	assert.True(t, strings.HasPrefix(cmdutil.ErrJaRelatado.Error(), "falha"))
}

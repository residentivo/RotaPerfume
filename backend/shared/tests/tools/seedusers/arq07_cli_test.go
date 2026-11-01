package seedusers_test

// ARQ-07: o cmd/seedusers virou main fino. -dry-run/-no-exec carregam a
// config SEM as credenciais do banco (M-03); a execução real usa o Load
// completo. NB-7: nem o log nem o dry-run (sem -show-password) levam hash.

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tools/seedusers"
)

func TestARQ07_CLISeedusers(t *testing.T) {
	logs := silenciarLog(t)
	t.Setenv("SEED_ADMIN_PASSWORD", "Admin@Env1")
	t.Setenv("SEED_USER_PASSWORD", "User@Env1")
	var carregou []string
	var saida bytes.Buffer
	amb := cmdutil.Ambiente{
		Saida: &saida, Erro: io.Discard, CarregarEnv: func() {},
		Config: func() (*config.Config, error) {
			carregou = append(carregou, "load")
			return nil, errors.New("defina DB_USUARIO/DB_SENHA")
		},
		ConfigSemDB: func() (*config.Config, error) { carregou = append(carregou, "semdb"); return cfgRapida(), nil },
	}

	require.Equal(t, cmdutil.SaidaOK, cmdutil.Executar(seedusers.Tag, seedusers.CLI, []string{"-dry-run"}, amb))
	assert.Equal(t, []string{"semdb"}, carregou, "dry-run não exige credenciais do banco (M-03)")
	assert.Contains(t, saida.String(), "=== DRY-RUN ===")
	assert.Empty(t, reHash.FindAllString(saida.String()+logs.String(), -1), "NB-7: nenhum hash completo")
	assert.Contains(t, logs.String(), "seedusers: argon2id m=64 t=1 p=1, hashes gerados (admin e user)")
	assert.NotContains(t, logs.String(), "$argon2id$", "NB-7: nem o prefixo do hash vai para o log")
	assert.NotContains(t, saida.String(), "Admin@Env1", "senha só com -show-password")

	saida.Reset()
	require.Equal(t, cmdutil.SaidaOK, cmdutil.Executar(seedusers.Tag, seedusers.CLI, []string{"-dry-run", "-show-password"}, amb))
	assert.Len(t, reHash.FindAllString(saida.String(), -1), 2)
	assert.Contains(t, saida.String(), "ADMIN_PASSWORD=Admin@Env1")

	carregou = nil
	assert.Equal(t, cmdutil.SaidaErro, cmdutil.Executar(seedusers.Tag, seedusers.CLI, nil, amb))
	assert.Equal(t, []string{"load"}, carregou, "a execução real usa o Load completo (SEC-11)")
	assert.Contains(t, logs.String(), "seedusers: falha ao carregar config: defina DB_USUARIO/DB_SENHA")
	assert.Equal(t, cmdutil.SaidaUso, cmdutil.Executar(seedusers.Tag, seedusers.CLI, []string{"-x"}, amb))
}

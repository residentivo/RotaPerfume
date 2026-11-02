package limpezaimagens_test

// CHORE-09 (Lote 31), TestBrain: bordas das idades mínimas, nomes quase
// válidos, erros do banco no Run e o CLI (flags, config, dry-run, falha ao
// apagar) com ambiente injetado.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tools/limpezaimagens"
)

func TestCHORE09_Bordas_IdadeENome(t *testing.T) {
	casos := []struct {
		nome    string
		arquivo string
		idade   time.Duration
		apaga   bool
	}{
		{"órfã com exatamente 1 h", "a1111111-1111-4111-8111-111111111111.jpg", limpezaimagens.IdadeMinimaOrfao, true},
		{"órfã com 1 h - 1 s", "a2222222-2222-4222-8222-222222222222.jpg", limpezaimagens.IdadeMinimaOrfao - time.Second, false},
		{".tmp-* com exatamente 24 h", ".tmp-x", limpezaimagens.IdadeMinimaTemp, true},
		{".tmp-* com 24 h - 1 s", ".tmp-y", limpezaimagens.IdadeMinimaTemp - time.Second, false},
		{"UUID maiúsculo (fora do regex)", "A3333333-3333-4333-8333-333333333333.jpg", 72 * time.Hour, false},
		{"UUID não v4 (fora do regex)", "a4444444-4444-1444-8444-444444444444.jpg", 72 * time.Hour, false},
		{"extensão .jpeg (fora do regex)", "a5555555-5555-4555-8555-555555555555.jpeg", 72 * time.Hour, false},
		{"extensão .gif (fora do regex)", "a6666666-6666-4666-8666-666666666666.gif", 72 * time.Hour, false},
		{"tmp sem o ponto inicial", "tmp-z", 72 * time.Hour, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir := t.TempDir()
			criar(t, dir, c.arquivo, c.idade)
			_, err := limpezaimagens.Limpar(limpezaimagens.Options{Dir: dir, Agora: func() time.Time { return agora }}, nil)
			require.NoError(t, err)
			assert.Equal(t, !c.apaga, existe(dir, c.arquivo))
		})
	}
}

func TestCHORE09_SubdiretorioComNomeValido_Ignorado(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, orfaAntiga)
	require.NoError(t, os.Mkdir(sub, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "dentro.jpg"), []byte("x"), 0o640))
	require.NoError(t, os.Chtimes(sub, agora.Add(-72*time.Hour), agora.Add(-72*time.Hour)))

	r, err := limpezaimagens.Limpar(limpezaimagens.Options{Dir: dir, Agora: func() time.Time { return agora }}, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, r.Ignorados)
	assert.Zero(t, r.OrfasApagadas)
	assert.True(t, existe(sub, "dentro.jpg"), "diretório não é seguido nem apagado")
}

func TestCHORE09_Run_ErrosDoBanco(t *testing.T) {
	errBanco := errors.New("falha-do-banco")
	casos := []struct {
		nome    string
		preparo func(sqlmock.Sqlmock)
		abrir   error
		msg     string
	}{
		{"abrir falha", nil, errBanco, "abrir banco"},
		{"ping falha", func(m sqlmock.Sqlmock) { m.ExpectPing().WillReturnError(errBanco); m.ExpectClose() }, nil, "conectar ao banco"},
		{"SELECT falha", func(m sqlmock.Sqlmock) {
			m.ExpectPing()
			m.ExpectQuery(`SELECT imagem FROM produtos`).WillReturnError(errBanco)
			m.ExpectClose()
		}, nil, "ler produtos.imagem"},
		{"scan falha (NULL)", func(m sqlmock.Sqlmock) {
			m.ExpectPing()
			m.ExpectQuery(`SELECT imagem FROM produtos`).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(nil))
			m.ExpectClose()
		}, nil, "ler produtos.imagem"},
		{"erro no meio das linhas", func(m sqlmock.Sqlmock) {
			m.ExpectPing()
			m.ExpectQuery(`SELECT imagem FROM produtos`).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).
				AddRow(referenciada).AddRow(orfaAntiga).RowError(1, errBanco))
			m.ExpectClose()
		}, nil, "ler produtos.imagem"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir, _ := montar(t)
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp), sqlmock.MonitorPingsOption(true))
			require.NoError(t, err)
			if c.preparo != nil {
				c.preparo(mock)
			}
			_, err = limpezaimagens.Run(limpezaimagens.Options{Dir: dir, Agora: func() time.Time { return agora }},
				func() (cmdutil.Conn, error) {
					if c.abrir != nil {
						return nil, c.abrir
					}
					return db, nil
				})
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.msg)
			assert.True(t, existe(dir, orfaAntiga), "erro no banco não apaga nada")
			if c.abrir == nil {
				assert.NoError(t, mock.ExpectationsWereMet(), "a conexão é fechada")
			}
		})
	}
}

// ambienteCLI injeta config (PRODUTO_IMAGENS_DIR = dir) e banco.
func ambienteCLI(t *testing.T, dir string, cfgErr error, db cmdutil.Conn) (cmdutil.Ambiente, *bytes.Buffer) {
	t.Helper()
	var erro bytes.Buffer
	carregar := func() (*config.Config, error) {
		if cfgErr != nil {
			return nil, cfgErr
		}
		return &config.Config{ProdutoImagensDir: dir}, nil
	}
	return cmdutil.Ambiente{
		Saida: &bytes.Buffer{}, Erro: &erro, CarregarEnv: func() {},
		Config: carregar, ConfigSemDB: carregar,
		Abrir: func(string) cmdutil.Opener { return func() (cmdutil.Conn, error) { return db, nil } },
	}, &erro
}

func mockReferencias(t *testing.T, nomes ...string) (cmdutil.Conn, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp), sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	rows := sqlmock.NewRows([]string{"imagem"})
	for _, n := range nomes {
		rows.AddRow(n)
	}
	mock.ExpectPing()
	mock.ExpectQuery(`SELECT imagem FROM produtos`).WillReturnRows(rows)
	mock.ExpectClose()
	return db, mock
}

func TestCHORE09_CLI(t *testing.T) {
	t.Run("flag desconhecida → uso", func(t *testing.T) {
		amb, erro := ambienteCLI(t, t.TempDir(), nil, nil)
		capturarLog(t)
		assert.Equal(t, cmdutil.SaidaUso, cmdutil.Executar(limpezaimagens.Tag, limpezaimagens.CLI, []string{"-apagar-tudo"}, amb))
		assert.Contains(t, erro.String(), "apagar-tudo")
	})
	t.Run("config inválida → erro sem tocar no banco", func(t *testing.T) {
		amb, _ := ambienteCLI(t, t.TempDir(), errors.New("DB_USUARIO ausente"), nil)
		buf := capturarLog(t)
		assert.Equal(t, cmdutil.SaidaErro, cmdutil.Executar(limpezaimagens.Tag, limpezaimagens.CLI, nil, amb))
		assert.Contains(t, buf.String(), "limpezaimagens: falha ao carregar config")
	})
	t.Run("-dry-run não apaga", func(t *testing.T) {
		// O CLI usa o relógio real (Options.Agora nil): idades relativas a time.Now.
		dir := t.TempDir()
		for nome, idade := range map[string]time.Duration{referenciada: 48 * time.Hour, orfaAntiga: 2 * time.Hour, orfaRecente: 0} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, nome), []byte("x"), 0o640))
			require.NoError(t, os.Chtimes(filepath.Join(dir, nome), time.Now().Add(-idade), time.Now().Add(-idade)))
		}
		db, mock := mockReferencias(t, referenciada)
		amb, _ := ambienteCLI(t, dir, nil, db)
		buf := capturarLog(t)
		assert.Equal(t, cmdutil.SaidaOK, cmdutil.Executar(limpezaimagens.Tag, limpezaimagens.CLI, []string{"-dry-run"}, amb))
		assert.True(t, existe(dir, orfaAntiga))
		assert.True(t, existe(dir, orfaRecente))
		assert.Contains(t, buf.String(), "dry_run=true")
		assert.Contains(t, buf.String(), "orfas_apagadas=1")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("sem -dry-run apaga a órfã antiga", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, orfaAntiga), []byte("x"), 0o640))
		require.NoError(t, os.Chtimes(filepath.Join(dir, orfaAntiga), time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour)))
		db, mock := mockReferencias(t)
		amb, _ := ambienteCLI(t, dir, nil, db)
		capturarLog(t)
		assert.Equal(t, cmdutil.SaidaOK, cmdutil.Executar(limpezaimagens.Tag, limpezaimagens.CLI, nil, amb))
		assert.False(t, existe(dir, orfaAntiga))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("falha ao apagar → saída de erro", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root apaga mesmo em diretório sem escrita")
		}
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, orfaAntiga), []byte("x"), 0o640))
		require.NoError(t, os.Chtimes(filepath.Join(dir, orfaAntiga), time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour)))
		require.NoError(t, os.Chmod(dir, 0o500))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		db, mock := mockReferencias(t)
		amb, _ := ambienteCLI(t, dir, nil, db)
		buf := capturarLog(t)
		assert.Equal(t, cmdutil.SaidaErro, cmdutil.Executar(limpezaimagens.Tag, limpezaimagens.CLI, nil, amb))
		assert.Contains(t, buf.String(), "falhas_ao_apagar=1")
		assert.Contains(t, buf.String(), "houve falhas ao apagar arquivos")
		assert.NotContains(t, buf.String(), orfaAntiga, "nem na falha o log leva o nome")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

package limpezaimagens_test

// CHORE-09 (Lote 31): limpeza de imagens órfãs com diretório temporário.

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/tools/limpezaimagens"
)

const (
	referenciada = "11111111-1111-4111-8111-111111111111.jpg"
	orfaAntiga   = "22222222-2222-4222-8222-222222222222.png"
	orfaRecente  = "33333333-3333-4333-8333-333333333333.webp"
	tmpAntigo    = ".tmp-44444444-4444-4444-8444-444444444444"
	tmpRecente   = ".tmp-probe-55555555"
	foraDoPadrao = "LEIA-ME.txt"
	semArquivo   = "66666666-6666-4666-8666-666666666666.jpg"
	symlinkNome  = "77777777-7777-4777-8777-777777777777.jpg"
)

var agora = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

func criar(t *testing.T, dir, nome string, idade time.Duration) {
	t.Helper()
	p := filepath.Join(dir, nome)
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o640))
	require.NoError(t, os.Chtimes(p, agora.Add(-idade), agora.Add(-idade)))
}

// montar cria o diretório com um arquivo de cada caso e um symlink (velho,
// com nome válido) para um arquivo FORA do diretório.
func montar(t *testing.T) (dir, alvoExterno string) {
	t.Helper()
	dir = t.TempDir()
	criar(t, dir, referenciada, 48*time.Hour)
	criar(t, dir, orfaAntiga, 2*time.Hour)
	criar(t, dir, orfaRecente, 10*time.Minute)
	criar(t, dir, tmpAntigo, 25*time.Hour)
	criar(t, dir, tmpRecente, 2*time.Hour)
	criar(t, dir, foraDoPadrao, 72*time.Hour)
	externo := t.TempDir()
	alvoExterno = filepath.Join(externo, "nao-apagar.jpg")
	require.NoError(t, os.WriteFile(alvoExterno, []byte("fora"), 0o640))
	require.NoError(t, os.Symlink(alvoExterno, filepath.Join(dir, symlinkNome)))
	return dir, alvoExterno
}

func existe(dir, nome string) bool {
	_, err := os.Lstat(filepath.Join(dir, nome))
	return err == nil
}

func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	anterior := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(anterior) })
	return &buf
}

func refs() map[string]bool { return map[string]bool{referenciada: true, semArquivo: true} }

func TestCHORE09_Limpar_ApagaSoOQueDeve(t *testing.T) {
	buf := capturarLog(t)
	dir, externo := montar(t)

	r, err := limpezaimagens.Limpar(limpezaimagens.Options{Dir: dir, Agora: func() time.Time { return agora }}, refs())
	require.NoError(t, err)

	assert.True(t, existe(dir, referenciada), "referenciada preservada")
	assert.False(t, existe(dir, orfaAntiga), "órfã com mais de 1 h apagada")
	assert.True(t, existe(dir, orfaRecente), "órfã recente preservada (idade mínima)")
	assert.False(t, existe(dir, tmpAntigo), ".tmp-* com mais de 24 h apagado")
	assert.True(t, existe(dir, tmpRecente), ".tmp-* recente preservado")
	assert.True(t, existe(dir, foraDoPadrao), "nome fora do regex ignorado")
	assert.True(t, existe(dir, symlinkNome), "symlink ignorado")
	_, err = os.Stat(externo)
	assert.NoError(t, err, "o alvo do symlink nunca é seguido nem apagado")

	assert.Equal(t, limpezaimagens.Relatorio{Referenciadas: 1, OrfasApagadas: 1, OrfasRecentes: 1, TempsApagados: 1,
		TempsRecentes: 1, Ignorados: 2, ReferenciasSemArquivo: 1}, r)

	saida := buf.String()
	assert.Contains(t, saida, "orfas_apagadas=1")
	assert.Contains(t, saida, "referencias_sem_arquivo=1")
	for _, proibido := range []string{orfaAntiga, tmpAntigo, referenciada, semArquivo, dir} {
		assert.NotContains(t, saida, proibido, "log só com totais")
	}
}

func TestCHORE09_DryRun_SemEfeito(t *testing.T) {
	dir, _ := montar(t)
	r, err := limpezaimagens.Limpar(limpezaimagens.Options{Dir: dir, DryRun: true, Agora: func() time.Time { return agora }}, refs())
	require.NoError(t, err)
	assert.Equal(t, 1, r.OrfasApagadas, "relata o que apagaria")
	assert.Equal(t, 1, r.TempsApagados)
	for _, nome := range []string{referenciada, orfaAntiga, orfaRecente, tmpAntigo, tmpRecente, foraDoPadrao, symlinkNome} {
		assert.True(t, existe(dir, nome), "dry-run não apaga %s", nome)
	}
}

func TestCHORE09_Run_LeReferenciasDoBanco(t *testing.T) {
	dir, _ := montar(t)
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp), sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	mock.ExpectPing()
	mock.ExpectQuery(`^SELECT imagem FROM produtos WHERE imagem IS NOT NULL AND imagem <> ''$`).
		WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(referenciada).AddRow(orfaAntiga))
	mock.ExpectClose()

	r, err := limpezaimagens.Run(limpezaimagens.Options{Dir: dir, Agora: func() time.Time { return agora }},
		func() (cmdutil.Conn, error) { return db, nil })
	require.NoError(t, err)
	assert.Equal(t, 2, r.Referenciadas)
	assert.Zero(t, r.OrfasApagadas)
	assert.True(t, existe(dir, orfaAntiga), "referenciada no banco: preservada")
	assert.NoError(t, mock.ExpectationsWereMet(), "só um SELECT, nenhum UPDATE (caso 2 só reporta)")
}

func TestCHORE09_DiretorioInexistente_Erro(t *testing.T) {
	_, err := limpezaimagens.Limpar(limpezaimagens.Options{Dir: filepath.Join(t.TempDir(), "nao-existe")}, nil)
	assert.Error(t, err)
}

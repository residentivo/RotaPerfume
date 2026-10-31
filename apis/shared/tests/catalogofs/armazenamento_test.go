package catalogofs_test

// IMG-01 (Lote 26, TestBrain): adapter catalogofs.Armazenamento (spec §4).
// Tudo roda em t.TempDir(): nada toca apis/rotaperfumes-api/data/.
//   - Novo cria o diretório (0750), abre o Root e faz o teste de escrita
//     (sem deixar o probe); caminho que não é diretório ou sem escrita → erro
//     (fail-fast do bootstrap);
//   - Gravar: nome <uuid-v4><ext> (regex), 0640, sem .tmp- sobrando,
//     extensão inválida recusada, ctx cancelado não grava;
//   - Remover/Abrir: nome fora do padrão (../x, maiúsculas, .tmp-, v1…)
//     recusado sem tocar o disco; Remover idempotente; Abrir recusa symlink
//     (para fora e para dentro) e diretório.

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"testing/cryptotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogofs"
)

var reNomeV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\.(jpg|png|webp)$`)

const uuidFixo = "3f2b8c1e-9a4d-4c7b-8e2f-1a2b3c4d5e6f"

func novo(t *testing.T) (*catalogofs.Armazenamento, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "imgs")
	a, err := catalogofs.Novo(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close() })
	return a, dir
}

func listar(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	require.NoError(t, err)
	nomes := make([]string, 0, len(ents))
	for _, e := range ents {
		nomes = append(nomes, e.Name())
	}
	return nomes
}

func TestIMG01_FS_Novo_CriaDiretorioESemProbe(t *testing.T) {
	_, dir := novo(t)
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, fs.FileMode(0o750), info.Mode().Perm()&^umaskBits(t))
	assert.Empty(t, listar(t, dir), "o teste de escrita não pode deixar arquivo")
}

func TestIMG01_FS_Novo_DiretorioExistente(t *testing.T) {
	dir := t.TempDir()
	a, err := catalogofs.Novo(dir)
	require.NoError(t, err)
	assert.NoError(t, a.Close())
}

// umaskBits devolve os bits que a umask do processo tira (para comparar perm).
func umaskBits(t *testing.T) fs.FileMode {
	t.Helper()
	p := filepath.Join(t.TempDir(), "probe")
	require.NoError(t, os.Mkdir(p, 0o777))
	info, err := os.Stat(p)
	require.NoError(t, err)
	return 0o777 &^ info.Mode().Perm()
}

func TestIMG01_FS_Novo_FailFast(t *testing.T) {
	base := t.TempDir()
	arquivo := filepath.Join(base, "arquivo")
	require.NoError(t, os.WriteFile(arquivo, []byte("x"), 0o600))
	casos := []struct {
		nome string
		dir  string
		prep func(t *testing.T) string
		msg  string
	}{
		{nome: "caminho é um arquivo", dir: arquivo, msg: "catalogofs: criar diretório"},
		{nome: "pai é um arquivo", dir: filepath.Join(arquivo, "sub"), msg: "catalogofs: criar diretório"},
		{nome: "diretório sem escrita", prep: func(t *testing.T) string {
			if os.Geteuid() == 0 {
				t.Skip("root ignora permissões de diretório")
			}
			d := filepath.Join(base, "somenteleitura")
			require.NoError(t, os.Mkdir(d, 0o500))
			t.Cleanup(func() { _ = os.Chmod(d, 0o700) })
			return d
		}, msg: "sem permissão de escrita"},
		{nome: "diretório sem leitura/execução", prep: func(t *testing.T) string {
			if os.Geteuid() == 0 {
				t.Skip("root ignora permissões de diretório")
			}
			d := filepath.Join(base, "fechado")
			require.NoError(t, os.Mkdir(d, 0o000))
			t.Cleanup(func() { _ = os.Chmod(d, 0o700) })
			return d
		}, msg: "catalogofs:"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir := c.dir
			if c.prep != nil {
				dir = c.prep(t)
			}
			a, err := catalogofs.Novo(dir)
			require.Error(t, err)
			assert.Nil(t, a)
			assert.Contains(t, err.Error(), c.msg)
		})
	}
}

func TestIMG01_FS_Gravar(t *testing.T) {
	for _, ext := range []string{".jpg", ".png", ".webp"} {
		t.Run(ext, func(t *testing.T) {
			a, dir := novo(t)
			dados := []byte("conteudo-" + ext)
			nome, err := a.Gravar(context.Background(), ext, dados)
			require.NoError(t, err)
			assert.Regexp(t, reNomeV4, nome)
			assert.True(t, strings.HasSuffix(nome, ext))
			got, err := os.ReadFile(filepath.Join(dir, nome))
			require.NoError(t, err)
			assert.Equal(t, dados, got)
			info, err := os.Stat(filepath.Join(dir, nome))
			require.NoError(t, err)
			assert.Equal(t, fs.FileMode(0o640)&^umaskBits(t), info.Mode().Perm())
			assert.Equal(t, []string{nome}, listar(t, dir), "nada de .tmp- sobrando")
		})
	}
}

func TestIMG01_FS_Gravar_NomesUnicosV4(t *testing.T) {
	a, dir := novo(t)
	vistos := map[string]bool{}
	for range 300 {
		nome, err := a.Gravar(context.Background(), ".png", []byte("x"))
		require.NoError(t, err)
		require.Regexp(t, reNomeV4, nome)
		require.False(t, vistos[nome], "nome repetido: %s", nome)
		vistos[nome] = true
	}
	assert.Len(t, listar(t, dir), 300)
}

func TestIMG01_FS_Gravar_Concorrente(t *testing.T) {
	a, dir := novo(t)
	var wg sync.WaitGroup
	nomes := make([]string, 50)
	for i := range nomes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := a.Gravar(context.Background(), ".jpg", []byte{byte(i)})
			assert.NoError(t, err)
			nomes[i] = n
		}()
	}
	wg.Wait()
	assert.Len(t, listar(t, dir), 50)
}

func TestIMG01_FS_Gravar_ExtensaoInvalida(t *testing.T) {
	for _, ext := range []string{"", ".gif", ".svg", ".jpeg", ".PNG", "/../x.png", ".png/../../x", "png"} {
		t.Run(ext, func(t *testing.T) {
			a, dir := novo(t)
			nome, err := a.Gravar(context.Background(), ext, []byte("x"))
			require.ErrorIs(t, err, catalogofs.ErrNomeInvalido)
			assert.Empty(t, nome)
			assert.Empty(t, listar(t, dir))
		})
	}
}

func TestIMG01_FS_Gravar_CtxCancelado(t *testing.T) {
	a, dir := novo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	nome, err := a.Gravar(ctx, ".png", []byte("x"))
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, nome)
	assert.Empty(t, listar(t, dir))
}

// Diretório que perdeu a permissão de escrita depois da subida: erro, e o
// temporário não fica para trás.
func TestIMG01_FS_Gravar_DiretorioSemEscrita(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignora permissões de diretório")
	}
	a, dir := novo(t)
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o750) })
	nome, err := a.Gravar(context.Background(), ".png", []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalogofs: gravar temporário")
	assert.Empty(t, nome)
	require.NoError(t, os.Chmod(dir, 0o750))
	assert.Empty(t, listar(t, dir))
}

var nomesAdulterados = []string{
	"", "../x", "../x.png", "../" + uuidFixo + ".png", "/etc/passwd", "a/" + uuidFixo + ".png",
	strings.ToUpper(uuidFixo) + ".png", uuidFixo + ".jpeg", uuidFixo + ".svg", ".tmp-" + uuidFixo,
	"3f2b8c1e-9a4d-1c7b-8e2f-1a2b3c4d5e6f.png", uuidFixo + ".png/..", "..", ".",
}

func TestIMG01_FS_Remover_NomeAdulterado_NaoTocaDisco(t *testing.T) {
	a, dir := novo(t)
	// alvos plausíveis do traversal, fora do Root e dentro dele
	fora := filepath.Join(filepath.Dir(dir), "x.png")
	require.NoError(t, os.WriteFile(fora, []byte("fora"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(dir), "x"), []byte("fora"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(dir), uuidFixo+".png"), []byte("fora"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tmp-"+uuidFixo), []byte("tmp"), 0o600))
	for _, nome := range nomesAdulterados {
		t.Run(nome, func(t *testing.T) {
			err := a.Remover(context.Background(), nome)
			require.ErrorIs(t, err, catalogofs.ErrNomeInvalido)
		})
	}
	assert.FileExists(t, fora)
	assert.FileExists(t, filepath.Join(filepath.Dir(dir), "x"))
	assert.FileExists(t, filepath.Join(filepath.Dir(dir), uuidFixo+".png"))
	assert.FileExists(t, filepath.Join(dir, ".tmp-"+uuidFixo))
}

func TestIMG01_FS_Remover(t *testing.T) {
	a, dir := novo(t)
	nome, err := a.Gravar(context.Background(), ".png", []byte("x"))
	require.NoError(t, err)

	require.NoError(t, a.Remover(context.Background(), nome))
	assert.NoFileExists(t, filepath.Join(dir, nome))
	// idempotente
	require.NoError(t, a.Remover(context.Background(), nome))
	require.NoError(t, a.Remover(context.Background(), uuidFixo+".webp"))
}

// Com ctx já cancelado nada é apagado e o erro do ctx volta ao chamador
// (o caso de uso chama com context.WithoutCancel para sempre limpar).
func TestIMG01_FS_Remover_CtxCancelado(t *testing.T) {
	a, dir := novo(t)
	nome, err := a.Gravar(context.Background(), ".png", []byte("x"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, a.Remover(ctx, nome), context.Canceled)
	assert.FileExists(t, filepath.Join(dir, nome))
	// sem cancelamento, a mesma chamada apaga
	require.NoError(t, a.Remover(context.Background(), nome))
	assert.NoFileExists(t, filepath.Join(dir, nome))
}

// Falha do disco que não é "não existe" (aqui: diretório não vazio com nome
// válido) sobe embrulhada.
func TestIMG01_FS_Remover_ErroDeDisco(t *testing.T) {
	a, dir := novo(t)
	nome := uuidFixo + ".png"
	require.NoError(t, os.MkdirAll(filepath.Join(dir, nome, "filho"), 0o700))
	err := a.Remover(context.Background(), nome)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalogofs: remover")
	assert.DirExists(t, filepath.Join(dir, nome))
}

func TestIMG01_FS_Abrir(t *testing.T) {
	a, _ := novo(t)
	antes := time.Now().Add(-time.Minute)
	nome, err := a.Gravar(context.Background(), ".webp", []byte("conteudo-webp"))
	require.NoError(t, err)
	f, mod, err := a.Abrir(nome)
	require.NoError(t, err)
	defer f.Close()
	got, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "conteudo-webp", string(got))
	assert.True(t, mod.After(antes))
	// é seekable (http.ServeContent usa Seek)
	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err)
}

func TestIMG01_FS_Abrir_Recusa(t *testing.T) {
	a, dir := novo(t)
	pai := filepath.Dir(dir)
	segredo := filepath.Join(pai, "segredo.txt")
	require.NoError(t, os.WriteFile(segredo, []byte("segredo"), 0o600))
	dentro, err := a.Gravar(context.Background(), ".png", []byte("img"))
	require.NoError(t, err)

	linkFora := "11111111-1111-4111-8111-111111111111.png"
	linkDentro := "22222222-2222-4222-9222-222222222222.png"
	linkAbsoluto := "33333333-3333-4333-a333-333333333333.jpg"
	diretorio := "44444444-4444-4444-b444-444444444444.webp"
	require.NoError(t, os.Symlink("../segredo.txt", filepath.Join(dir, linkFora)))
	require.NoError(t, os.Symlink(dentro, filepath.Join(dir, linkDentro)))
	require.NoError(t, os.Symlink(segredo, filepath.Join(dir, linkAbsoluto)))
	require.NoError(t, os.Mkdir(filepath.Join(dir, diretorio), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tmp-"+uuidFixo), []byte("tmp"), 0o600))

	casos := append([]string{linkFora, linkDentro, linkAbsoluto, diretorio, uuidFixo + ".png"}, nomesAdulterados...)
	for _, nome := range casos {
		t.Run(nome, func(t *testing.T) {
			f, mod, err := a.Abrir(nome)
			require.ErrorIs(t, err, catalogo.ErrImagemNaoEncontrada)
			assert.Nil(t, f)
			assert.True(t, mod.IsZero())
		})
	}
}

// sequenciaDeterministica devolve os 4 primeiros nomes que o adapter gera
// depois do Novo (o probe consome um UUID) com a semente dada.
func sequenciaDeterministica(t *testing.T, semente uint64) []string {
	t.Helper()
	cryptotest.SetGlobalRandom(t, semente)
	a, _ := novo(t)
	out := make([]string, 4)
	for i := range out {
		n, err := a.Gravar(context.Background(), ".png", []byte("x"))
		require.NoError(t, err)
		out[i] = n
	}
	return out
}

// Colisão de UUID (forçada com crypto/rand determinístico): sorteia outro;
// três colisões seguidas → erro, sem gravar nada.
func TestIMG01_FS_Gravar_ColisaoDeNome(t *testing.T) {
	const semente = 26
	nomes := sequenciaDeterministica(t, semente)

	t.Run("uma colisão: usa o próximo UUID", func(t *testing.T) {
		cryptotest.SetGlobalRandom(t, semente)
		a, dir := novo(t)
		require.NoError(t, os.WriteFile(filepath.Join(dir, nomes[0]), []byte("existente"), 0o600))
		n, err := a.Gravar(context.Background(), ".png", []byte("novo"))
		require.NoError(t, err)
		assert.Equal(t, nomes[1], n)
		got, _ := os.ReadFile(filepath.Join(dir, nomes[0]))
		assert.Equal(t, "existente", string(got), "o arquivo existente não pode ser sobrescrito")
	})
	t.Run("colisão com symlink pendente também conta", func(t *testing.T) {
		cryptotest.SetGlobalRandom(t, semente)
		a, dir := novo(t)
		require.NoError(t, os.Symlink("/nao/existe", filepath.Join(dir, nomes[0])))
		n, err := a.Gravar(context.Background(), ".png", []byte("novo"))
		require.NoError(t, err)
		assert.Equal(t, nomes[1], n)
	})
	t.Run("três colisões: erro", func(t *testing.T) {
		cryptotest.SetGlobalRandom(t, semente)
		a, dir := novo(t)
		for _, n := range nomes[:3] {
			require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("existente"), 0o600))
		}
		n, err := a.Gravar(context.Background(), ".png", []byte("novo"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nome livre")
		assert.Empty(t, n)
		assert.Len(t, listar(t, dir), 3)
	})
	t.Run("temporário já existe: erro sem gravar o final", func(t *testing.T) {
		cryptotest.SetGlobalRandom(t, semente)
		a, dir := novo(t)
		tmp := ".tmp-" + strings.TrimSuffix(nomes[0], ".png")
		require.NoError(t, os.Mkdir(filepath.Join(dir, tmp), 0o700))
		n, err := a.Gravar(context.Background(), ".png", []byte("novo"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "catalogofs: gravar temporário")
		assert.Empty(t, n)
		assert.NoFileExists(t, filepath.Join(dir, nomes[0]))
		assert.DirExists(t, filepath.Join(dir, tmp), "o temporário alheio não pode ser apagado")
	})
	t.Run("temporário alheio (arquivo) continua intacto", func(t *testing.T) {
		cryptotest.SetGlobalRandom(t, semente)
		a, dir := novo(t)
		tmp := filepath.Join(dir, ".tmp-"+strings.TrimSuffix(nomes[0], ".png"))
		require.NoError(t, os.WriteFile(tmp, []byte("outra-gravacao"), 0o600))
		n, err := a.Gravar(context.Background(), ".png", []byte("novo"))
		require.Error(t, err)
		require.ErrorIs(t, err, fs.ErrExist)
		assert.Empty(t, n)
		assert.NoFileExists(t, filepath.Join(dir, nomes[0]))
		got, err := os.ReadFile(tmp)
		require.NoError(t, err, "o temporário alheio tem de continuar no disco")
		assert.Equal(t, "outra-gravacao", string(got))
	})
}

// O teste de escrita do Novo falha (aqui: o nome do probe já está ocupado
// por um diretório) → erro de subida, Root fechado.
func TestIMG01_FS_Novo_TesteDeEscritaFalha(t *testing.T) {
	const semente = 77
	a, _ := novo(t)
	cryptotest.SetGlobalRandom(t, semente)
	primeiro, err := a.Gravar(context.Background(), ".png", []byte("x"))
	require.NoError(t, err)
	probe := ".tmp-probe-" + strings.TrimSuffix(primeiro, ".png")

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, probe), 0o700))
	cryptotest.SetGlobalRandom(t, semente)
	b, err := catalogofs.Novo(dir)
	require.Error(t, err)
	assert.Nil(t, b)
	assert.Contains(t, err.Error(), "sem permissão de escrita")
	assert.DirExists(t, filepath.Join(dir, probe))
}

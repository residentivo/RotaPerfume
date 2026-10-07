package seedusers_test

// SEC-10: seedusers não altera sql/, renderiza em <raiz>/tmp/seed (0600,
// dir 0700), apaga as cópias após executar (inclusive em erro), exige
// DB_USUARIO/DB_SENHA só quando vai executar SQL e não tem senha padrão.
// Reaproveita raizFalsa/ler/execucoes/cfgRapida/credenciaisDB/silenciarLog de
// seedusers_test.go.

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tools/seedusers"
)

// sec10Unset remove a variável (ausente, não vazia) e a restaura no Cleanup.
func sec10Unset(t *testing.T, k string) {
	t.Helper()
	t.Setenv(k, "")
	require.NoError(t, os.Unsetenv(k))
}

// arquivosEm lista os arquivos de dir (vazio se dir não existir).
func arquivosEm(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	var nomes []string
	for _, e := range ents {
		nomes = append(nomes, e.Name())
	}
	return nomes
}

func TestSEC10_Run_ExigeCredenciaisDB(t *testing.T) {
	casos := []struct {
		nome    string
		usuario string // "<unset>" = ausente
		senha   string
	}{
		{"ambas ausentes", "<unset>", "<unset>"},
		{"usuário ausente", "<unset>", "s"},
		{"senha ausente", "u", "<unset>"},
		{"usuário vazio", "", "s"},
		{"senha vazia", "u", ""},
		{"ambas vazias", "", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			silenciarLog(t)
			t.Setenv("SEED_ADMIN_PASSWORD", "a")
			t.Setenv("SEED_USER_PASSWORD", "b")
			for k, v := range map[string]string{"DB_USUARIO": c.usuario, "DB_SENHA": c.senha} {
				if v == "<unset>" {
					sec10Unset(t, k)
				} else {
					t.Setenv(k, v)
				}
			}
			root := raizFalsa(t)
			err := seedusers.Run(cfgRapida(), seedusers.Options{}, seedusers.Deps{
				Out: &bytes.Buffer{},
				ProjectRoot: func() (string, error) {
					t.Error("não deveria nem localizar a raiz")
					return root, nil
				},
				RunSQL: func(*config.Config, string) error { t.Error("RunSQL não deveria ser chamado"); return nil },
			})
			require.Error(t, err)
			assert.ErrorIs(t, err, seedusers.ErrCredenciaisDB)
			assert.Contains(t, err.Error(), "defina DB_USUARIO/DB_SENHA no .env")
			assert.Empty(t, arquivosEm(t, filepath.Join(root, "tmp", "seed")), "nada é renderizado")
		})
	}
}

func TestSEC10_Run_NoExecEDryRunNaoExigemCredenciais(t *testing.T) {
	casos := []struct {
		nome string
		opts seedusers.Options
	}{
		{"-no-exec", seedusers.Options{NoExec: true}},
		{"-dry-run", seedusers.Options{DryRun: true}},
		{"-no-exec -dry-run", seedusers.Options{NoExec: true, DryRun: true}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			silenciarLog(t)
			t.Setenv("SEED_ADMIN_PASSWORD", "a")
			t.Setenv("SEED_USER_PASSWORD", "b")
			sec10Unset(t, "DB_USUARIO")
			sec10Unset(t, "DB_SENHA")
			root := raizFalsa(t)
			var out bytes.Buffer
			err := seedusers.Run(cfgRapida(), c.opts, seedusers.Deps{
				Out:         &out,
				ProjectRoot: func() (string, error) { return root, nil },
				RunSQL:      func(*config.Config, string) error { t.Error("RunSQL não deveria ser chamado"); return nil },
			})
			require.NoError(t, err)
		})
	}
}

func TestSEC10_Run_NoExecMantemArquivos(t *testing.T) {
	silenciarLog(t)
	t.Setenv("SEED_ADMIN_PASSWORD", "a")
	t.Setenv("SEED_USER_PASSWORD", "b")
	root := raizFalsa(t)
	var out bytes.Buffer
	err := seedusers.Run(cfgRapida(), seedusers.Options{NoExec: true}, seedusers.Deps{
		Out: &out, ProjectRoot: func() (string, error) { return root, nil },
		RunSQL: func(*config.Config, string) error { t.Error("não executa"); return nil },
	})
	require.NoError(t, err)
	seedDir := filepath.Join(root, "tmp", "seed")
	assert.ElementsMatch(t, []string{"02_seed_admin.sql", "03_seed_vendedores.sql"}, arquivosEm(t, seedDir))
	for _, f := range []string{"02_seed_admin.sql", "03_seed_vendedores.sql"} {
		p := filepath.Join(seedDir, f)
		assert.Contains(t, out.String(), p, "imprime o caminho renderizado")
		assert.Regexp(t, reHash, ler(t, p))
	}
	assert.Contains(t, out.String(), "apague-os após o uso")
	assert.Equal(t, sqlAdmin, ler(t, filepath.Join(root, "sql", "02_seed_admin.sql")))
	assert.Equal(t, sqlVend, ler(t, filepath.Join(root, "sql", "03_seed_vendedores.sql")))
}

// TestSEC10_Run_ApagaTmpSeed: após executar não sobra nada em tmp/seed,
// inclusive quando o RunSQL ou a renderização falham no meio.
func TestSEC10_Run_ApagaTmpSeed(t *testing.T) {
	casos := []struct {
		nome    string
		falhar  string
		prepara func(t *testing.T, root string)
		wantErr string
	}{
		{nome: "sucesso"},
		{nome: "RunSQL falha no 1º arquivo", falhar: "02_seed_admin.sql", wantErr: "mysql falhou"},
		{nome: "RunSQL falha no 2º arquivo", falhar: "03_seed_vendedores.sql", wantErr: "mysql falhou"},
		{
			nome: "2º SQL sem placeholder (1º já renderizado)",
			prepara: func(t *testing.T, root string) {
				require.NoError(t, os.WriteFile(filepath.Join(root, "sql", "03_seed_vendedores.sql"), []byte("SELECT 1;\n"), 0o600))
			},
			wantErr: "sem placeholder",
		},
		{
			nome: "cópia antiga em tmp/seed é sobrescrita e apagada",
			prepara: func(t *testing.T, root string) {
				dir := filepath.Join(root, "tmp", "seed")
				require.NoError(t, os.MkdirAll(dir, 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "02_seed_admin.sql"), []byte("velho"), 0o644))
			},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			silenciarLog(t)
			t.Setenv("SEED_ADMIN_PASSWORD", "a")
			t.Setenv("SEED_USER_PASSWORD", "b")
			credenciaisDB(t)
			root := raizFalsa(t)
			if c.prepara != nil {
				c.prepara(t, root)
			}
			var ex execucoes
			err := seedusers.Run(cfgRapida(), seedusers.Options{}, seedusers.Deps{
				Out: &bytes.Buffer{}, ProjectRoot: func() (string, error) { return root, nil }, RunSQL: ex.runner(c.falhar),
			})
			if c.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Empty(t, arquivosEm(t, filepath.Join(root, "tmp", "seed")), "tmp/seed deve ficar vazio")
			// Durante a execução o RunSQL recebeu a cópia renderizada (com hash).
			for _, conteudo := range ex.conteudos {
				assert.NotContains(t, conteudo, "PLACEHOLDER")
				assert.Regexp(t, reHash, conteudo)
			}
			assert.Equal(t, sqlAdmin, ler(t, filepath.Join(root, "sql", "02_seed_admin.sql")), "sql/ intacto")
		})
	}
}

func TestSEC10_RenderSeedFile(t *testing.T) {
	const hashA = "$2a$04$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	const hashU = "$2a$04$UUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUUU"
	repl := map[string]string{seedusers.PlaceholderAdmin: hashA, seedusers.PlaceholderUser: hashU}
	// Conteúdo com CRLF, UTF-8 e BOM: src deve ficar idêntico byte a byte.
	src := "\xef\xbb\xbf-- comentário ção\r\nINSERT ('" + seedusers.PlaceholderAdmin + "');\r\nSET @h='" +
		seedusers.PlaceholderUser + "';\r\nSET @h2='" + seedusers.PlaceholderUser + "';\r\n"

	casos := []struct {
		nome     string
		conteudo *string // nil = src inexistente
		wantN    int
		wantErr  error
		wantAny  bool // erro qualquer (não sentinela)
	}{
		{nome: "3 placeholders", conteudo: &src, wantN: 3},
		{nome: "0 placeholders", conteudo: strPtr("SELECT '$2a$12$outro';\n"), wantErr: seedusers.ErrSemPlaceholder},
		{nome: "arquivo vazio", conteudo: strPtr(""), wantErr: seedusers.ErrSemPlaceholder},
		{nome: "src inexistente", conteudo: nil, wantErr: os.ErrNotExist},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir := t.TempDir()
			srcPath := filepath.Join(dir, "sql", "02_seed_admin.sql")
			require.NoError(t, os.MkdirAll(filepath.Dir(srcPath), 0o755))
			var antes []byte
			if c.conteudo != nil {
				antes = []byte(*c.conteudo)
				require.NoError(t, os.WriteFile(srcPath, antes, 0o644))
			}
			dstDir := filepath.Join(dir, "tmp", "seed")

			dst, n, err := seedusers.RenderSeedFile(srcPath, dstDir, repl)

			if c.conteudo != nil {
				depois, rerr := os.ReadFile(srcPath)
				require.NoError(t, rerr)
				assert.True(t, bytes.Equal(antes, depois), "src deve ficar idêntico byte a byte")
			}
			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				assert.Empty(t, dst)
				assert.Zero(t, n)
				assert.NoDirExists(t, dstDir, "nada é criado quando não há o que gravar")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantN, n)
			assert.Equal(t, filepath.Join(dstDir, "02_seed_admin.sql"), dst)
			got := ler(t, dst)
			assert.NotContains(t, got, "PLACEHOLDER")
			assert.Equal(t, 1, strings.Count(got, hashA))
			assert.Equal(t, 2, strings.Count(got, hashU))
			assert.Equal(t, strings.NewReplacer(seedusers.PlaceholderAdmin, hashA, seedusers.PlaceholderUser, hashU).Replace(src), got,
				"o resto do arquivo (CRLF, BOM, UTF-8) é preservado")
			if runtime.GOOS != "windows" {
				fi, err := os.Stat(dst)
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
				di, err := os.Stat(dstDir)
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0o700), di.Mode().Perm())
			}
		})
	}
	t.Run("sobrescreve cópia existente e força 0600", func(t *testing.T) {
		dir := t.TempDir()
		srcPath := filepath.Join(dir, "a.sql")
		require.NoError(t, os.WriteFile(srcPath, []byte(seedusers.PlaceholderAdmin), 0o644))
		dstDir := filepath.Join(dir, "out")
		require.NoError(t, os.MkdirAll(dstDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dstDir, "a.sql"), []byte("conteúdo antigo bem maior que o novo ......"), 0o644))
		dst, n, err := seedusers.RenderSeedFile(srcPath, dstDir, repl)
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, hashA, ler(t, dst))
		if runtime.GOOS != "windows" {
			fi, err := os.Stat(dst)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
		}
	})
	t.Run("dstDir impossível de criar (é um arquivo)", func(t *testing.T) {
		dir := t.TempDir()
		srcPath := filepath.Join(dir, "a.sql")
		require.NoError(t, os.WriteFile(srcPath, []byte(seedusers.PlaceholderAdmin), 0o644))
		bloqueio := filepath.Join(dir, "bloqueio")
		require.NoError(t, os.WriteFile(bloqueio, []byte("x"), 0o644))
		_, _, err := seedusers.RenderSeedFile(srcPath, filepath.Join(bloqueio, "seed"), repl)
		assert.Error(t, err)
	})
	t.Run("dst é um diretório (escrita falha)", func(t *testing.T) {
		dir := t.TempDir()
		srcPath := filepath.Join(dir, "a.sql")
		require.NoError(t, os.WriteFile(srcPath, []byte(seedusers.PlaceholderAdmin), 0o644))
		dstDir := filepath.Join(dir, "out")
		require.NoError(t, os.MkdirAll(filepath.Join(dstDir, "a.sql"), 0o700))
		_, _, err := seedusers.RenderSeedFile(srcPath, dstDir, repl)
		assert.Error(t, err)
	})
}

func strPtr(s string) *string { return &s }

func TestSEC10_SeedFiles(t *testing.T) {
	root := filepath.Join("x", "raiz")
	assert.Equal(t, []string{
		filepath.Join(root, "sql", "02_seed_admin.sql"),
		filepath.Join(root, "sql", "03_seed_vendedores.sql"),
	}, seedusers.SeedFiles(root))
}

// TestSEC10_SQLsReaisTemPlaceholders: guarda de regressão. Os SQLs versionados
// em sql/ devem conter os placeholders e nenhum hash real, bcrypt ou argon2id
// (ex.: um seedusers antigo que tenha gravado por cima, ou commit acidental).
func TestSEC10_SQLsReaisTemPlaceholders(t *testing.T) {
	root, err := cmdutil.FindProjectRoot()
	require.NoError(t, err)
	reHashReal := regexp.MustCompile(`\$2[abxy]\$|\$argon2(id|i|d)\$`)
	casos := []struct {
		arquivo     string
		placeholder string
	}{
		{"02_seed_admin.sql", seedusers.PlaceholderAdmin},
		{"03_seed_vendedores.sql", seedusers.PlaceholderUser},
	}
	for i, c := range casos {
		t.Run(c.arquivo, func(t *testing.T) {
			p := seedusers.SeedFiles(root)[i]
			require.Equal(t, c.arquivo, filepath.Base(p))
			conteudo := ler(t, p)
			assert.Contains(t, conteudo, c.placeholder)
			semPlaceholders := strings.NewReplacer(seedusers.PlaceholderAdmin, "", seedusers.PlaceholderUser, "").Replace(conteudo)
			assert.Empty(t, reHashReal.FindAllString(semPlaceholders, -1), "há hash real (não placeholder) em %s", p)
		})
	}
}

// TestSEC10_Run_FalhaAoApagarAvisa: no Windows um handle aberto impede o
// os.Remove; o Run não falha por isso, mas loga para apagar manualmente.
func TestSEC10_Run_FalhaAoApagarAvisa(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("só no Windows um arquivo aberto não pode ser removido")
	}
	logs := silenciarLog(t)
	t.Setenv("SEED_ADMIN_PASSWORD", "a")
	t.Setenv("SEED_USER_PASSWORD", "b")
	credenciaisDB(t)
	root := raizFalsa(t)
	var abertos []*os.File
	t.Cleanup(func() {
		for _, f := range abertos {
			_ = f.Close()
		}
	})
	err := seedusers.Run(cfgRapida(), seedusers.Options{}, seedusers.Deps{
		Out: &bytes.Buffer{}, ProjectRoot: func() (string, error) { return root, nil },
		RunSQL: func(_ *config.Config, file string) error {
			f, err := os.Open(file) // mantém aberto até o fim do teste
			require.NoError(t, err)
			abertos = append(abertos, f)
			return nil
		},
	})
	require.NoError(t, err)
	assert.Contains(t, logs.String(), "não foi possível apagar")
	assert.Contains(t, logs.String(), "apague manualmente")
}

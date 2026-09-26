package seedusers_test

// TST-03 (Lote 7): tools/seedusers com Deps falsas (raiz do projeto em
// t.TempDir(), RunSQL gravando as chamadas e saída em bytes.Buffer).

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tools/seedusers"
)

var errMySQL = errors.New("mysql saiu com código 1")

const (
	sqlAdmin = "INSERT INTO usuarios VALUES ('admin', '" + seedusers.PlaceholderAdmin + "');\n"
	sqlVend  = "INSERT INTO usuarios VALUES ('v1', '" + seedusers.PlaceholderUser + "');\n" +
		"INSERT INTO usuarios VALUES ('v2', '" + seedusers.PlaceholderUser + "');\n"
)

func silenciarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	saida, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(saida); log.SetFlags(flags) })
	return &buf
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	antigo, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(antigo) })
}

// raizFalsa cria <tmp>/sql/02_seed_admin.sql e 03_seed_vendedores.sql com
// os placeholders.
func raizFalsa(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sql"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sql", "02_seed_admin.sql"), []byte(sqlAdmin), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sql", "03_seed_vendedores.sql"), []byte(sqlVend), 0o600))
	return root
}

func ler(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

type execucoes struct{ arquivos []string }

func (e *execucoes) runner(falharEm string) seedusers.SQLRunner {
	return func(_ *config.Config, file string) error {
		e.arquivos = append(e.arquivos, filepath.Base(file))
		if filepath.Base(file) == falharEm {
			return errMySQL
		}
		return nil
	}
}

func cfgRapida() *config.Config { return &config.Config{BCryptCost: bcrypt.MinCost} }

var reHash = regexp.MustCompile(`\$2a\$04\$[./A-Za-z0-9]{53}`)

func TestRun_SubstituiEExecuta(t *testing.T) {
	logs := silenciarLog(t)
	t.Setenv("SEED_ADMIN_PASSWORD", "Admin@Env1")
	t.Setenv("SEED_USER_PASSWORD", "User@Env1")
	root := raizFalsa(t)
	var ex execucoes
	var out bytes.Buffer

	err := seedusers.Run(cfgRapida(), seedusers.Options{}, seedusers.Deps{
		Out: &out, ProjectRoot: func() (string, error) { return root, nil }, RunSQL: ex.runner(""),
	})
	require.NoError(t, err)

	admin := ler(t, filepath.Join(root, "sql", "02_seed_admin.sql"))
	vend := ler(t, filepath.Join(root, "sql", "03_seed_vendedores.sql"))
	assert.NotContains(t, admin+vend, "PLACEHOLDER", "nenhum placeholder sobra")

	hAdmin := reHash.FindString(admin)
	hVend := reHash.FindAllString(vend, -1)
	require.NotEmpty(t, hAdmin)
	require.Len(t, hVend, 2)
	assert.Equal(t, hVend[0], hVend[1], "os vendedores recebem o mesmo hash")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hAdmin), []byte("Admin@Env1")))
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hVend[0]), []byte("User@Env1")))

	assert.Equal(t, []string{"02_seed_admin.sql", "03_seed_vendedores.sql"}, ex.arquivos, "executa na ordem")
	assert.Contains(t, logs.String(), "total de placeholders substituídos: 3")
	assert.Contains(t, logs.String(), "OK — 2 arquivos SQL aplicados")
	assert.NotContains(t, out.String(), "Admin@Env1", "senha só aparece com --show-password")
	assert.Contains(t, out.String(), "use --show-password para exibir")
}

func TestRun_ShowPasswordENoExec(t *testing.T) {
	silenciarLog(t)
	t.Setenv("SEED_ADMIN_PASSWORD", "Admin@Env1")
	t.Setenv("SEED_USER_PASSWORD", "User@Env1")
	root := raizFalsa(t)
	var ex execucoes
	var out bytes.Buffer

	err := seedusers.Run(cfgRapida(), seedusers.Options{NoExec: true, ShowPassword: true}, seedusers.Deps{
		Out: &out, ProjectRoot: func() (string, error) { return root, nil }, RunSQL: ex.runner(""),
	})
	require.NoError(t, err)
	assert.Empty(t, ex.arquivos, "--no-exec não roda o mysql")
	assert.Contains(t, out.String(), "ADMIN_PASSWORD=Admin@Env1")
	assert.Contains(t, out.String(), "USER_PASSWORD =User@Env1")
	assert.NotContains(t, ler(t, filepath.Join(root, "sql", "02_seed_admin.sql")), "PLACEHOLDER")
}

func TestRun_DryRunNaoTocaArquivos(t *testing.T) {
	silenciarLog(t)
	t.Setenv("SEED_ADMIN_PASSWORD", "")
	t.Setenv("SEED_USER_PASSWORD", "")
	root := raizFalsa(t)
	var out bytes.Buffer
	err := seedusers.Run(cfgRapida(), seedusers.Options{DryRun: true}, seedusers.Deps{
		Out: &out,
		ProjectRoot: func() (string, error) {
			t.Fatal("dry-run não precisa da raiz")
			return "", nil
		},
		RunSQL: func(*config.Config, string) error { t.Fatal("dry-run não executa SQL"); return nil },
	})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "=== DRY-RUN ===")
	assert.Len(t, reHash.FindAllString(out.String(), -1), 2, "imprime os dois hashes")
	assert.Equal(t, sqlAdmin, ler(t, filepath.Join(root, "sql", "02_seed_admin.sql")))
}

func TestRun_Erros(t *testing.T) {
	casos := []struct {
		nome   string
		cfg    *config.Config
		root   func(t *testing.T) (string, error)
		falhar string
		errSub string
		errIs  error
	}{
		{
			nome:   "custo bcrypt inválido",
			cfg:    &config.Config{BCryptCost: bcrypt.MaxCost + 1},
			errSub: "hash admin falhou",
		},
		{
			nome:   "raiz do projeto não encontrada",
			root:   func(*testing.T) (string, error) { return "", os.ErrNotExist },
			errIs:  os.ErrNotExist,
			errSub: "file does not exist",
		},
		{
			nome:   "SQL de seed ausente",
			root:   func(t *testing.T) (string, error) { return t.TempDir(), nil },
			errSub: "02_seed_admin.sql",
		},
		{
			nome:   "mysql falha no segundo arquivo",
			root:   func(t *testing.T) (string, error) { return raizFalsa(t), nil },
			falhar: "03_seed_vendedores.sql",
			errIs:  errMySQL,
			errSub: "mysql falhou em",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			silenciarLog(t)
			t.Setenv("SEED_ADMIN_PASSWORD", "a")
			t.Setenv("SEED_USER_PASSWORD", "b")
			cfg := c.cfg
			if cfg == nil {
				cfg = cfgRapida()
			}
			root := func() (string, error) { return raizFalsa(t), nil }
			if c.root != nil {
				root = func() (string, error) { return c.root(t) }
			}
			var ex execucoes
			err := seedusers.Run(cfg, seedusers.Options{}, seedusers.Deps{Out: &bytes.Buffer{}, ProjectRoot: root, RunSQL: ex.runner(c.falhar)})
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.errSub)
			if c.errIs != nil {
				assert.ErrorIs(t, err, c.errIs)
			}
		})
	}
}

func TestResolveSeedPassword(t *testing.T) {
	logs := silenciarLog(t)
	t.Setenv("SEED_X", "definida")
	got, err := seedusers.ResolveSeedPassword("SEED_X", "padrao")
	require.NoError(t, err)
	assert.Equal(t, "definida", got)

	t.Setenv("SEED_X", "")
	got, err = seedusers.ResolveSeedPassword("SEED_X", "padrao")
	require.NoError(t, err)
	assert.Len(t, got, 16)
	assert.NotEqual(t, "padrao", got, "modo dev gera senha aleatória, não usa o default fixo")
	assert.Contains(t, logs.String(), "SEED_X não definido")
}

func TestGenerateRandomPassword(t *testing.T) {
	a, err := seedusers.GenerateRandomPassword(32)
	require.NoError(t, err)
	b, err := seedusers.GenerateRandomPassword(32)
	require.NoError(t, err)
	assert.Regexp(t, `^[a-zA-Z0-9]{32}$`, a)
	assert.NotEqual(t, a, b)
}

func TestShortHash(t *testing.T) {
	assert.Equal(t, "curto", seedusers.ShortHash("curto"))
	assert.Equal(t, "12345678901234567890", seedusers.ShortHash("12345678901234567890"))
	assert.Equal(t, "12345678901234567890", seedusers.ShortHash("12345678901234567890XYZ"))
}

func TestReplaceInFile(t *testing.T) {
	dir := t.TempDir()
	t.Run("substitui todas as ocorrências de todas as chaves", func(t *testing.T) {
		p := filepath.Join(dir, "a.sql")
		require.NoError(t, os.WriteFile(p, []byte("A B A C"), 0o600))
		n, err := seedusers.ReplaceInFile(p, map[string]string{"A": "x", "C": "y", "Z": "w"})
		require.NoError(t, err)
		assert.Equal(t, 3, n)
		assert.Equal(t, "x B x y", ler(t, p))
	})
	t.Run("sem ocorrências não regrava o arquivo", func(t *testing.T) {
		p := filepath.Join(dir, "b.sql")
		require.NoError(t, os.WriteFile(p, []byte("nada"), 0o400)) // só leitura
		n, err := seedusers.ReplaceInFile(p, map[string]string{"A": "x"})
		require.NoError(t, err)
		assert.Zero(t, n)
	})
	t.Run("arquivo ausente", func(t *testing.T) {
		_, err := seedusers.ReplaceInFile(filepath.Join(dir, "nada.sql"), map[string]string{"A": "x"})
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
	t.Run("arquivo só leitura com ocorrência", func(t *testing.T) {
		p := filepath.Join(dir, "c.sql")
		require.NoError(t, os.WriteFile(p, []byte("A"), 0o400))
		_, err := seedusers.ReplaceInFile(p, map[string]string{"A": "x"})
		assert.Error(t, err)
	})
}

func TestMySQLArgs_SenhaNaoVaiNaLinhaDeComando(t *testing.T) {
	cfg := &config.Config{DBUsuario: "u", DBSenha: "segredo", DBHost: "h", DBPort: "3307", DBName: "rp"}
	args := seedusers.MySQLArgs(cfg)
	assert.Equal(t, []string{"-uu", "-hh", "-P3307", "--default-character-set=utf8mb4", "--local-infile=1", "rp"}, args)
	assert.NotContains(t, strings.Join(args, " "), "segredo", "senha vai via MYSQL_PWD")
}

func TestRunMySQL(t *testing.T) {
	cfg := &config.Config{DBUsuario: "u", DBSenha: "s", DBHost: "h", DBPort: "1", DBName: "d"}
	t.Run("arquivo ausente", func(t *testing.T) {
		err := seedusers.RunMySQL(cfg, filepath.Join(t.TempDir(), "nada.sql"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "leitura de")
	})
	t.Run("cliente mysql fora do PATH", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "x.sql")
		require.NoError(t, os.WriteFile(p, []byte("SELECT 1;"), 0o600))
		t.Setenv("PATH", t.TempDir())
		assert.Error(t, seedusers.RunMySQL(cfg, p))
	})
}

func TestFindProjectRoot(t *testing.T) {
	t.Run("acha a pasta com go.mod e apis/shared/go.mod", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, "apis", "shared", "cmd", "seedusers"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(root, "apis", "shared", "go.mod"), []byte("module y\n"), 0o600))
		chdir(t, filepath.Join(root, "apis", "shared", "cmd", "seedusers"))
		got, err := seedusers.FindProjectRoot()
		require.NoError(t, err)
		assertMesmoDir(t, root, got)
	})
	t.Run("sem go.mod na raiz cai no fallback cwd/../../..", func(t *testing.T) {
		base := t.TempDir()
		fundo := filepath.Join(base, "a", "b", "c")
		require.NoError(t, os.MkdirAll(fundo, 0o755))
		chdir(t, fundo)
		got, err := seedusers.FindProjectRoot()
		require.NoError(t, err)
		assertMesmoDir(t, base, got)
	})
}

// assertMesmoDir compara diretórios resolvendo links (o TEMP do Windows pode
// vir em formato curto 8.3).
func assertMesmoDir(t *testing.T, want, got string) {
	t.Helper()
	w, err := filepath.EvalSymlinks(want)
	require.NoError(t, err)
	g, err := filepath.EvalSymlinks(got)
	require.NoError(t, err)
	assert.Equal(t, w, g)
}

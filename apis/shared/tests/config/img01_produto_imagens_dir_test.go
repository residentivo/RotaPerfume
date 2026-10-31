package config_test

// IMG-01 (Lote 26, TestBrain): PRODUTO_IMAGENS_DIR.
//   - ausente → ./data/produtos-imagens, absoluto e limpo;
//   - Clean+Abs com trim; barra final removida;
//   - vazio, só espaços, "/" (inclusive "/.." e "//") e qualquer caminho
//     dentro de frontend/public derrubam a subida (ErrProdutoImagensDir).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

func TestIMG01_Load_ProdutoImagensDir(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	casos := []struct {
		nome   string
		valor  *string
		want   string
		recusa bool
	}{
		{"ausente → padrão absoluto", nil, filepath.Join(cwd, "data", "produtos-imagens"), false},
		{"absoluto", sp("/data/produtos-imagens"), "/data/produtos-imagens", false},
		{"com espaços (trim)", sp("  /data/produtos-imagens \t"), "/data/produtos-imagens", false},
		{"barra final", sp("/data/produtos-imagens/"), "/data/produtos-imagens", false},
		{"com .. (Clean)", sp("/data/x/../produtos-imagens"), "/data/produtos-imagens", false},
		{"relativo", sp("imgs/./a/../b"), filepath.Join(cwd, "imgs", "b"), false},
		{"parecido com frontend/public, mas não é", sp("/srv/frontend/publico"), "/srv/frontend/publico", false},
		{"frontend-public", sp("/srv/frontend-public/img"), "/srv/frontend-public/img", false},
		{"public sem frontend", sp("/srv/public/img"), "/srv/public/img", false},
		{"vazio", sp(""), "", true},
		{"só espaços", sp("   \t"), "", true},
		{"raiz", sp("/"), "", true},
		{"raiz com ..", sp("/../.."), "", true},
		{"raiz dupla", sp("//"), "", true},
		{"raiz com ponto", sp("/./"), "", true},
		{"frontend/public", sp("/repo/frontend/public"), "", true},
		{"dentro de frontend/public", sp("/repo/frontend/public/produtos"), "", true},
		{"frontend/public com barra final", sp("/repo/frontend/public/"), "", true},
		{"frontend/public por ..", sp("/repo/frontend/x/../public/img"), "", true},
		{"frontend/public relativo", sp("frontend/public/img"), "", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = sp("u")
			env["DB_SENHA"] = sp("s")
			env["PRODUTO_IMAGENS_DIR"] = c.valor
			sec11Env(t, env)

			cfg, err := config.Load()
			if c.recusa {
				require.ErrorIs(t, err, config.ErrProdutoImagensDir)
				assert.Nil(t, cfg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, cfg.ProdutoImagensDir)
			assert.True(t, filepath.IsAbs(cfg.ProdutoImagensDir))
		})
	}
}

// LoadSemCredenciaisDB também valida o diretório (o comando gerarimagens
// usa o mesmo PRODUTO_IMAGENS_DIR).
func TestIMG01_LoadSemCredenciaisDB_ProdutoImagensDir(t *testing.T) {
	env := sec11Base()
	env["DB_USUARIO"] = nil
	env["DB_SENHA"] = nil
	env["PRODUTO_IMAGENS_DIR"] = sp("/")
	sec11Env(t, env)
	_, err := config.LoadSemCredenciaisDB()
	require.ErrorIs(t, err, config.ErrProdutoImagensDir)

	t.Setenv("PRODUTO_IMAGENS_DIR", "/data/img")
	cfg, err := config.LoadSemCredenciaisDB()
	require.NoError(t, err)
	assert.Equal(t, "/data/img", cfg.ProdutoImagensDir)
}

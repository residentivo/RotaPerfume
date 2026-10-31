package gerarimagens_test

// IMG-01 (Lote 26, TestBrain): comando gerarimagens (D4, spec §7).
//   - sem -confirmar nem -dry-run: código 2, nada feito (nem config, nem banco);
//   - -dry-run: lista e renderiza sem gravar (o diretório nem é criado);
//   - -confirmar: grava pelo MESMO caso de uso do upload (normalizador real,
//     nome UUID v4 .png, troca transacional), pula quem já tem imagem;
//     -sobrescrever troca e apaga a antiga;
//   - falhas de config/banco/montagem → código 1; falha num produto não
//     para o lote, mas o comando termina com erro;
//   - RenderizarPNG: PNG 600x600 válido pela validação do upload, com
//     truncamento de nomes longos.
// Banco: sqlmock. Disco: t.TempDir().

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/gerarimagens"
	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoimagem"
	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/kernel"
)

var reNomeV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\.png$`)

const antiga = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa.png"

var colunasProduto = []string{"id", "sku", "descricao", "categoria", "marca", "nota_olfativa", "preco_tabela",
	"custo_unitario", "unidade", "data_lancamento", "ativo", "imagem", "created_at", "updated_at"}

func linha(rows *sqlmock.Rows, id int64, sku, desc, imagem string) *sqlmock.Rows {
	agora := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	return rows.AddRow(id, sku, desc, "EDP", "Marca", "", 10.0, 5.0, "UN", nil, true, imagem, agora, agora)
}

type execucao struct {
	saida, erro, logs bytes.Buffer
	codigo            int
	configLido        bool
	bancoAberto       bool
}

func novoMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true), sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func cfgTeste(dir string) *config.Config {
	return &config.Config{DBName: "rotaperfumes", DBHost: "db.local", DBPort: "3306", ProdutoImagensDir: dir}
}

func rodar(t *testing.T, db *sql.DB, deps gerarimagens.Deps, cfg *config.Config, cfgErr error, args ...string) *execucao {
	t.Helper()
	ex := &execucao{}
	anterior := log.Writer()
	log.SetOutput(&ex.logs)
	t.Cleanup(func() { log.SetOutput(anterior) })
	if deps.AbrirBanco == nil {
		deps.AbrirBanco = func(string) (*sql.DB, error) { ex.bancoAberto = true; return db, nil }
	}
	if deps.MontarCatalogo == nil {
		deps.MontarCatalogo = gerarimagens.DepsPadrao().MontarCatalogo
	}
	amb := cmdutil.Ambiente{
		Saida: &ex.saida, Erro: &ex.erro,
		CarregarEnv: func() {},
		Config: func() (*config.Config, error) {
			ex.configLido = true
			return cfg, cfgErr
		},
		ConfigSemDB: func() (*config.Config, error) {
			ex.configLido = true
			return cfg, cfgErr
		},
	}
	ex.codigo = cmdutil.Executar(gerarimagens.Tag, gerarimagens.CLI(deps), args, amb)
	return ex
}

func esperarLista(m sqlmock.Sqlmock, total int, rows *sqlmock.Rows) {
	m.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(total))
	m.ExpectQuery(`FROM produtos ORDER BY id ASC LIMIT \? OFFSET \?`).WithArgs(100, 0).WillReturnRows(rows)
}

func arquivos(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	require.NoError(t, err)
	out := []string{}
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

func TestIMG01_Gerar_SemConfirmacao(t *testing.T) {
	casos := []struct {
		nome   string
		args   []string
		codigo int
		erro   string
	}{
		{"sem flags", nil, cmdutil.SaidaUso, "use -confirmar"},
		{"só -sobrescrever", []string{"-sobrescrever"}, cmdutil.SaidaUso, "use -confirmar"},
		{"-confirmar=false", []string{"-confirmar=false"}, cmdutil.SaidaUso, "use -confirmar"},
		{"flag desconhecida", []string{"-forcar"}, cmdutil.SaidaUso, "flag provided but not defined"},
		{"-h", []string{"-h"}, cmdutil.SaidaOK, "-confirmar"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "imgs")
			db, mock := novoMock(t)
			ex := rodar(t, db, gerarimagens.Deps{}, cfgTeste(dir), nil, c.args...)
			assert.Equal(t, c.codigo, ex.codigo)
			assert.Contains(t, ex.erro.String(), c.erro)
			assert.False(t, ex.configLido, "nada é feito antes da confirmação")
			assert.False(t, ex.bancoAberto)
			assert.NoDirExists(t, dir)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestIMG01_Gerar_DryRunNaoGrava(t *testing.T) {
	for _, args := range [][]string{{"-dry-run"}, {"-dry-run", "-confirmar"}, {"-dry-run", "-sobrescrever"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "imgs")
			db, mock := novoMock(t)
			mock.ExpectPing()
			rows := sqlmock.NewRows(colunasProduto)
			linha(rows, 1, "SKU-1", "Perfume Um", "")
			linha(rows, 2, "SKU-2", "Perfume Dois", antiga)
			esperarLista(mock, 2, rows)

			ex := rodar(t, db, gerarimagens.Deps{}, cfgTeste(dir), nil, args...)
			require.Equal(t, cmdutil.SaidaOK, ex.codigo, ex.logs.String())
			out := ex.saida.String()
			assert.Contains(t, out, "modo: dry-run")
			assert.Contains(t, out, "SIMULA id=1 sku=SKU-1 bytes=")
			sobrescrever := strings.Contains(strings.Join(args, " "), "-sobrescrever")
			if sobrescrever {
				assert.Contains(t, out, "SIMULA id=2")
				assert.Contains(t, out, "Total: 2 | gerados: 0 | pulados (já tinham imagem): 0 | simulados: 2 | falhas: 0")
			} else {
				assert.NotContains(t, out, "id=2")
				assert.Contains(t, out, "Total: 2 | gerados: 0 | pulados (já tinham imagem): 1 | simulados: 1 | falhas: 0")
			}
			assert.NoDirExists(t, dir, "dry-run não cria nem grava nada")
			assert.NoError(t, mock.ExpectationsWereMet(), "nenhum UPDATE")
		})
	}
}

func esperarTroca(m sqlmock.Sqlmock, id int64, anterior string) {
	var ant any
	if anterior != "" {
		ant = anterior
	}
	m.ExpectBegin()
	m.ExpectQuery(`SELECT imagem FROM produtos WHERE id = \? FOR UPDATE`).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(ant))
	m.ExpectExec(`UPDATE produtos SET imagem = \? WHERE id = \?`).WithArgs(sqlmock.AnyArg(), id).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	rows := sqlmock.NewRows(colunasProduto)
	linha(rows, id, fmt.Sprintf("SKU-%d", id), "x", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb.png")
	m.ExpectQuery(`FROM produtos WHERE id = \?`).WithArgs(id).WillReturnRows(rows)
}

func TestIMG01_Gerar_Confirmar_PulaQuemTemImagem(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "imgs")
	db, mock := novoMock(t)
	mock.ExpectPing()
	rows := sqlmock.NewRows(colunasProduto)
	linha(rows, 1, "SKU-1", "Perfume Um Eau de Parfum 100ml", "")
	linha(rows, 2, "SKU-2", "Perfume Dois", antiga)
	esperarLista(mock, 2, rows)
	esperarTroca(mock, 1, "")

	ex := rodar(t, db, gerarimagens.Deps{}, cfgTeste(dir), nil, "-confirmar")
	require.Equal(t, cmdutil.SaidaOK, ex.codigo, ex.logs.String())
	assert.Contains(t, ex.saida.String(), "modo: gravar (pula quem já tem imagem)")
	assert.Contains(t, ex.saida.String(), "OK     id=1 sku=SKU-1 arquivo=")
	assert.Contains(t, ex.saida.String(), "Total: 2 | gerados: 1 | pulados (já tinham imagem): 1 | simulados: 0 | falhas: 0")
	nomes := arquivos(t, dir)
	require.Len(t, nomes, 1)
	assert.Regexp(t, reNomeV4, nomes[0])
	dados, err := os.ReadFile(filepath.Join(dir, nomes[0]))
	require.NoError(t, err)
	cfg, err := png.DecodeConfig(bytes.NewReader(dados))
	require.NoError(t, err)
	assert.Equal(t, gerarimagens.LadoPx, cfg.Width)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIMG01_Gerar_Sobrescrever_TrocaEApagaAntiga(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "imgs")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, antiga), []byte("antiga"), 0o640))
	db, mock := novoMock(t)
	mock.ExpectPing()
	rows := sqlmock.NewRows(colunasProduto)
	linha(rows, 2, "SKU-2", "Perfume Dois", antiga)
	esperarLista(mock, 1, rows)
	esperarTroca(mock, 2, antiga)

	ex := rodar(t, db, gerarimagens.Deps{}, cfgTeste(dir), nil, "-confirmar", "-sobrescrever")
	require.Equal(t, cmdutil.SaidaOK, ex.codigo, ex.logs.String())
	assert.Contains(t, ex.saida.String(), "modo: gravar (sobrescrever)")
	nomes := arquivos(t, dir)
	require.Len(t, nomes, 1)
	assert.NotEqual(t, antiga, nomes[0])
	assert.Regexp(t, reNomeV4, nomes[0])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIMG01_Gerar_FalhaNumProdutoNaoParaOLote(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "imgs")
	db, mock := novoMock(t)
	mock.ExpectPing()
	rows := sqlmock.NewRows(colunasProduto)
	linha(rows, 1, "SKU-1", "Um", "")
	linha(rows, 2, "SKU-2", "Dois", "")
	esperarLista(mock, 2, rows)
	mock.ExpectBegin()
	mock.ExpectQuery(`FOR UPDATE`).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"imagem"})) // sumiu
	mock.ExpectRollback()
	esperarTroca(mock, 2, "")

	ex := rodar(t, db, gerarimagens.Deps{}, cfgTeste(dir), nil, "-confirmar")
	assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
	assert.Contains(t, ex.saida.String(), "FALHA  id=1 sku=SKU-1: produto não encontrado")
	assert.Contains(t, ex.saida.String(), "OK     id=2")
	assert.Contains(t, ex.saida.String(), "falhas: 1")
	assert.Contains(t, ex.logs.String(), "gerarimagens: 1 produto(s) falharam")
	assert.Len(t, arquivos(t, dir), 1, "o arquivo do produto que falhou é apagado")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIMG01_Gerar_FalhasDeInfra(t *testing.T) {
	base := t.TempDir()
	arquivo := filepath.Join(base, "arquivo")
	require.NoError(t, os.WriteFile(arquivo, []byte("x"), 0o600))
	casos := []struct {
		nome   string
		dir    string
		cfgErr error
		deps   func(db *sql.DB) gerarimagens.Deps
		mock   func(m sqlmock.Sqlmock)
		log    string
	}{
		{"config inválida", filepath.Join(base, "a"), errors.New("JWT_SECRET"), nil, nil, "falha ao carregar config"},
		{"banco não abre", filepath.Join(base, "b"), nil, func(*sql.DB) gerarimagens.Deps {
			return gerarimagens.Deps{AbrirBanco: func(string) (*sql.DB, error) { return nil, errors.New("recusado") }}
		}, nil, "gerarimagens: db: recusado"},
		{"ping falha", filepath.Join(base, "c"), nil, nil, func(m sqlmock.Sqlmock) {
			m.ExpectPing().WillReturnError(errors.New("timeout"))
		}, "gerarimagens: db ping: timeout"},
		{"diretório inutilizável (fail-fast)", filepath.Join(arquivo, "sub"), nil, nil, func(m sqlmock.Sqlmock) {
			m.ExpectPing()
		}, "gerarimagens: imagens de produto: catalogofs: criar diretório"},
		{"listar falha", filepath.Join(base, "d"), nil, nil, func(m sqlmock.Sqlmock) {
			m.ExpectPing()
			m.ExpectQuery(`SELECT COUNT`).WillReturnError(errors.New("tabela"))
		}, "gerarimagens: listar produtos"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := novoMock(t)
			if c.mock != nil {
				c.mock(mock)
			}
			var deps gerarimagens.Deps
			if c.deps != nil {
				deps = c.deps(db)
			}
			ex := rodar(t, db, deps, cfgTeste(c.dir), c.cfgErr, "-confirmar")
			assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
			assert.Contains(t, ex.logs.String(), c.log)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---- Gerar com Catalogo falso: paginação, totais e propagação de erros ----

type catalogoFalso struct {
	produtos  []catalogo.Produto
	errListar error
	errDef    map[int64]error
	paginas   []int
	definidos []int64
	dados     map[int64][]byte
}

func (c *catalogoFalso) ListarProdutos(_ context.Context, f catalogo.FiltroProduto, p kernel.Pagina) (kernel.Resultado[catalogo.Produto], error) {
	if c.errListar != nil {
		return kernel.Resultado[catalogo.Produto]{}, c.errListar
	}
	c.paginas = append(c.paginas, p.Numero())
	ini := (p.Numero() - 1) * p.Limite()
	fim := min(ini+p.Limite(), len(c.produtos))
	if ini > len(c.produtos) {
		ini = len(c.produtos)
	}
	return kernel.NovoResultado(c.produtos[ini:fim], p, len(c.produtos)), nil
}

func (c *catalogoFalso) DefinirImagemProduto(_ context.Context, id int64, dados []byte) (*catalogo.Produto, error) {
	if err := c.errDef[id]; err != nil {
		return nil, err
	}
	c.definidos = append(c.definidos, id)
	if c.dados == nil {
		c.dados = map[int64][]byte{}
	}
	c.dados[id] = dados
	return &catalogo.Produto{ID: id, Imagem: "cccccccc-cccc-4ccc-8ccc-cccccccccccc.png"}, nil
}

func produtos(n int, comImagem func(i int) bool) []catalogo.Produto {
	out := make([]catalogo.Produto, n)
	for i := range out {
		out[i] = catalogo.Produto{ID: int64(i + 1)}
		out[i].Descricao = fmt.Sprintf("Produto %d", i+1)
		if comImagem(i) {
			out[i].Imagem = antiga
		}
	}
	return out
}

func TestIMG01_Gerar_Paginacao(t *testing.T) {
	casos := []struct {
		nome      string
		n         int
		comImagem func(int) bool
		op        gerarimagens.Opcoes
		paginas   []int
		want      gerarimagens.Totais
		definidos int
	}{
		{"vazio", 0, func(int) bool { return false }, gerarimagens.Opcoes{}, []int{1}, gerarimagens.Totais{}, 0},
		{"uma página cheia, todos com imagem", 100, func(int) bool { return true }, gerarimagens.Opcoes{}, []int{1}, gerarimagens.Totais{Lidos: 100, Pulados: 100}, 0},
		{"250 produtos, 5 sem imagem", 250, func(i int) bool { return i%50 != 0 }, gerarimagens.Opcoes{}, []int{1, 2, 3},
			gerarimagens.Totais{Lidos: 250, Gerados: 5, Pulados: 245}, 5},
		{"sobrescrevendo", 3, func(int) bool { return true }, gerarimagens.Opcoes{Sobrescrever: true}, []int{1},
			gerarimagens.Totais{Lidos: 3, Gerados: 3}, 3},
		{"dry-run em 2 páginas", 101, func(i int) bool { return i != 100 }, gerarimagens.Opcoes{DryRun: true}, []int{1, 2},
			gerarimagens.Totais{Lidos: 101, Pulados: 100, Simulados: 1}, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			cat := &catalogoFalso{produtos: produtos(c.n, c.comImagem)}
			var out bytes.Buffer
			tot, err := gerarimagens.Gerar(context.Background(), cat, c.op, &out)
			require.NoError(t, err)
			assert.Equal(t, c.want, tot)
			assert.Equal(t, c.paginas, cat.paginas)
			assert.Len(t, cat.definidos, c.definidos)
		})
	}
}

// As imagens geradas passam pela mesma validação do upload.
func TestIMG01_Gerar_ImagemPassaNaValidacao(t *testing.T) {
	cat := &catalogoFalso{produtos: produtos(3, func(int) bool { return false })}
	_, err := gerarimagens.Gerar(context.Background(), cat, gerarimagens.Opcoes{}, &bytes.Buffer{})
	require.NoError(t, err)
	for id, d := range cat.dados {
		norm, err := catalogoimagem.Novo().Normalizar(d)
		require.NoError(t, err, "id=%d", id)
		assert.Equal(t, ".png", norm.Extensao)
	}
}

func TestIMG01_Gerar_Erros(t *testing.T) {
	cat := &catalogoFalso{errListar: errors.New("banco")}
	_, err := gerarimagens.Gerar(context.Background(), cat, gerarimagens.Opcoes{}, &bytes.Buffer{})
	require.ErrorContains(t, err, "listar produtos: banco")

	cat = &catalogoFalso{produtos: produtos(3, func(int) bool { return false }), errDef: map[int64]error{2: catalogo.ErrImagemInvalida}}
	var out bytes.Buffer
	tot, err := gerarimagens.Gerar(context.Background(), cat, gerarimagens.Opcoes{}, &out)
	require.EqualError(t, err, "1 produto(s) falharam")
	assert.Equal(t, gerarimagens.Totais{Lidos: 3, Gerados: 2, Falhas: 1}, tot)
	assert.Equal(t, []int64{1, 3}, cat.definidos)
	assert.Contains(t, out.String(), "FALHA  id=2")
}

func TestIMG01_ImprimirTotais(t *testing.T) {
	var b bytes.Buffer
	gerarimagens.ImprimirTotais(&b, gerarimagens.Totais{Lidos: 5, Gerados: 1, Pulados: 2, Simulados: 0, Falhas: 2})
	assert.Equal(t, "Total: 5 | gerados: 1 | pulados (já tinham imagem): 2 | simulados: 0 | falhas: 2\n", b.String())
}

func TestIMG01_RenderizarPNG(t *testing.T) {
	casos := []struct{ nome, desc, sku string }{
		{"curto", "Perfume", "SKU-1"},
		{"normal", "Perfume Floral Eau de Parfum 100ml", "SKU-2"},
		{"muito longo (trunca)", strings.Repeat("Palavra Comprida ", 40), "SKU-3"},
		{"palavra única enorme", strings.Repeat("A", 300), "SKU-4"},
		{"unicode", "Água de Colônia Lavanda & Âmbar — edição", "SKU-5"},
		{"espaços extras", "   Perfume    com\tespaços \n ", "SKU-6"},
		{"sem descrição usa o SKU", "", "SKU-7"},
		{"sem descrição nem SKU", "", ""},
		{"só SKU com espaços", "", "   "},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, err := gerarimagens.RenderizarPNG(c.desc, c.sku)
			require.NoError(t, err)
			img, err := png.Decode(bytes.NewReader(d))
			require.NoError(t, err)
			assert.Equal(t, gerarimagens.LadoPx, img.Bounds().Dx())
			assert.Equal(t, gerarimagens.LadoPx, img.Bounds().Dy())
			assert.LessOrEqual(t, len(d), catalogo.MaxImagemBytes)
			_, err = catalogoimagem.Novo().Normalizar(d)
			require.NoError(t, err)
		})
	}
}

func TestIMG01_RenderizarPNG_DeterministicoECorPeloSKU(t *testing.T) {
	a1, err := gerarimagens.RenderizarPNG("Perfume", "SKU-A")
	require.NoError(t, err)
	a2, err := gerarimagens.RenderizarPNG("Perfume", "SKU-A")
	require.NoError(t, err)
	b, err := gerarimagens.RenderizarPNG("Perfume", "SKU-B")
	require.NoError(t, err)
	assert.Equal(t, a1, a2)
	assert.NotEqual(t, a1, b)

	ia, _ := png.Decode(bytes.NewReader(a1))
	ib, _ := png.Decode(bytes.NewReader(b))
	assert.NotEqual(t, ia.At(0, 0), ib.At(0, 0), "a cor de fundo deriva do SKU")
}

// Cobre as 6 faixas de matiz do HSL: SKUs diferentes caem em matizes
// diferentes; todos renderizam.
func TestIMG01_RenderizarPNG_VariosSKUs(t *testing.T) {
	cores := map[string]bool{}
	for i := range 24 {
		d, err := gerarimagens.RenderizarPNG("P", fmt.Sprintf("SKU-%03d", i))
		require.NoError(t, err)
		img, err := png.Decode(bytes.NewReader(d))
		require.NoError(t, err)
		r, g, b, _ := img.At(0, 0).RGBA()
		cores[fmt.Sprint(r>>8, g>>8, b>>8)] = true
	}
	assert.Greater(t, len(cores), 12)
}

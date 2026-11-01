package catalogo_test

// IMG-01 (Lote 26, TestBrain): casos de uso de imagem do produto
// (catalogoapp/imagens.go) com fakes de repositório, transação,
// armazenamento e normalizador (spec §4):
//   - troca: normaliza → grava → tx (FOR UPDATE, UPDATE, commit) → só então
//     apaga a antiga, com ctx sem cancelamento;
//   - produto inexistente, falha no UPDATE ou no commit apagam o arquivo novo;
//   - remoção zera e apaga; sem imagem é idempotente (sem UPDATE);
//   - nome adulterado no banco (../x) nunca chega ao Remover;
//   - montado sem imagens → ErrImagensIndisponiveis (500) / 404 na leitura;
//   - concorrência no mesmo produto com o adapter real (catalogofs em
//     t.TempDir()): sobra exatamente o arquivo que ficou no banco.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
	"github.com/rotaperfumes/shared/catalogo/catalogofs"
	"github.com/rotaperfumes/shared/kernel"
)

const (
	img01Antiga = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa.png"
	img01Outra  = "bbbbbbbb-bbbb-4bbb-9bbb-bbbbbbbbbbbb.jpg"
)

var errImgInfra = errors.New("falha simulada")

// img01Banco é o "banco" compartilhado entre o repositório da tx e o de
// leitura (PorID). O mutex da tx imita o FOR UPDATE.
type img01Banco struct {
	mu      sync.Mutex // estado
	lock    sync.Mutex // FOR UPDATE
	imagens map[int64]string
	eventos []string

	errLer, errDefinir, errCommit, errPorID error
	aoCommit                                func()
	definicoes                              int
}

func novoImg01Banco(imagens map[int64]string) *img01Banco {
	return &img01Banco{imagens: imagens}
}

func (b *img01Banco) evento(e string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.eventos = append(b.eventos, e)
}

func (b *img01Banco) Eventos() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.eventos...)
}

// txRepo é o repositório ligado à tx: escreve num rascunho aplicado no commit.
type txRepo struct {
	b        *img01Banco
	rascunho map[int64]*string
}

func (r *txRepo) ImagemParaAtualizar(_ context.Context, id int64) (string, error) {
	r.b.evento("select-for-update")
	if r.b.errLer != nil {
		return "", r.b.errLer
	}
	r.b.mu.Lock()
	defer r.b.mu.Unlock()
	img, ok := r.b.imagens[id]
	if !ok {
		return "", catalogo.ErrProdutoNaoEncontrado
	}
	return img, nil
}

func (r *txRepo) DefinirImagem(_ context.Context, id int64, nome string) error {
	r.b.evento("update:" + nome)
	r.b.mu.Lock()
	r.b.definicoes++
	r.b.mu.Unlock()
	if r.b.errDefinir != nil {
		return r.b.errDefinir
	}
	n := nome
	r.rascunho[id] = &n
	return nil
}

// WithTx satisfaz catalogoapp.Transactor[catalogoapp.TxImagemProduto].
func (b *img01Banco) WithTx(_ context.Context, fn func(catalogoapp.TxImagemProduto) error) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	r := &txRepo{b: b, rascunho: map[int64]*string{}}
	if err := fn(catalogoapp.TxImagemProduto{Produtos: r}); err != nil {
		b.evento("rollback")
		return err
	}
	if b.errCommit != nil {
		b.evento("rollback")
		return b.errCommit
	}
	b.mu.Lock()
	for id, n := range r.rascunho {
		b.imagens[id] = *n
	}
	b.mu.Unlock()
	b.evento("commit")
	if b.aoCommit != nil {
		b.aoCommit()
	}
	return nil
}

// img01Produtos é a porta Produtos: só PorID importa aqui.
type img01Produtos struct{ b *img01Banco }

func (p img01Produtos) Listar(context.Context, catalogo.FiltroProduto, kernel.Pagina) (kernel.Resultado[catalogo.Produto], error) {
	return kernel.Resultado[catalogo.Produto]{}, nil
}

func (p img01Produtos) PorID(_ context.Context, id int64) (*catalogo.Produto, error) {
	if p.b.errPorID != nil {
		return nil, p.b.errPorID
	}
	p.b.mu.Lock()
	defer p.b.mu.Unlock()
	img, ok := p.b.imagens[id]
	if !ok {
		return nil, catalogo.ErrProdutoNaoEncontrado
	}
	return &catalogo.Produto{ID: id, Imagem: img, Ativo: true}, nil
}
func (img01Produtos) ExisteSKU(context.Context, catalogo.SKU) (bool, error)          { return false, nil }
func (img01Produtos) Inserir(context.Context, *catalogo.Produto) error               { return nil }
func (img01Produtos) Atualizar(context.Context, int64, catalogo.CamposProduto) error { return nil }
func (img01Produtos) DefinirAtivo(context.Context, int64, bool) error                { return nil }
func (img01Produtos) AlternarAtivo(context.Context, int64) error                     { return nil }

// img01Armazenamento é o ArmazenamentoImagens em memória.
type img01Armazenamento struct {
	b         *img01Banco
	mu        sync.Mutex
	arquivos  map[string][]byte
	seq       int
	errGravar error
	errRemov  error
	ctxRemov  []error
	removidos []string
}

func novoImg01Arm(b *img01Banco, existentes ...string) *img01Armazenamento {
	a := &img01Armazenamento{b: b, arquivos: map[string][]byte{}}
	for _, n := range existentes {
		a.arquivos[n] = []byte("antigo")
	}
	return a
}

func (a *img01Armazenamento) Gravar(_ context.Context, ext string, dados []byte) (string, error) {
	if a.errGravar != nil {
		return "", a.errGravar
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.seq++
	nome := strings.Replace("cccccccc-cccc-4ccc-8ccc-00000000000X", "X", string(rune('0'+a.seq%10)), 1) + ext
	a.arquivos[nome] = dados
	a.b.evento("gravar:" + nome)
	return nome, nil
}

func (a *img01Armazenamento) Remover(ctx context.Context, nome string) error {
	a.b.evento("remover:" + nome)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ctxRemov = append(a.ctxRemov, ctx.Err())
	a.removidos = append(a.removidos, nome)
	if a.errRemov != nil {
		return a.errRemov
	}
	delete(a.arquivos, nome)
	return nil
}

func (a *img01Armazenamento) Abrir(nome string) (io.ReadSeekCloser, time.Time, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.arquivos[nome]
	if !ok {
		return nil, time.Time{}, catalogo.ErrImagemNaoEncontrada
	}
	return nopCloser{bytes.NewReader(d)}, time.Unix(1700000000, 0), nil
}

type nopCloser struct{ *bytes.Reader }

func (nopCloser) Close() error { return nil }

type img01Normalizador struct {
	err      error
	chamadas atomic.Int32
}

func (n *img01Normalizador) Normalizar(d []byte) (catalogo.ImagemNormalizada, error) {
	n.chamadas.Add(1)
	if n.err != nil {
		return catalogo.ImagemNormalizada{}, n.err
	}
	return catalogo.ImagemNormalizada{Tipo: catalogo.TipoImagemPNG, Extensao: ".png", Dados: append([]byte("norm:"), d...)}, nil
}

type img01Fixture struct {
	banco *img01Banco
	arm   *img01Armazenamento
	norm  *img01Normalizador
	casos *catalogoapp.Casos
	logs  *bytes.Buffer
}

func novoImg01(t *testing.T, imagens map[int64]string, existentes ...string) *img01Fixture {
	t.Helper()
	b := novoImg01Banco(imagens)
	f := &img01Fixture{banco: b, arm: novoImg01Arm(b, existentes...), norm: &img01Normalizador{}, logs: &bytes.Buffer{}}
	f.casos = catalogoapp.NovoComImagens(img01Produtos{b}, &fakeEstoques{}, catalogoapp.Imagens{
		Armazenamento: f.arm, Normalizador: f.norm, Tx: b,
	})
	anterior := log.Writer()
	log.SetOutput(f.logs)
	t.Cleanup(func() { log.SetOutput(anterior) })
	return f
}

func TestIMG01_NovoComImagens_ExigeDependencias(t *testing.T) {
	b := novoImg01Banco(nil)
	arm, norm := novoImg01Arm(b), &img01Normalizador{}
	casos := map[string]catalogoapp.Imagens{
		"sem armazenamento": {Normalizador: norm, Tx: b},
		"sem normalizador":  {Armazenamento: arm, Tx: b},
		"sem transactor":    {Armazenamento: arm, Normalizador: norm},
		"vazio":             {},
	}
	for nome, img := range casos {
		t.Run(nome, func(t *testing.T) {
			assert.Panics(t, func() { catalogoapp.NovoComImagens(img01Produtos{b}, &fakeEstoques{}, img) })
		})
	}
}

func TestIMG01_CasosDesligados(t *testing.T) {
	c := catalogoapp.Novo(img01Produtos{novoImg01Banco(map[int64]string{1: ""})}, &fakeEstoques{})
	_, err := c.DefinirImagemProduto(context.Background(), 1, []byte("x"))
	assert.ErrorIs(t, err, catalogo.ErrImagensIndisponiveis)
	assert.Equal(t, kernel.KindInterno, img01Kind(err))
	_, err = c.RemoverImagemProduto(context.Background(), 1)
	assert.ErrorIs(t, err, catalogo.ErrImagensIndisponiveis)
	f, _, err := c.AbrirImagemProduto(img01Antiga)
	assert.ErrorIs(t, err, catalogo.ErrImagemNaoEncontrada)
	assert.Nil(t, f)
}

func TestIMG01_DefinirImagem_SemImagemAnterior(t *testing.T) {
	f := novoImg01(t, map[int64]string{7: ""})
	p, err := f.casos.DefinirImagemProduto(context.Background(), 7, []byte("png"))
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, int64(7), p.ID)
	assert.True(t, catalogo.NomeImagemValido(p.Imagem))
	assert.Equal(t, p.Imagem, f.banco.imagens[7])
	assert.Equal(t, []byte("norm:png"), f.arm.arquivos[p.Imagem], "grava os bytes NORMALIZADOS")
	assert.Empty(t, f.arm.removidos)
	assert.Equal(t, []string{"gravar:" + p.Imagem, "select-for-update", "update:" + p.Imagem, "commit"}, f.banco.Eventos())
	assert.Contains(t, f.logs.String(), "[produtos] imagem gravada: id=7 arquivo="+p.Imagem+" tipo=image/png bytes=8")
}

func TestIMG01_DefinirImagem_TrocaApagaAntigaDepoisDoCommit(t *testing.T) {
	f := novoImg01(t, map[int64]string{7: img01Antiga}, img01Antiga)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.banco.aoCommit = cancel // a requisição cai logo depois do commit
	p, err := f.casos.DefinirImagemProduto(ctx, 7, []byte("png"))
	require.NoError(t, err)
	ev := f.banco.Eventos()
	assert.Equal(t, []string{"gravar:" + p.Imagem, "select-for-update", "update:" + p.Imagem, "commit", "remover:" + img01Antiga}, ev)
	assert.NotContains(t, f.arm.arquivos, img01Antiga)
	assert.Contains(t, f.arm.arquivos, p.Imagem)
	assert.Equal(t, []error{nil}, f.arm.ctxRemov, "a remoção pós-commit usa ctx sem cancelamento")
}

func TestIMG01_DefinirImagem_FalhasApagamONovo(t *testing.T) {
	casos := []struct {
		nome     string
		imagens  map[int64]string
		prep     func(f *img01Fixture)
		id       int64
		errIs    error
		kind     kernel.Kind
		removido bool
		antigaOK bool
	}{
		{"produto inexistente → 404", map[int64]string{}, nil, 99, catalogo.ErrProdutoNaoEncontrado, kernel.KindNaoEncontrado, true, false},
		{"falha no SELECT FOR UPDATE", map[int64]string{7: img01Antiga}, func(f *img01Fixture) { f.banco.errLer = errImgInfra }, 7, errImgInfra, kernel.KindInterno, true, true},
		{"falha no UPDATE", map[int64]string{7: img01Antiga}, func(f *img01Fixture) { f.banco.errDefinir = errImgInfra }, 7, errImgInfra, kernel.KindInterno, true, true},
		{"UPDATE sem linha (produto apagado)", map[int64]string{7: img01Antiga}, func(f *img01Fixture) { f.banco.errDefinir = catalogo.ErrProdutoNaoEncontrado }, 7, catalogo.ErrProdutoNaoEncontrado, kernel.KindNaoEncontrado, true, true},
		{"falha no commit", map[int64]string{7: img01Antiga}, func(f *img01Fixture) { f.banco.errCommit = errImgInfra }, 7, errImgInfra, kernel.KindInterno, true, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			f := novoImg01(t, c.imagens, img01Antiga)
			if c.prep != nil {
				c.prep(f)
			}
			p, err := f.casos.DefinirImagemProduto(context.Background(), c.id, []byte("png"))
			require.ErrorIs(t, err, c.errIs)
			assert.Equal(t, c.kind, img01Kind(err))
			assert.Nil(t, p)
			require.Len(t, f.arm.removidos, 1, "o arquivo novo tem de ser apagado")
			assert.NotEqual(t, img01Antiga, f.arm.removidos[0], "a antiga nunca é apagada se a troca falhou")
			assert.NotContains(t, f.arm.arquivos, f.arm.removidos[0])
			assert.Contains(t, f.arm.arquivos, img01Antiga)
			if c.antigaOK {
				assert.Equal(t, img01Antiga, f.banco.imagens[c.id], "o banco continua com a antiga")
			}
		})
	}
}

func TestIMG01_DefinirImagem_FalhasAntesDaTx(t *testing.T) {
	casos := []struct {
		nome  string
		prep  func(f *img01Fixture)
		errIs error
	}{
		{"conteúdo inválido", func(f *img01Fixture) { f.norm.err = catalogo.ErrImagemInvalida }, catalogo.ErrImagemInvalida},
		{"conteúdo grande", func(f *img01Fixture) { f.norm.err = catalogo.ErrImagemMuitoGrande }, catalogo.ErrImagemMuitoGrande},
		{"falha ao gravar", func(f *img01Fixture) { f.arm.errGravar = errImgInfra }, errImgInfra},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			f := novoImg01(t, map[int64]string{7: img01Antiga}, img01Antiga)
			c.prep(f)
			p, err := f.casos.DefinirImagemProduto(context.Background(), 7, []byte("x"))
			require.ErrorIs(t, err, c.errIs)
			assert.Nil(t, p)
			assert.NotContains(t, f.banco.Eventos(), "select-for-update", "nenhuma tx sem arquivo gravado")
			assert.Equal(t, int32(1), f.norm.chamadas.Load())
			assert.Empty(t, f.arm.removidos)
			assert.Equal(t, img01Antiga, f.banco.imagens[7])
		})
	}
}

func TestIMG01_DefinirImagem_AntigaAdulteradaNaoChegaAoDisco(t *testing.T) {
	for _, antiga := range []string{"../x", "../../etc/passwd", ".tmp-x", strings.ToUpper(img01Antiga)} {
		t.Run(antiga, func(t *testing.T) {
			f := novoImg01(t, map[int64]string{7: antiga})
			p, err := f.casos.DefinirImagemProduto(context.Background(), 7, []byte("png"))
			require.NoError(t, err)
			assert.Empty(t, f.arm.removidos, "nome fora do padrão não toca o disco")
			assert.Equal(t, p.Imagem, f.banco.imagens[7])
			assert.Contains(t, f.logs.String(), "[produtos] nome de imagem inválido id=7")
			assert.NotContains(t, f.logs.String(), antiga)
		})
	}
}

func TestIMG01_DefinirImagem_FalhaAoApagarAntigaNaoFalhaOCaso(t *testing.T) {
	f := novoImg01(t, map[int64]string{7: img01Antiga}, img01Antiga)
	f.arm.errRemov = errImgInfra
	p, err := f.casos.DefinirImagemProduto(context.Background(), 7, []byte("png"))
	require.NoError(t, err)
	assert.Equal(t, p.Imagem, f.banco.imagens[7])
	assert.Contains(t, f.logs.String(), "[produtos] falha ao remover arquivo de imagem id=7 arquivo="+img01Antiga)
}

func TestIMG01_DefinirImagem_ReleituraFalha(t *testing.T) {
	f := novoImg01(t, map[int64]string{7: ""})
	f.banco.errPorID = errImgInfra
	p, err := f.casos.DefinirImagemProduto(context.Background(), 7, []byte("png"))
	require.Error(t, err)
	assert.Nil(t, p)
	assert.Equal(t, kernel.KindInterno, img01Kind(err))
	assert.NotEmpty(t, f.banco.imagens[7], "o vínculo já foi gravado; o arquivo fica")
	assert.Empty(t, f.arm.removidos)
}

func TestIMG01_RemoverImagem(t *testing.T) {
	casos := []struct {
		nome        string
		imagens     map[int64]string
		prep        func(f *img01Fixture)
		id          int64
		errIs       error
		removidos   []string
		definicoes  int
		imagemFinal string
	}{
		{"com imagem: zera e apaga", map[int64]string{7: img01Antiga}, nil, 7, nil, []string{img01Antiga}, 1, ""},
		{"sem imagem: idempotente, sem UPDATE", map[int64]string{7: ""}, nil, 7, nil, nil, 0, ""},
		{"inexistente: 404", map[int64]string{}, nil, 9, catalogo.ErrProdutoNaoEncontrado, nil, 0, ""},
		{"falha no UPDATE: não apaga", map[int64]string{7: img01Antiga}, func(f *img01Fixture) { f.banco.errDefinir = errImgInfra }, 7, errImgInfra, nil, 1, img01Antiga},
		{"falha no commit: não apaga", map[int64]string{7: img01Antiga}, func(f *img01Fixture) { f.banco.errCommit = errImgInfra }, 7, errImgInfra, nil, 1, img01Antiga},
		{"nome adulterado no banco: zera sem tocar o disco", map[int64]string{7: "../x"}, nil, 7, nil, nil, 1, ""},
		{"falha ao apagar o arquivo: sucesso assim mesmo", map[int64]string{7: img01Antiga}, func(f *img01Fixture) { f.arm.errRemov = errImgInfra }, 7, nil, []string{img01Antiga}, 1, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			f := novoImg01(t, c.imagens, img01Antiga)
			if c.prep != nil {
				c.prep(f)
			}
			p, err := f.casos.RemoverImagemProduto(context.Background(), c.id)
			assert.Equal(t, c.removidos, f.arm.removidos)
			assert.Equal(t, c.definicoes, f.banco.definicoes)
			if c.errIs != nil {
				require.ErrorIs(t, err, c.errIs)
				assert.Nil(t, p)
				if img, ok := c.imagens[c.id]; ok {
					assert.Equal(t, img, f.banco.imagens[c.id])
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.imagemFinal, p.Imagem)
			assert.Equal(t, c.imagemFinal, f.banco.imagens[c.id])
		})
	}
}

func TestIMG01_RemoverImagem_ApagaSoDepoisDoCommit(t *testing.T) {
	f := novoImg01(t, map[int64]string{7: img01Antiga}, img01Antiga)
	_, err := f.casos.RemoverImagemProduto(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, []string{"select-for-update", "update:", "commit", "remover:" + img01Antiga}, f.banco.Eventos())
	assert.Contains(t, f.logs.String(), "[produtos] imagem removida: id=7 arquivo="+img01Antiga)
	// segunda remoção: idempotente
	p, err := f.casos.RemoverImagemProduto(context.Background(), 7)
	require.NoError(t, err)
	assert.Empty(t, p.Imagem)
	assert.Len(t, f.arm.removidos, 1)
}

func TestIMG01_RemoverImagem_ReleituraFalha(t *testing.T) {
	f := novoImg01(t, map[int64]string{7: img01Antiga}, img01Antiga)
	f.banco.errPorID = errImgInfra
	_, err := f.casos.RemoverImagemProduto(context.Background(), 7)
	require.Error(t, err)
	assert.Equal(t, kernel.KindInterno, img01Kind(err))
}

func TestIMG01_AbrirImagem(t *testing.T) {
	f := novoImg01(t, map[int64]string{}, img01Antiga)
	r, mod, err := f.casos.AbrirImagemProduto(img01Antiga)
	require.NoError(t, err)
	got, _ := io.ReadAll(r)
	assert.Equal(t, "antigo", string(got))
	assert.Equal(t, time.Unix(1700000000, 0), mod)

	for _, nome := range []string{"", "../x", img01Outra, ".tmp-" + img01Antiga, strings.ToUpper(img01Antiga)} {
		t.Run(nome, func(t *testing.T) {
			r, mod, err := f.casos.AbrirImagemProduto(nome)
			require.ErrorIs(t, err, catalogo.ErrImagemNaoEncontrada)
			assert.Nil(t, r)
			assert.True(t, mod.IsZero())
		})
	}
}

// Concorrência com o adapter real: N trocas simultâneas do mesmo produto
// (serializadas pelo "FOR UPDATE" do fake). No fim sobra exatamente um
// arquivo no diretório, e é o que está no banco; nenhum .tmp- fica.
func TestIMG01_DefinirImagem_ConcorrenciaComDiscoReal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "imgs")
	arm, err := catalogofs.Novo(dir)
	require.NoError(t, err)
	defer arm.Close()
	b := novoImg01Banco(map[int64]string{7: ""})
	casos := catalogoapp.NovoComImagens(img01Produtos{b}, &fakeEstoques{}, catalogoapp.Imagens{
		Armazenamento: arm, Normalizador: &img01Normalizador{}, Tx: b,
	})
	anterior := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(anterior)

	const n = 20
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := casos.DefinirImagemProduto(context.Background(), 7, []byte{byte(i)})
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	ents, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, ents, 1, "só a imagem vigente pode sobrar")
	assert.Equal(t, b.imagens[7], ents[0].Name())

	_, err = casos.RemoverImagemProduto(context.Background(), 7)
	require.NoError(t, err)
	ents, _ = os.ReadDir(dir)
	assert.Empty(t, ents)
	assert.Empty(t, b.imagens[7])
}

// img01Kind é o Kind que o mapper HTTP enxerga (erro sem *kernel.Erro na
// cadeia vira 500).
func img01Kind(err error) kernel.Kind {
	var e *kernel.Erro
	if errors.As(err, &e) {
		return e.Kind()
	}
	return kernel.KindInterno
}

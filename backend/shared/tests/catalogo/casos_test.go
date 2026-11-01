package catalogo_test

// ARQ-02: casos de uso do Catálogo com repositórios fake em memória (as
// portas são interfaces do consumidor). Cobrem as decisões que o sqlmock não
// isola bem: falha de infra em cada passo, releitura (BUG-09) e montagem.

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
	"github.com/rotaperfumes/shared/kernel"
)

var errInfra = errors.New("banco fora")

// fakeProdutos: cada campo err* força a falha do método correspondente.
type fakeProdutos struct {
	porID                                     map[int64]catalogo.Produto
	existe                                    bool
	errListar, errPorID, errExiste, errInsert error
	errAtualizar, errAtivo                    error
	inseridos                                 []catalogo.Produto
}

func (f *fakeProdutos) Listar(context.Context, catalogo.FiltroProduto, kernel.Pagina) (kernel.Resultado[catalogo.Produto], error) {
	return kernel.Resultado[catalogo.Produto]{}, f.errListar
}

func (f *fakeProdutos) PorID(_ context.Context, id int64) (*catalogo.Produto, error) {
	if f.errPorID != nil {
		return nil, f.errPorID
	}
	p, ok := f.porID[id]
	if !ok {
		return nil, catalogo.ErrProdutoNaoEncontrado
	}
	return &p, nil
}

func (f *fakeProdutos) ExisteSKU(context.Context, catalogo.SKU) (bool, error) {
	return f.existe, f.errExiste
}

func (f *fakeProdutos) Inserir(_ context.Context, p *catalogo.Produto) error {
	if f.errInsert != nil {
		return f.errInsert
	}
	p.ID = 42
	f.inseridos = append(f.inseridos, *p)
	return nil
}

func (f *fakeProdutos) Atualizar(context.Context, int64, catalogo.CamposProduto) error {
	return f.errAtualizar
}

func (f *fakeProdutos) DefinirAtivo(_ context.Context, id int64, ativo bool) error {
	return f.gravarAtivo(id, func(bool) bool { return ativo })
}

// AlternarAtivo (BUG-17) inverte o status gravado, como o UPDATE … NOT ativo.
func (f *fakeProdutos) AlternarAtivo(_ context.Context, id int64) error {
	return f.gravarAtivo(id, func(atual bool) bool { return !atual })
}

func (f *fakeProdutos) gravarAtivo(id int64, novo func(bool) bool) error {
	if f.errAtivo != nil {
		return f.errAtivo
	}
	p, ok := f.porID[id]
	if !ok {
		return catalogo.ErrProdutoNaoEncontrado
	}
	p.Ativo = novo(p.Ativo)
	f.porID[id] = p
	return nil
}

type fakeEstoques struct {
	porID                                 map[int64]catalogo.RegistroEstoque
	errPorID, errInsert, errSaldo, errLer error
	historico, ultima                     int
}

func (f *fakeEstoques) Historico(context.Context, catalogo.FiltroEstoque, kernel.Pagina) (kernel.Resultado[catalogo.RegistroEstoque], error) {
	f.historico++
	return kernel.Resultado[catalogo.RegistroEstoque]{}, f.errLer
}

func (f *fakeEstoques) UltimaPosicao(context.Context, catalogo.FiltroEstoque, kernel.Pagina) (kernel.Resultado[catalogo.RegistroEstoque], error) {
	f.ultima++
	return kernel.Resultado[catalogo.RegistroEstoque]{}, f.errLer
}

func (f *fakeEstoques) PorID(_ context.Context, id int64) (*catalogo.RegistroEstoque, error) {
	if f.errPorID != nil {
		return nil, f.errPorID
	}
	e, ok := f.porID[id]
	if !ok {
		return nil, catalogo.ErrEstoqueNaoEncontrado
	}
	return &e, nil
}

func (f *fakeEstoques) Inserir(_ context.Context, e *catalogo.RegistroEstoque) error {
	if f.errInsert == nil {
		e.ID = 7
	}
	return f.errInsert
}

func (f *fakeEstoques) AtualizarSaldo(context.Context, int64, int, bool) error { return f.errSaldo }

func TestCatalogoapp_NovoExigeRepositorios(t *testing.T) {
	assert.Panics(t, func() { catalogoapp.Novo(nil, &fakeEstoques{}) })
	assert.Panics(t, func() { catalogoapp.Novo(&fakeProdutos{}, nil) })
	assert.NotPanics(t, func() { catalogoapp.Novo(&fakeProdutos{}, &fakeEstoques{}) })
}

func TestCatalogoapp_ListarEstoque_EscolheConsulta(t *testing.T) {
	casos := []struct {
		nome              string
		q                 catalogoapp.ConsultaEstoque
		historico, ultima int
		err               error
	}{
		{"padrão: última posição", catalogoapp.ConsultaEstoque{}, 0, 1, nil},
		{"historico=true: série completa", catalogoapp.ConsultaEstoque{Historico: true}, 1, 0, nil},
		{"data inválida não consulta", catalogoapp.ConsultaEstoque{DataDe: "x", Historico: true}, 0, 0, catalogo.ErrEstoqueDataInvalida},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			est := &fakeEstoques{}
			_, err := catalogoapp.Novo(&fakeProdutos{}, est).ListarEstoque(context.Background(), c.q, kernel.NovaPagina(1, 20))
			assert.ErrorIs(t, err, c.err)
			assert.Equal(t, c.historico, est.historico)
			assert.Equal(t, c.ultima, est.ultima)
		})
	}
}

func TestCatalogoapp_AjustarEstoque_Erros(t *testing.T) {
	dados := catalogo.DadosEstoque{SKU: "S", DataSnapshot: "2024-01-01", Saldo: 1}
	casos := []struct {
		nome string
		prod *fakeProdutos
		est  *fakeEstoques
		err  error
	}{
		{"erro ao conferir sku é propagado", &fakeProdutos{errExiste: errInfra}, &fakeEstoques{}, errInfra},
		{"sku inexistente", &fakeProdutos{existe: false}, &fakeEstoques{}, catalogo.ErrEstoqueSKUInexistente},
		{"erro no insert é propagado", &fakeProdutos{existe: true}, &fakeEstoques{errInsert: errInfra}, errInfra},
		{"validação antes do banco", &fakeProdutos{errExiste: errInfra}, &fakeEstoques{}, catalogo.ErrSKUObrigatorio},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d := dados
			if c.err == catalogo.ErrSKUObrigatorio {
				d.SKU = ""
			}
			e, err := catalogoapp.Novo(c.prod, c.est).AjustarEstoque(context.Background(), d)
			assert.Nil(t, e)
			assert.ErrorIs(t, err, c.err)
		})
	}
}

// TestCatalogoapp_AjustarEstoque_ReleituraFalha: o INSERT confirmado não
// vira erro; volta o objeto em memória (BUG-09).
func TestCatalogoapp_AjustarEstoque_ReleituraFalha(t *testing.T) {
	est := &fakeEstoques{errPorID: errInfra}
	e, err := catalogoapp.Novo(&fakeProdutos{existe: true}, est).
		AjustarEstoque(context.Background(), catalogo.DadosEstoque{SKU: "S", DataSnapshot: "2024-01-01"})
	require.NoError(t, err)
	assert.Equal(t, int64(7), e.ID)
	assert.True(t, e.Ruptura)
	assert.True(t, e.CreatedAt.IsZero())
}

func TestCatalogoapp_AtualizarSaldo_Erros(t *testing.T) {
	atual := map[int64]catalogo.RegistroEstoque{1: {ID: 1, SKU: "S", Saldo: 3}}
	casos := []struct {
		nome  string
		est   *fakeEstoques
		saldo int
		err   error
	}{
		{"inexistente", &fakeEstoques{porID: map[int64]catalogo.RegistroEstoque{}}, 1, catalogo.ErrEstoqueNaoEncontrado},
		{"saldo negativo", &fakeEstoques{porID: atual}, -1, catalogo.ErrEstoqueSaldoInvalido},
		{"erro no update é propagado", &fakeEstoques{porID: atual, errSaldo: errInfra}, 1, errInfra},
		{"update sem linha", &fakeEstoques{porID: atual, errSaldo: catalogo.ErrEstoqueNaoEncontrado}, 1, catalogo.ErrEstoqueNaoEncontrado},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e, err := catalogoapp.Novo(&fakeProdutos{}, c.est).AtualizarSaldo(context.Background(), 1, c.saldo)
			assert.Nil(t, e)
			assert.ErrorIs(t, err, c.err)
		})
	}
}

// TestCatalogoapp_ReleituraAposAtualizar_Vira500: se o registro some entre
// o UPDATE e a releitura, o erro NÃO é o sentinela (não vira 404), como no
// legado, em que a releitura devolvia o erro cru do repositório.
func TestCatalogoapp_ReleituraAposAtualizar_Vira500(t *testing.T) {
	ctx := context.Background()

	_, err := catalogoapp.Novo(&fakeProdutos{porID: map[int64]catalogo.Produto{}}, &fakeEstoques{}).
		AtualizarProduto(ctx, 9, dadosProdutoValidos())
	require.Error(t, err)
	var de *kernel.Erro
	assert.False(t, errors.As(err, &de), "releitura falha → erro interno, não %v", de)
	assert.Contains(t, err.Error(), "produto não encontrado", "o motivo vai para o log")
}

func TestCatalogoapp_Produtos_Erros(t *testing.T) {
	ctx := context.Background()
	um := map[int64]catalogo.Produto{1: {ID: 1, SKU: "S", Ativo: true}}

	_, err := catalogoapp.Novo(&fakeProdutos{errListar: errInfra}, &fakeEstoques{}).
		ListarProdutos(ctx, catalogo.FiltroProduto{}, kernel.Pagina{})
	assert.ErrorIs(t, err, errInfra)

	_, err = catalogoapp.Novo(&fakeProdutos{errInsert: errInfra}, &fakeEstoques{}).CriarProduto(ctx, dadosProdutoValidos())
	assert.ErrorIs(t, err, errInfra)

	_, err = catalogoapp.Novo(&fakeProdutos{errAtualizar: errInfra}, &fakeEstoques{}).AtualizarProduto(ctx, 1, dadosProdutoValidos())
	assert.ErrorIs(t, err, errInfra)

	_, err = catalogoapp.Novo(&fakeProdutos{porID: um, errAtivo: catalogo.ErrProdutoNaoEncontrado}, &fakeEstoques{}).
		AlternarAtivoProduto(ctx, 1, nil)
	assert.ErrorIs(t, err, catalogo.ErrProdutoNaoEncontrado, "UPDATE do toggle sem linha")

	p, err := catalogoapp.Novo(&fakeProdutos{porID: um}, &fakeEstoques{}).AlternarAtivoProduto(ctx, 1, nil)
	require.NoError(t, err)
	assert.False(t, p.Ativo, "toggle")

	prod := &fakeProdutos{errPorID: errInfra}
	p, err = catalogoapp.Novo(prod, &fakeEstoques{}).CriarProduto(ctx, dadosProdutoValidos())
	require.NoError(t, err, "releitura falha: objeto em memória (BUG-09)")
	assert.Equal(t, int64(42), p.ID)
	assert.Len(t, prod.inseridos, 1)
}

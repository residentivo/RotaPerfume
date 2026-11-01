package comercial_test

// ARQ-07 / pendência (g) (reforço opcional do 🔴 TestBrain): a suíte de
// shared passa a cobrir, sem o HTTP, as duas regras de escopo do Comercial:
//   - leitura/edição de registro de outra carteira responde como "não
//     existe" (404, E-04), nunca 403 que confirmaria a existência;
//   - na escrita, o restrito só grava para cliente da carteira ATIVA
//     (400 ErrClienteForaDaCarteira, E-05) e com o próprio vendedor,
//     qualquer que seja o vendedor_id do payload.

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
)

const (
	reDesligado      = `SELECT data_desligamento IS NOT NULL FROM vendedores`
	reVisitaPorID    = `FROM visitas WHERE visita_id = \? LIMIT 1`
	reVinculoAtivo   = `WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL`
	reInserirVisita  = `INSERT INTO visitas`
	vendedorDoEscopo = int64(3)
	vendedorAlheio   = int64(9)
)

var colsVisita = []string{"visita_id", "cliente_id", "vendedor_id", "data_visita", "resultado", "duracao_min", "created_at", "updated_at"}

func visitaDoVendedor(id, vendedorID int64) *sqlmock.Rows {
	return sqlmock.NewRows(colsVisita).AddRow(id, int64(2), vendedorID, agoraFixo, "ok", 30, agoraFixo, agoraFixo)
}

func esperarAtivo(m sqlmock.Sqlmock) {
	m.ExpectQuery(reDesligado).WithArgs(vendedorDoEscopo).WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(0))
}

func TestARQ07_G_RegistroDeOutraCarteira_404Nunca403(t *testing.T) {
	ctx := context.Background()
	lerVisita := func(id int64) func() (comercial.DadosVisita, error) {
		return func() (comercial.DadosVisita, error) {
			t.Fatalf("o body não pode ser lido antes da checagem de posse (visita %d)", id)
			return comercial.DadosVisita{}, nil
		}
	}
	type caso struct {
		nome       string
		vendedor   int64 // dono do registro lido
		querySQL   bool  // false: SemAcesso, nenhuma consulta
		semVinculo bool
		op         string
		want       error
	}
	cs := []caso{
		{nome: "obter visita alheia", vendedor: vendedorAlheio, querySQL: true, op: "obter", want: comercial.ErrVisitaNaoEncontrada},
		{nome: "atualizar visita alheia", vendedor: vendedorAlheio, querySQL: true, op: "atualizar", want: comercial.ErrVisitaNaoEncontrada},
		{nome: "excluir visita alheia", vendedor: vendedorAlheio, querySQL: true, op: "excluir", want: comercial.ErrVisitaNaoEncontrada},
		{nome: "obter visita sem vendedor vinculado", semVinculo: true, op: "obter", want: comercial.ErrVisitaNaoEncontrada},
		{nome: "excluir visita sem vendedor vinculado", semVinculo: true, op: "excluir", want: comercial.ErrVisitaNaoEncontrada},
		{nome: "obter visita própria", vendedor: vendedorDoEscopo, querySQL: true, op: "obter", want: nil},
	}
	for _, c := range cs {
		t.Run(c.nome, func(t *testing.T) {
			pool, mock := novoMock(t)
			v := vinculoFake{id: vendedorDoEscopo}
			if c.semVinculo {
				v = vinculoFake{}
			}
			if c.querySQL {
				esperarAtivo(mock)
				mock.ExpectQuery(reVisitaPorID).WithArgs(int64(5)).WillReturnRows(visitaDoVendedor(5, c.vendedor))
			}
			uc := novosCasos(pool, v)
			p := principal(t, kernel.RoleNormal)
			var err error
			switch c.op {
			case "obter":
				var vi *comercial.Visita
				vi, err = uc.ObterVisita(ctx, p, 5)
				if c.want == nil {
					require.NoError(t, err)
					assert.Equal(t, vendedorDoEscopo, vi.VendedorID)
				}
			case "atualizar":
				_, err = uc.AtualizarVisita(ctx, p, 5, lerVisita(5))
			case "excluir":
				err = uc.ExcluirVisita(ctx, p, 5)
			}
			if c.want != nil {
				assert.Same(t, c.want, err, "E-04: a mesma resposta de \"não existe\"")
				var ke *kernel.Erro
				require.ErrorAs(t, err, &ke)
				assert.Equal(t, kernel.KindNaoEncontrado, ke.Kind(), "404, nunca 403")
			}
			assert.NoError(t, mock.ExpectationsWereMet(), "nenhuma escrita depois da recusa")
		})
	}
}

func TestARQ07_G_ClienteForaDaCarteira_404NaLeitura(t *testing.T) {
	pool, mock := novoMock(t)
	esperarAtivo(mock)
	mock.ExpectQuery(reClientePorID).WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows(colsCliente).AddRow(int64(2), "c", "r", "s", "ci", "PR", "", agoraFixo, true, agoraFixo, agoraFixo))
	mock.ExpectQuery(reVinculoAtivo).WithArgs(vendedorDoEscopo, int64(2)).WillReturnRows(sqlmock.NewRows(colsCarteira))

	_, err := novosCasos(pool, vinculoFake{id: vendedorDoEscopo}).ObterCliente(context.Background(), principal(t, kernel.RoleNormal), 2)
	assert.Same(t, comercial.ErrClienteNaoEncontrado, err, "cliente existe, mas fora da carteira: 404")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestARQ07_G_EscritaExigeCarteiraAtivaEForcaOVendedor(t *testing.T) {
	ctx := context.Background()
	dados := func(vendedorPayload int64) func() (comercial.DadosVisita, error) {
		return func() (comercial.DadosVisita, error) {
			return comercial.DadosVisita{ClienteID: 2, VendedorID: vendedorPayload, DataVisita: "2026-05-06", Resultado: "ok", DuracaoMin: 30}, nil
		}
	}
	casos := []struct {
		nome            string
		vendedorPayload int64
		naCarteira      bool
		want            error
	}{
		{"cliente fora da carteira ativa → 400, sem INSERT", vendedorDoEscopo, false, comercial.ErrClienteForaDaCarteira},
		{"fora da carteira, mesmo apontando outro vendedor → 400", vendedorAlheio, false, comercial.ErrClienteForaDaCarteira},
		{"na carteira, payload com vendedor alheio → grava com o do escopo", vendedorAlheio, true, nil},
		{"na carteira, sem vendedor no payload → grava com o do escopo", 0, true, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pool, mock := novoMock(t)
			esperarAtivo(mock)
			vinculo := sqlmock.NewRows(colsCarteira)
			if c.naCarteira {
				vinculo.AddRow(int64(4), int64(2), vendedorDoEscopo, agoraFixo, nil, agoraFixo, agoraFixo)
			}
			mock.ExpectQuery(reVinculoAtivo).WithArgs(vendedorDoEscopo, int64(2)).WillReturnRows(vinculo)
			if c.naCarteira {
				mock.ExpectQuery(reExisteCliente).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(reExisteVendedor).WithArgs(vendedorDoEscopo).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectExec(reInserirVisita).WithArgs(int64(2), vendedorDoEscopo, sqlmock.AnyArg(), "ok", 30).
					WillReturnResult(sqlmock.NewResult(11, 1))
				mock.ExpectQuery(reVisitaPorID).WithArgs(int64(11)).WillReturnRows(visitaDoVendedor(11, vendedorDoEscopo))
			}

			v, err := novosCasos(pool, vinculoFake{id: vendedorDoEscopo}).CriarVisita(ctx, principal(t, kernel.RoleNormal), dados(c.vendedorPayload))
			if c.want != nil {
				assert.Same(t, c.want, err)
				assert.Nil(t, v)
			} else {
				require.NoError(t, err)
				assert.Equal(t, vendedorDoEscopo, v.VendedorID, "E-05: o restrito grava sempre com o próprio vendedor")
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

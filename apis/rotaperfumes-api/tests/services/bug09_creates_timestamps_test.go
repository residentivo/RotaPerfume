package services_test

// BUG-09: os POSTs de vendedor, produto, pagamento, oportunidade, visita e
// estoque devolviam created_at/updated_at zerados porque o service retornava o
// objeto em memória. Agora cada create relê o registro via repo.GetByID após o
// INSERT (fallback: objeto em memória, sem erro, se a releitura falhar — o
// INSERT já foi confirmado). Mesmo padrão do BUG-08 (clientes).

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

// bug09Resultado normaliza o retorno dos 6 creates para as asserções comuns.
type bug09Resultado struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
	// Marcador é um campo cujo valor difere entre o objeto em memória e a linha
	// do banco mockada: prova de onde veio a resposta.
	Marcador string
}

type bug09Caso struct {
	nome string
	id   int64
	// antesDoInsert registra as consultas de validação e o INSERT.
	antesDoInsert func(mock sqlmock.Sqlmock)
	// reGetByID é a regex da releitura (repo.GetByID).
	reGetByID string
	// linhaGravada monta a linha devolvida pela releitura.
	linhaGravada func(id int64, criado, atualizado time.Time) *sqlmock.Rows
	// criar chama o create do service.
	criar func(db *sql.DB, verbose bool) (*bug09Resultado, error)
	// marcadorBanco / marcadorMemoria: valor esperado do Marcador quando a
	// resposta vem da releitura / do objeto em memória (fallback).
	marcadorBanco   string
	marcadorMemoria string
}

func bug09Casos() []bug09Caso {
	return []bug09Caso{
		{
			nome: "vendedor",
			id:   5,
			antesDoInsert: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO vendedores \(nome, regiao, uf, data_admissao, data_desligamento, meta_mensal\)`).
					WithArgs("Novo Vendedor", "Sudeste", "SP", sqlmock.AnyArg(), nil, 5000.0).
					WillReturnResult(sqlmock.NewResult(5, 1))
			},
			reGetByID: `SELECT ` + vendedorGetColunasRegex,
			linhaGravada: func(id int64, criado, atualizado time.Time) *sqlmock.Rows {
				return sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_admissao", "data_desligamento", "meta_mensal", "created_at", "updated_at"}).
					AddRow(id, "Vendedor Do Banco", "Sudeste", "SP", criado, nil, 5000.0, criado, atualizado)
			},
			criar: func(db *sql.DB, verbose bool) (*bug09Resultado, error) {
				v, err := services.NewVendedorService(db, vendedorTestCfg(verbose)).
					CreateVendedor(context.Background(), db, validVendedorInput())
				if err != nil || v == nil {
					return nil, err
				}
				return &bug09Resultado{v.ID, v.CreatedAt, v.UpdatedAt, v.Nome}, nil
			},
			marcadorBanco:   "Vendedor Do Banco",
			marcadorMemoria: "Novo Vendedor",
		},
		{
			nome: "produto",
			id:   11,
			antesDoInsert: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO produtos \(sku, descricao, categoria, marca, nota_olfativa, preco_tabela, custo_unitario, unidade, data_lancamento, ativo\)`).
					WithArgs("SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico", 99.90, 45.00, "UN", sqlmock.AnyArg(), true).
					WillReturnResult(sqlmock.NewResult(11, 1))
			},
			reGetByID: `SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`,
			linhaGravada: func(id int64, criado, atualizado time.Time) *sqlmock.Rows {
				return sqlmock.NewRows(produtoColunasHeader()).
					AddRow(id, "SKU-001", "Descricao Do Banco", "Perfumaria", "Marca X", "Cítrico",
						99.90, 45.00, "UN", criado, true, criado, atualizado)
			},
			criar: func(db *sql.DB, verbose bool) (*bug09Resultado, error) {
				p, err := services.NewProdutoService(db, produtoTestCfg(verbose)).
					CreateProduto(context.Background(), db, validProdutoInput())
				if err != nil || p == nil {
					return nil, err
				}
				return &bug09Resultado{p.ID, p.CreatedAt, p.UpdatedAt, p.Descricao}, nil
			},
			marcadorBanco:   "Descricao Do Banco",
			marcadorMemoria: "Perfume Teste",
		},
		{
			nome: "pagamento",
			id:   21,
			antesDoInsert: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM pedidos WHERE pedido_id_origem = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectExec(`INSERT INTO pagamentos \(pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento\)`).
					WithArgs(int64(1), "PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto").
					WillReturnResult(sqlmock.NewResult(21, 1))
			},
			reGetByID: `SELECT ` + pagamentoColunasRegex + pagamentoFromRegex + ` WHERE pagamento_id = \? LIMIT 1`,
			linhaGravada: func(id int64, criado, atualizado time.Time) *sqlmock.Rows {
				return sqlmock.NewRows(pagamentoColunasHeader()).
					AddRow(id, int64(1), "Boleto", uint8(1), 100.0, 0.0, 100.0, criado, nil, "Em aberto", criado, atualizado)
			},
			criar: func(db *sql.DB, verbose bool) (*bug09Resultado, error) {
				p, err := services.NewPagamentoService(db, pagamentoTestCfg(verbose)).
					CreatePagamento(context.Background(), db, validPagamentoInput())
				if err != nil || p == nil {
					return nil, err
				}
				return &bug09Resultado{p.PagamentoID, p.CreatedAt, p.UpdatedAt, p.FormaPagamento}, nil
			},
			marcadorBanco:   "Boleto",
			marcadorMemoria: "PIX",
		},
		{
			nome: "oportunidade",
			id:   31,
			antesDoInsert: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(clienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(vendedorExistsRows())
				mock.ExpectExec(`INSERT INTO oportunidades`).
					WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(31, 1))
			},
			reGetByID: `SELECT ` + oportunidadeColunasRegex + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`,
			linhaGravada: func(id int64, criado, atualizado time.Time) *sqlmock.Rows {
				return sqlmock.NewRows([]string{
					"oportunidade_id", "cliente_id", "vendedor_id", "origem", "data_abertura", "etapa",
					"probabilidade_pct", "valor_estimado", "data_fechamento", "ciclo_dias", "motivo_perda",
					"created_at", "updated_at",
				}).AddRow(id, int64(100), int64(1), "Origem Do Banco", criado, "Prospeccao", 10.0, 1000.0, nil, nil, nil, criado, atualizado)
			},
			criar: func(db *sql.DB, verbose bool) (*bug09Resultado, error) {
				o, err := services.NewOportunidadeService(db, oportunidadeTestCfg(verbose)).
					CreateOportunidade(context.Background(), db, validOportunidadeInput())
				if err != nil || o == nil {
					return nil, err
				}
				return &bug09Resultado{o.OportunidadeID, o.CreatedAt, o.UpdatedAt, o.Origem}, nil
			},
			marcadorBanco:   "Origem Do Banco",
			marcadorMemoria: "Site",
		},
		{
			nome: "visita",
			id:   41,
			antesDoInsert: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(visitaClienteExistsRows())
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(visitaVendedorExistsRows())
				mock.ExpectExec(`INSERT INTO visitas`).
					WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 30).
					WillReturnResult(sqlmock.NewResult(41, 1))
			},
			reGetByID: `SELECT ` + visitaColunasRegex + ` FROM visitas WHERE visita_id = \? LIMIT 1`,
			linhaGravada: func(id int64, criado, atualizado time.Time) *sqlmock.Rows {
				return sqlmock.NewRows([]string{
					"visita_id", "cliente_id", "vendedor_id", "data_visita", "resultado", "duracao_min",
					"created_at", "updated_at",
				}).AddRow(id, int64(100), int64(1), criado, "Negativa", 30, criado, atualizado)
			},
			criar: func(db *sql.DB, verbose bool) (*bug09Resultado, error) {
				v, err := services.NewVisitaService(db, visitaTestCfg(verbose)).
					CreateVisita(context.Background(), db, validVisitaInput())
				if err != nil || v == nil {
					return nil, err
				}
				return &bug09Resultado{v.VisitaID, v.CreatedAt, v.UpdatedAt, v.Resultado}, nil
			},
			marcadorBanco:   "Negativa",
			marcadorMemoria: "Positiva",
		},
		{
			nome: "estoque",
			id:   51,
			antesDoInsert: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM produtos WHERE sku = \? LIMIT 1`).
					WithArgs("SKU-001").
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectExec(`INSERT INTO estoque`).
					WithArgs("2024-06-01", "SKU-001", 10, false).
					WillReturnResult(sqlmock.NewResult(51, 1))
			},
			reGetByID: `SELECT .+ FROM estoque.+WHERE e\.id = \? LIMIT 1`,
			linhaGravada: func(id int64, criado, atualizado time.Time) *sqlmock.Rows {
				// produto_descricao só existe no banco (JOIN): prova da releitura.
				return sqlmock.NewRows(estoqueColunasHeader()).
					AddRow(id, criado, "SKU-001", "Perfume Teste", 10, false, criado, atualizado)
			},
			criar: func(db *sql.DB, verbose bool) (*bug09Resultado, error) {
				e, err := services.NewEstoqueService(db, estoqueTestCfg(verbose)).
					CreateEstoque(context.Background(), db, validEstoqueInput())
				if err != nil || e == nil {
					return nil, err
				}
				return &bug09Resultado{e.ID, e.CreatedAt, e.UpdatedAt, e.ProdutoDescricao}, nil
			},
			marcadorBanco:   "Perfume Teste",
			marcadorMemoria: "",
		},
	}
}

func TestBUG09_Create_DevolveTimestampsDoBanco(t *testing.T) {
	criado := time.Date(2026, 9, 26, 10, 30, 0, 0, time.Local)
	atualizado := time.Date(2026, 9, 26, 10, 31, 0, 0, time.Local)

	for _, tc := range bug09Casos() {
		for _, verbose := range []bool{false, true} {
			tc, verbose := tc, verbose
			nome := tc.nome + "/quiet"
			if verbose {
				nome = tc.nome + "/verbose"
			}
			t.Run(nome, func(t *testing.T) {
				db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
				require.NoError(t, err)
				t.Cleanup(func() { db.Close() })

				tc.antesDoInsert(mock)
				mock.ExpectQuery(tc.reGetByID).WithArgs(tc.id).
					WillReturnRows(tc.linhaGravada(tc.id, criado, atualizado))

				r, err := tc.criar(db, verbose)

				require.NoError(t, err)
				require.NotNil(t, r)
				assert.Equal(t, tc.id, r.ID)
				assert.False(t, r.CreatedAt.IsZero(), "created_at deve vir preenchido")
				assert.False(t, r.UpdatedAt.IsZero(), "updated_at deve vir preenchido")
				assert.True(t, r.CreatedAt.Equal(criado), "created_at deve vir do GetByID")
				assert.True(t, r.UpdatedAt.Equal(atualizado), "updated_at deve vir do GetByID")
				assert.Equal(t, tc.marcadorBanco, r.Marcador, "resposta deve ser a linha relida")
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

func TestBUG09_Create_FalhaNaReleitura_DevolveObjetoEmMemoriaSemErro(t *testing.T) {
	falhas := []struct {
		nome string
		err  error
	}{
		{"erro de conexão", errors.New("db down")},
		{"linha não encontrada", sql.ErrNoRows},
	}

	for _, tc := range bug09Casos() {
		for _, f := range falhas {
			tc, f := tc, f
			t.Run(tc.nome+"/"+f.nome, func(t *testing.T) {
				db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
				require.NoError(t, err)
				t.Cleanup(func() { db.Close() })

				tc.antesDoInsert(mock)
				mock.ExpectQuery(tc.reGetByID).WithArgs(tc.id).WillReturnError(f.err)

				r, err := tc.criar(db, false)

				require.NoError(t, err, "INSERT confirmado não pode virar erro por falha só na releitura")
				require.NotNil(t, r)
				assert.Equal(t, tc.id, r.ID, "fallback mantém o id gerado no INSERT")
				assert.True(t, r.CreatedAt.IsZero(), "fallback é o objeto em memória")
				assert.True(t, r.UpdatedAt.IsZero(), "fallback é o objeto em memória")
				assert.Equal(t, tc.marcadorMemoria, r.Marcador, "resposta deve ser o objeto em memória")
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

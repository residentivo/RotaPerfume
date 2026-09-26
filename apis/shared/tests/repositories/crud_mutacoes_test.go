package repositories_test

// Operações de escrita/consulta pontual dos repositórios que não tinham
// cobertura: Delete (oportunidade, pagamento, visita), SetAtivo/Update/
// SetDeveTrocarSenha (usuário e cliente), ReativarVinculo (carteira),
// Exists* (cliente, pagamento, produto), GetByID de cliente,
// GetVinculoByClienteVendedorData (carteira), UsuarioRepository.Create e
// PedidoRepository.DeleteComItens. Tudo com sqlmock e casos parametrizados.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

// ---------------------------------------------------------------------------
// Execs que devolvem ErrNotFound quando nenhuma linha é afetada
// ---------------------------------------------------------------------------

type opExec struct {
	nome  string
	sqlRe string
	args  []driver.Value
	// checaRowsAffected: a função devolve o erro de RowsAffected (true) ou o
	// ignora e trata como 0 linhas, isto é, ErrNotFound (false).
	checaRowsAffected bool
	chama             func(ctx context.Context, db *sql.DB) error
}

func TestRepositorios_ExecComNotFound(t *testing.T) {
	ctx := context.Background()
	vend := int64(3)
	cli := &models.Cliente{CNPJ: "11222333000181", RazaoSocial: "ACME", UF: "SP", DataCadastro: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}

	ops := []opExec{
		{
			nome: "OportunidadeRepository.Delete", sqlRe: `DELETE FROM oportunidades WHERE oportunidade_id = \?`,
			args: []driver.Value{int64(5)}, checaRowsAffected: true,
			chama: func(ctx context.Context, db *sql.DB) error {
				return repositories.NewOportunidadeRepository().Delete(ctx, db, 5)
			},
		},
		{
			nome: "PagamentoRepository.Delete", sqlRe: `DELETE FROM pagamentos WHERE pagamento_id = \?`,
			args: []driver.Value{int64(5)}, checaRowsAffected: true,
			chama: func(ctx context.Context, db *sql.DB) error {
				return repositories.NewPagamentoRepository().Delete(ctx, db, 5)
			},
		},
		{
			nome: "VisitaRepository.Delete", sqlRe: `DELETE FROM visitas WHERE visita_id = \?`,
			args: []driver.Value{int64(5)}, checaRowsAffected: true,
			chama: func(ctx context.Context, db *sql.DB) error {
				return repositories.NewVisitaRepository().Delete(ctx, db, 5)
			},
		},
		{
			nome: "CarteiraRepository.ReativarVinculo", sqlRe: `UPDATE carteiras SET data_fim = NULL WHERE carteira_id_origem = \?`,
			args: []driver.Value{int64(5)}, checaRowsAffected: true,
			chama: func(ctx context.Context, db *sql.DB) error {
				return repositories.NewCarteiraRepository().ReativarVinculo(ctx, db, 5)
			},
		},
		{
			nome: "ClienteRepository.Update", sqlRe: `UPDATE clientes\s+SET cnpj = \?, razao_social = \?, segmento = \?, cidade = \?, uf = \?, bairro = \?, data_cadastro = \?\s+WHERE cliente_id_origem = \?`,
			args:              []driver.Value{cli.CNPJ, cli.RazaoSocial, "", "", "SP", "", cli.DataCadastro, int64(5)},
			checaRowsAffected: true,
			chama: func(ctx context.Context, db *sql.DB) error {
				return repositories.NewClienteRepository().Update(ctx, db, 5, cli)
			},
		},
		{
			nome: "ClienteRepository.SetAtivo", sqlRe: `UPDATE clientes SET ativo = \? WHERE cliente_id_origem = \?`,
			args: []driver.Value{false, int64(5)},
			chama: func(ctx context.Context, db *sql.DB) error {
				return repositories.NewClienteRepository().SetAtivo(ctx, db, 5, false)
			},
		},
		{
			nome: "UsuarioRepository.SetAtivo", sqlRe: `UPDATE usuarios SET ativo = \? WHERE id = \?`,
			args: []driver.Value{true, int64(5)},
			chama: func(ctx context.Context, db *sql.DB) error {
				return repositories.NewUsuarioRepository().SetAtivo(ctx, db, 5, true)
			},
		},
		{
			nome: "UsuarioRepository.Update com vendedor", sqlRe: `UPDATE usuarios SET nome = \?, role = \?, id_vendedor = \? WHERE id = \?`,
			args: []driver.Value{"Ana", "normal", int64(3), int64(5)},
			chama: func(ctx context.Context, db *sql.DB) error {
				return repositories.NewUsuarioRepository().Update(ctx, db, 5, "Ana", "normal", &vend)
			},
		},
		{
			nome: "UsuarioRepository.Update sem vendedor (NULL)", sqlRe: `UPDATE usuarios SET nome = \?, role = \?, id_vendedor = \? WHERE id = \?`,
			args: []driver.Value{"Adm", "admin", nil, int64(5)},
			chama: func(ctx context.Context, db *sql.DB) error {
				return repositories.NewUsuarioRepository().Update(ctx, db, 5, "Adm", "admin", nil)
			},
		},
		{
			nome: "UsuarioRepository.SetDeveTrocarSenha", sqlRe: `UPDATE usuarios SET deve_trocar_senha = \? WHERE id = \?`,
			args: []driver.Value{true, int64(5)},
			chama: func(ctx context.Context, db *sql.DB) error {
				return repositories.NewUsuarioRepository().SetDeveTrocarSenha(ctx, db, 5, true)
			},
		},
	}

	errExec := errors.New("falha no exec")
	errRA := errors.New("falha no rowsAffected")

	for _, op := range ops {
		casos := []struct {
			nome      string
			resultado func(e *sqlmock.ExpectedExec)
			wantErrIs error
		}{
			{"sucesso", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 1)) }, nil},
			{"nenhuma linha vira ErrNotFound", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 0)) }, repositories.ErrNotFound},
			{"erro do exec é propagado", func(e *sqlmock.ExpectedExec) { e.WillReturnError(errExec) }, errExec},
			{"erro de RowsAffected", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewErrorResult(errRA)) },
				map[bool]error{true: errRA, false: repositories.ErrNotFound}[op.checaRowsAffected]},
		}
		for _, c := range casos {
			t.Run(op.nome+"/"+c.nome, func(t *testing.T) {
				db, mock := newMock(t)
				defer db.Close()
				c.resultado(mock.ExpectExec(op.sqlRe).WithArgs(op.args...))

				err := op.chama(ctx, db)
				if c.wantErrIs == nil {
					assert.NoError(t, err)
				} else {
					assert.ErrorIs(t, err, c.wantErrIs)
				}
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

func TestClienteUpdate_CNPJDuplicado(t *testing.T) {
	casos := []struct {
		nome    string
		err     error
		wantDup bool
	}{
		{"1062 no índice de CNPJ", &mysql.MySQLError{Number: 1062, Message: "Duplicate entry '1' for key 'clientes.uq_clientes_cnpj'"}, true},
		{"1062 em outro índice não é CNPJ duplicado", &mysql.MySQLError{Number: 1062, Message: "Duplicate entry '1' for key 'PRIMARY'"}, false},
		{"outro erro MySQL", &mysql.MySQLError{Number: 1452, Message: "uq_clientes_cnpj"}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			mock.ExpectExec(`UPDATE clientes`).WillReturnError(c.err)

			err := repositories.NewClienteRepository().Update(context.Background(), db, 5, &models.Cliente{CNPJ: "1"})
			require.Error(t, err)
			assert.Equal(t, c.wantDup, errors.Is(err, repositories.ErrCNPJDuplicado))
			if !c.wantDup {
				var myErr *mysql.MySQLError
				assert.True(t, errors.As(err, &myErr), "o erro original deve continuar embrulhado")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Exists*
// ---------------------------------------------------------------------------

func TestRepositorios_Exists(t *testing.T) {
	ctx := context.Background()
	ops := []struct {
		nome  string
		sqlRe string
		arg   driver.Value
		chama func(db *sql.DB) (bool, error)
	}{
		{"ClienteRepository.ExistsByID", `SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`, int64(9),
			func(db *sql.DB) (bool, error) { return repositories.NewClienteRepository().ExistsByID(ctx, db, 9) }},
		{"PagamentoRepository.ExistsByPedidoID", `SELECT 1 FROM pagamentos WHERE pedido_id = \? LIMIT 1`, int64(9),
			func(db *sql.DB) (bool, error) {
				return repositories.NewPagamentoRepository().ExistsByPedidoID(ctx, db, 9)
			}},
		{"ProdutoRepository.ExistsBySKU", `SELECT 1 FROM produtos WHERE sku = \? LIMIT 1`, "SKU-9",
			func(db *sql.DB) (bool, error) {
				return repositories.NewProdutoRepository().ExistsBySKU(ctx, db, "SKU-9")
			}},
	}
	errBanco := errors.New("timeout")

	for _, op := range ops {
		casos := []struct {
			nome      string
			prepara   func(e *sqlmock.ExpectedQuery)
			want      bool
			wantErrIs error
		}{
			{"existe", func(e *sqlmock.ExpectedQuery) { e.WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1)) }, true, nil},
			{"não existe", func(e *sqlmock.ExpectedQuery) { e.WillReturnRows(sqlmock.NewRows([]string{"1"})) }, false, nil},
			{"erro do banco", func(e *sqlmock.ExpectedQuery) { e.WillReturnError(errBanco) }, false, errBanco},
		}
		for _, c := range casos {
			t.Run(op.nome+"/"+c.nome, func(t *testing.T) {
				db, mock := newMock(t)
				defer db.Close()
				c.prepara(mock.ExpectQuery(op.sqlRe).WithArgs(op.arg))

				got, err := op.chama(db)
				assert.Equal(t, c.want, got)
				if c.wantErrIs == nil {
					assert.NoError(t, err)
				} else {
					assert.ErrorIs(t, err, c.wantErrIs)
				}
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

// ---------------------------------------------------------------------------
// ClienteRepository.GetByID e CarteiraRepository.GetVinculoByClienteVendedorData
// ---------------------------------------------------------------------------

func TestClienteGetByID(t *testing.T) {
	agora := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	errBanco := errors.New("conexão caiu")
	casos := []struct {
		nome      string
		prepara   func(e *sqlmock.ExpectedQuery)
		wantErrIs error
	}{
		{"encontrado", func(e *sqlmock.ExpectedQuery) {
			e.WillReturnRows(sqlmock.NewRows(clienteRepoColumns).
				AddRow(int64(9), "11222333000181", "ACME", "Varejo", "Campinas", "SP", "Centro", agora, true, agora, agora))
		}, nil},
		{"não encontrado", func(e *sqlmock.ExpectedQuery) { e.WillReturnRows(sqlmock.NewRows(clienteRepoColumns)) }, repositories.ErrNotFound},
		{"erro do banco", func(e *sqlmock.ExpectedQuery) { e.WillReturnError(errBanco) }, errBanco},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			c.prepara(mock.ExpectQuery(`SELECT ` + clienteColunasRegexp + ` FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).WithArgs(int64(9)))

			got, err := repositories.NewClienteRepository().GetByID(context.Background(), db, 9)
			if c.wantErrIs != nil {
				assert.ErrorIs(t, err, c.wantErrIs)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, int64(9), got.ClienteIDOrigem)
				assert.Equal(t, "ACME", got.RazaoSocial)
				assert.Equal(t, "Centro", got.Bairro)
				assert.True(t, got.Ativo)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCarteiraGetVinculoByClienteVendedorData(t *testing.T) {
	// A data é enviada só como YYYY-MM-DD, independente do horário/fuso.
	dataInicio := time.Date(2026, 9, 26, 23, 59, 0, 0, time.FixedZone("BRT", -3*3600))
	agora := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	errBanco := errors.New("conexão caiu")

	casos := []struct {
		nome      string
		prepara   func(e *sqlmock.ExpectedQuery)
		wantErrIs error
		wantFim   bool
	}{
		{"vínculo ativo", func(e *sqlmock.ExpectedQuery) {
			e.WillReturnRows(sqlmock.NewRows(carteiraColumns).AddRow(int64(4), int64(10), int64(3), dataInicio, nil, agora, agora))
		}, nil, false},
		{"vínculo encerrado", func(e *sqlmock.ExpectedQuery) {
			e.WillReturnRows(sqlmock.NewRows(carteiraColumns).AddRow(int64(4), int64(10), int64(3), dataInicio, agora, agora, agora))
		}, nil, true},
		{"não encontrado", func(e *sqlmock.ExpectedQuery) { e.WillReturnRows(sqlmock.NewRows(carteiraColumns)) }, repositories.ErrNotFound, false},
		{"erro do banco", func(e *sqlmock.ExpectedQuery) { e.WillReturnError(errBanco) }, errBanco, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			c.prepara(mock.ExpectQuery(`FROM carteiras\s+WHERE cliente_id = \? AND vendedor_id = \? AND data_inicio = \?`).
				WithArgs(int64(10), int64(3), "2026-09-26"))

			got, err := repositories.NewCarteiraRepository().GetVinculoByClienteVendedorData(context.Background(), db, 10, 3, dataInicio)
			if c.wantErrIs != nil {
				assert.ErrorIs(t, err, c.wantErrIs)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, int64(4), got.CarteiraIDOrigem)
				assert.Equal(t, c.wantFim, got.DataFim != nil)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// UsuarioRepository.Create
// ---------------------------------------------------------------------------

func TestUsuarioCreate(t *testing.T) {
	agora := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	vend := int64(3)
	errBanco := errors.New("falha de rede")
	errLastID := errors.New("sem last insert id")

	const insertRe = `INSERT INTO usuarios \(nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha\)`
	const selectRe = `WHERE u.id = \?`

	casos := []struct {
		nome      string
		prepara   func(m sqlmock.Sqlmock)
		wantErrIs error
		wantErr   bool
	}{
		{"sucesso relê o registro criado", func(m sqlmock.Sqlmock) {
			m.ExpectExec(insertRe).WithArgs("Ana", "ana@rp.com", "hash", "normal", int64(3), true, true).
				WillReturnResult(sqlmock.NewResult(42, 1))
			m.ExpectQuery(selectRe).WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows(baseColumns).
				AddRow(int64(42), "Ana", "ana@rp.com", "hash", "normal", int64(3), true, true, agora, agora, nil, "Vendedor 3"))
		}, nil, false},
		{"email duplicado (Error 1062)", func(m sqlmock.Sqlmock) {
			m.ExpectExec(insertRe).WillReturnError(errors.New("Error 1062 (23000): Duplicate entry 'ana@rp.com' for key 'usuarios.email'"))
		}, repositories.ErrEmailDuplicado, true},
		{"email duplicado (só Duplicate entry)", func(m sqlmock.Sqlmock) {
			m.ExpectExec(insertRe).WillReturnError(errors.New("Duplicate entry 'ana@rp.com'"))
		}, repositories.ErrEmailDuplicado, true},
		{"erro genérico no insert", func(m sqlmock.Sqlmock) {
			m.ExpectExec(insertRe).WillReturnError(errBanco)
		}, errBanco, true},
		{"erro no LastInsertId", func(m sqlmock.Sqlmock) {
			m.ExpectExec(insertRe).WillReturnResult(sqlmock.NewErrorResult(errLastID))
		}, errLastID, true},
		{"registro some antes da releitura", func(m sqlmock.Sqlmock) {
			m.ExpectExec(insertRe).WillReturnResult(sqlmock.NewResult(42, 1))
			m.ExpectQuery(selectRe).WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows(baseColumns))
		}, repositories.ErrNotFound, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			c.prepara(mock)

			u := &models.Usuario{Nome: "Ana", Email: "ana@rp.com", PasswordHash: "hash", Role: "normal", IDVendedor: &vend, Ativo: true, DeveTrocarSenha: true}
			err := repositories.NewUsuarioRepository().Create(context.Background(), db, u)
			if c.wantErr {
				assert.ErrorIs(t, err, c.wantErrIs)
				assert.Zero(t, u.ID, "o usuário não deve ser populado em caso de erro")
			} else {
				require.NoError(t, err)
				assert.Equal(t, int64(42), u.ID)
				assert.Equal(t, agora, u.CreatedAt)
				require.NotNil(t, u.VendedorNome)
				assert.Equal(t, "Vendedor 3", *u.VendedorNome)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// PedidoRepository.DeleteComItens
// ---------------------------------------------------------------------------

func TestPedidoDeleteComItens(t *testing.T) {
	const delItens = `DELETE FROM itens_pedido WHERE pedido_id = \?`
	const delPedido = `DELETE FROM pedidos WHERE pedido_id_origem = \?`
	errX := errors.New("falha")

	casos := []struct {
		nome      string
		prepara   func(m sqlmock.Sqlmock)
		wantErrIs error
		wantMsg   string
	}{
		{"sucesso remove itens e pedido na mesma transação", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(delItens).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 3))
			m.ExpectExec(delPedido).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
		}, nil, ""},
		{"pedido sem itens também é removido", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(delItens).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectExec(delPedido).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
		}, nil, ""},
		{"falha no begin", func(m sqlmock.Sqlmock) {
			m.ExpectBegin().WillReturnError(errX)
		}, errX, "begin tx delete pedido"},
		{"falha ao remover itens faz rollback", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(delItens).WillReturnError(errX)
			m.ExpectRollback()
		}, errX, "delete itens_pedido"},
		{"falha ao remover pedido faz rollback", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(delItens).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(delPedido).WillReturnError(errX)
			m.ExpectRollback()
		}, errX, "delete pedido"},
		{"erro de RowsAffected faz rollback", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(delItens).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(delPedido).WillReturnResult(sqlmock.NewErrorResult(errX))
			m.ExpectRollback()
		}, errX, "rowsAffected"},
		{"pedido inexistente faz rollback e devolve ErrNotFound", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(delItens).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectExec(delPedido).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectRollback()
		}, repositories.ErrNotFound, ""},
		{"falha no commit", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(delItens).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(delPedido).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit().WillReturnError(errX)
		}, errX, "commit delete pedido"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			c.prepara(mock)

			err := repositories.NewPedidoRepository().DeleteComItens(context.Background(), db, 7)
			if c.wantErrIs == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, c.wantErrIs)
				assert.Contains(t, err.Error(), c.wantMsg)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

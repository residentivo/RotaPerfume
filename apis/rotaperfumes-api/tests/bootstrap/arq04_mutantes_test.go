package bootstrap_test

// ARQ-04 (TestBrain): os dois UoW do Comercial montados pelo bootstrap
// (desligamento de vendedor, I-09; cliente + vínculo de carteira do usuário
// restrito, E-05) precisam rodar TODOS os passos no MESMO *sql.Tx. Com o
// sqlmock, um passo executado no pool em vez da tx passa despercebido (as
// expectativas são as mesmas); com o pool limitado a UMA conexão, o passo
// fora da tx fica esperando a conexão presa pela tx e estoura o prazo do
// contexto. Mata os mutantes "adapter montado sobre o pool em vez do q"
// nos dois Transactors do bootstrap.

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialapp"
	"github.com/rotaperfumes/shared/kernel"
)

// poolDeUmaConexao: a tx segura a única conexão; qualquer SQL fora dela
// bloqueia até o prazo do contexto.
func poolDeUmaConexao(t *testing.T) (*sql.DB, sqlmock.Sqlmock, context.Context) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return db, mock, ctx
}

func TestARQ04_Bootstrap_DesligamentoTodosOsPassosNaMesmaTx(t *testing.T) {
	db, mock, ctx := poolDeUmaConexao(t)
	agora := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = COALESCE\(data_desligamento, \?\) WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE usuarios SET ativo = 0, tokens_validos_desde = \? WHERE id_vendedor = \? AND ativo = 1`).
		WithArgs(sqlmock.AnyArg(), int64(5)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`UPDATE refresh_tokens rt\s+JOIN usuarios u ON u\.id = rt\.usuario_id`).
		WithArgs(sqlmock.AnyArg(), "inativacao", int64(5)).WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectCommit()
	mock.ExpectQuery(`FROM vendedores WHERE id = \? LIMIT 1`).WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_admissao", "data_desligamento", "meta_mensal", "created_at", "updated_at"}).
			AddRow(int64(5), "Cinco", "Sul", "PR", agora, agora, 10.0, agora, agora))

	admin, err := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	require.NoError(t, err)
	v, err := bootstrap.NovoComercial(db).DesligarVendedor(ctx, admin, 5)

	require.NoError(t, err, "todos os passos do desligamento devem usar a conexão da tx (I-09)")
	require.NotNil(t, v)
	assert.Equal(t, int64(5), v.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestARQ04_Bootstrap_ClienteNaCarteiraClienteEVinculoNaMesmaTx(t *testing.T) {
	db, mock, ctx := poolDeUmaConexao(t)

	// Escopo do usuário restrito 7 → vendedor 3, ativo (fora da tx).
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(3)))
	mock.ExpectQuery(`SELECT data_desligamento IS NOT NULL FROM vendedores WHERE id = \? LIMIT 1`).WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"x"}).AddRow(int64(0)))
	// UoW: cliente + vínculo com o próprio vendedor.
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO clientes`).WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectExec(`INSERT INTO carteiras`).WithArgs(int64(77), int64(3), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(900, 1))
	mock.ExpectCommit()
	// Releitura (falha tolerada: devolve o objeto em memória).
	mock.ExpectQuery(`FROM clientes`).WillReturnError(sql.ErrConnDone)

	restrito, err := kernel.NovoPrincipal(7, kernel.RoleNormal)
	require.NoError(t, err)
	ler := comercialapp.Leitor[comercial.DadosCliente](func() (comercial.DadosCliente, error) {
		return comercial.DadosCliente{CNPJ: "11222333000181", RazaoSocial: "Empresa", Segmento: "varejo",
			Cidade: "Curitiba", UF: "PR", Bairro: "Centro", DataCadastro: "2024-02-10"}, nil
	})
	c, err := bootstrap.NovoComercial(db).CriarCliente(ctx, restrito, ler)

	require.NoError(t, err, "cliente e vínculo de carteira devem usar a conexão da tx (E-05)")
	require.NotNil(t, c)
	assert.Equal(t, int64(77), c.ClienteIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

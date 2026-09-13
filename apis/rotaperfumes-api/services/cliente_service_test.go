package services_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

func clienteTestCfg(verbose bool) *config.Config {
	return &config.Config{Verbose: verbose}
}

func newClienteTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

var clienteColunasRegex = `id, cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, COALESCE\(bairro, ''\), data_cadastro, ativo, created_at, updated_at`

func clienteRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
		"data_cadastro", "ativo", "created_at", "updated_at",
	}).AddRow(int64(1), int64(100), "12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro",
		now, true, now, now)
}

// ---------------------------------------------------------------------------
// ListClientes
// ---------------------------------------------------------------------------

func TestClienteService_ListClientes(t *testing.T) {
	testCases := []struct {
		nome    string
		filtro  services.ClienteFiltro
		page    int
		limit   int
		mock    func(mock sqlmock.Sqlmock)
		wantErr bool
		wantLen int
		wantTot int
	}{
		{
			nome:   "sem filtros - sucesso",
			filtro: services.ClienteFiltro{},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes ORDER BY id ASC LIMIT \? OFFSET \?`).
					WithArgs(20, 0).
					WillReturnRows(clienteRows())
			},
			wantLen: 1,
			wantTot: 1,
		},
		{
			nome:   "com filtros uf/segmento/ativo/q - repassa args ao repo",
			filtro: services.ClienteFiltro{UF: "SP", Segmento: "varejo", Ativo: boolPtr(true), Q: "Teste"},
			page:   2,
			limit:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE uf = \? AND segmento = \? AND ativo = \? AND \(razao_social LIKE \? OR cnpj LIKE \?\)`).
					WithArgs("SP", "varejo", true, "%Teste%", "%Teste%").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
				mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE uf = \? AND segmento = \? AND ativo = \? AND \(razao_social LIKE \? OR cnpj LIKE \?\) ORDER BY id ASC LIMIT \? OFFSET \?`).
					WithArgs("SP", "varejo", true, "%Teste%", "%Teste%", 10, 10).
					WillReturnRows(clienteRows())
			},
			wantLen: 1,
			wantTot: 5,
		},
		{
			nome:   "erro no count do repo é propagado",
			filtro: services.ClienteFiltro{},
			page:   1,
			limit:  20,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newClienteTestDB(t)
			tc.mock(mock)

			svc := services.NewClienteService(db, clienteTestCfg(true))
			clientes, total, err := svc.ListClientes(context.Background(), db, tc.page, tc.limit, tc.filtro)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, clientes, tc.wantLen)
				assert.Equal(t, tc.wantTot, total)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetClienteByID
// ---------------------------------------------------------------------------

func TestClienteService_GetClienteByID(t *testing.T) {
	t.Run("encontrado", func(t *testing.T) {
		db, mock := newClienteTestDB(t)
		mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(clienteRows())

		svc := services.NewClienteService(db, clienteTestCfg(false))
		c, err := svc.GetClienteByID(context.Background(), db, 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), c.ID)
		assert.Equal(t, "Empresa Teste LTDA", c.RazaoSocial)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("não encontrado retorna ErrClienteNaoEncontrado", func(t *testing.T) {
		db, mock := newClienteTestDB(t)
		mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
			WithArgs(int64(999)).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
				"data_cadastro", "ativo", "created_at", "updated_at",
			}))

		svc := services.NewClienteService(db, clienteTestCfg(false))
		c, err := svc.GetClienteByID(context.Background(), db, 999)
		assert.Nil(t, c)
		assert.ErrorIs(t, err, services.ErrClienteNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro genérico do repo é propagado", func(t *testing.T) {
		db, mock := newClienteTestDB(t)
		mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewClienteService(db, clienteTestCfg(false))
		c, err := svc.GetClienteByID(context.Background(), db, 1)
		assert.Nil(t, c)
		assert.Error(t, err)
		assert.False(t, err == services.ErrClienteNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// ToggleAtivoCliente
// ---------------------------------------------------------------------------

func TestClienteService_ToggleAtivoCliente(t *testing.T) {
	t.Run("ativo nil - inverte status atual (toggle)", func(t *testing.T) {
		db, mock := newClienteTestDB(t)
		// cliente atual está ativo=true
		mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(clienteRows())
		mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE id = \?`).
			WithArgs(false, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		svc := services.NewClienteService(db, clienteTestCfg(true))
		c, err := svc.ToggleAtivoCliente(context.Background(), db, 1, nil)
		require.NoError(t, err)
		assert.False(t, c.Ativo)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ativo explícito - define valor informado", func(t *testing.T) {
		db, mock := newClienteTestDB(t)
		mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(clienteRows())
		mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE id = \?`).
			WithArgs(true, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		svc := services.NewClienteService(db, clienteTestCfg(false))
		ativo := true
		c, err := svc.ToggleAtivoCliente(context.Background(), db, 1, &ativo)
		require.NoError(t, err)
		assert.True(t, c.Ativo)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("cliente não encontrado no GetByID", func(t *testing.T) {
		db, mock := newClienteTestDB(t)
		mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
			WithArgs(int64(999)).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
				"data_cadastro", "ativo", "created_at", "updated_at",
			}))

		svc := services.NewClienteService(db, clienteTestCfg(false))
		c, err := svc.ToggleAtivoCliente(context.Background(), db, 999, nil)
		assert.Nil(t, c)
		assert.ErrorIs(t, err, services.ErrClienteNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("cliente some entre GetByID e SetAtivo (corrida) - ErrNotFound no SetAtivo", func(t *testing.T) {
		db, mock := newClienteTestDB(t)
		mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(clienteRows())
		mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE id = \?`).
			WithArgs(false, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		svc := services.NewClienteService(db, clienteTestCfg(false))
		c, err := svc.ToggleAtivoCliente(context.Background(), db, 1, nil)
		assert.Nil(t, c)
		assert.ErrorIs(t, err, services.ErrClienteNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func boolPtr(b bool) *bool { return &b }

// ---------------------------------------------------------------------------
// CreateCliente
// ---------------------------------------------------------------------------

func validClienteInput() services.ClienteInput {
	return services.ClienteInput{
		CNPJ:         "12345678000199",
		RazaoSocial:  "Empresa Teste LTDA",
		Segmento:     "varejo",
		Cidade:       "São Paulo",
		UF:           "SP",
		Bairro:       "Centro",
		DataCadastro: "2024-01-15",
	}
}

func TestClienteService_CreateCliente(t *testing.T) {
	testCases := []struct {
		nome      string
		input     func() services.ClienteInput
		mock      func(mock sqlmock.Sqlmock)
		wantErr   error
		wantErrIs bool
	}{
		{
			nome:  "sucesso",
			input: validClienteInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COALESCE\(MAX\(cliente_id_origem\), 0\) \+ 1 FROM clientes`).
					WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(101)))
				mock.ExpectExec(`INSERT INTO clientes \(cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo\)`).
					WithArgs(int64(101), "12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", sqlmock.AnyArg(), true).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			nome: "razao_social vazia",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.RazaoSocial = "   "
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrRazaoSocialObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "cnpj vazio",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.CNPJ = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrCNPJObrigatorio,
			wantErrIs: true,
		},
		{
			nome: "segmento vazio",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.Segmento = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrSegmentoObrigatorio,
			wantErrIs: true,
		},
		{
			nome: "cidade vazia",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.Cidade = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrCidadeObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "uf inválida",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.UF = "S"
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrUFInvalida,
			wantErrIs: true,
		},
		{
			nome: "data_cadastro inválida",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.DataCadastro = "15/01/2024"
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrDataCadastroInvalida,
			wantErrIs: true,
		},
		{
			nome: "data_cadastro vazia usa hoje (sucesso)",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.DataCadastro = ""
				return in
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COALESCE\(MAX\(cliente_id_origem\), 0\) \+ 1 FROM clientes`).
					WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(101)))
				mock.ExpectExec(`INSERT INTO clientes \(cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo\)`).
					WithArgs(int64(101), "12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", sqlmock.AnyArg(), true).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			nome:  "erro no NextClienteIDOrigem é propagado",
			input: validClienteInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COALESCE\(MAX\(cliente_id_origem\), 0\) \+ 1 FROM clientes`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
		{
			nome:  "erro no Create é propagado",
			input: validClienteInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COALESCE\(MAX\(cliente_id_origem\), 0\) \+ 1 FROM clientes`).
					WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(int64(101)))
				mock.ExpectExec(`INSERT INTO clientes \(cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo\)`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newClienteTestDB(t)
			tc.mock(mock)

			svc := services.NewClienteService(db, clienteTestCfg(true))
			c, err := svc.CreateCliente(context.Background(), db, tc.input())

			if tc.wantErr != nil {
				assert.Nil(t, c)
				if tc.wantErrIs {
					assert.ErrorIs(t, err, tc.wantErr)
				} else {
					assert.Error(t, err)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, c)
				assert.Equal(t, int64(1), c.ID)
				assert.Equal(t, int64(101), c.ClienteIDOrigem)
				assert.True(t, c.Ativo)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateCliente
// ---------------------------------------------------------------------------

func TestClienteService_UpdateCliente(t *testing.T) {
	testCases := []struct {
		nome      string
		input     func() services.ClienteInput
		mock      func(mock sqlmock.Sqlmock)
		wantErr   error
		wantErrIs bool
	}{
		{
			nome:  "sucesso",
			input: validClienteInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE clientes\s+SET cnpj = \?, razao_social = \?, segmento = \?, cidade = \?, uf = \?, bairro = \?, data_cadastro = \?\s+WHERE id = \?`).
					WithArgs("12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", sqlmock.AnyArg(), int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(clienteRows())
			},
		},
		{
			nome: "razao_social vazia",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.RazaoSocial = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrRazaoSocialObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "cnpj vazio",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.CNPJ = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrCNPJObrigatorio,
			wantErrIs: true,
		},
		{
			nome: "segmento vazio",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.Segmento = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrSegmentoObrigatorio,
			wantErrIs: true,
		},
		{
			nome: "cidade vazia",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.Cidade = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrCidadeObrigatoria,
			wantErrIs: true,
		},
		{
			nome: "uf inválida",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.UF = "SPX"
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrUFInvalida,
			wantErrIs: true,
		},
		{
			nome: "data_cadastro inválida",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.DataCadastro = "2024-31-01"
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrDataCadastroInvalida,
			wantErrIs: true,
		},
		{
			nome: "data_cadastro vazia é erro na edição (defaultHoje=false)",
			input: func() services.ClienteInput {
				in := validClienteInput()
				in.DataCadastro = ""
				return in
			},
			mock:      func(mock sqlmock.Sqlmock) {},
			wantErr:   services.ErrDataCadastroInvalida,
			wantErrIs: true,
		},
		{
			nome:  "id inexistente retorna ErrClienteNaoEncontrado",
			input: validClienteInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE clientes\s+SET cnpj = \?, razao_social = \?, segmento = \?, cidade = \?, uf = \?, bairro = \?, data_cadastro = \?\s+WHERE id = \?`).
					WithArgs("12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", sqlmock.AnyArg(), int64(999)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr:   services.ErrClienteNaoEncontrado,
			wantErrIs: true,
		},
		{
			nome:  "erro no Update é propagado",
			input: validClienteInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE clientes\s+SET cnpj = \?, razao_social = \?, segmento = \?, cidade = \?, uf = \?, bairro = \?, data_cadastro = \?\s+WHERE id = \?`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
		{
			nome:  "erro no GetByID pós-update é propagado",
			input: validClienteInput,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE clientes\s+SET cnpj = \?, razao_social = \?, segmento = \?, cidade = \?, uf = \?, bairro = \?, data_cadastro = \?\s+WHERE id = \?`).
					WithArgs("12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", sqlmock.AnyArg(), int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newClienteTestDB(t)
			tc.mock(mock)

			id := int64(1)
			if tc.nome == "id inexistente retorna ErrClienteNaoEncontrado" {
				id = 999
			}

			svc := services.NewClienteService(db, clienteTestCfg(true))
			c, err := svc.UpdateCliente(context.Background(), db, id, tc.input())

			if tc.wantErr != nil {
				assert.Nil(t, c)
				if tc.wantErrIs {
					assert.ErrorIs(t, err, tc.wantErr)
				} else {
					assert.Error(t, err)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, c)
				assert.Equal(t, int64(1), c.ID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

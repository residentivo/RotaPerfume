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

const vendedorGetColunasRegex = `id, nome, regiao, uf, data_admissao, data_desligamento, meta_mensal, created_at, updated_at FROM vendedores WHERE id = \? LIMIT 1`
const clienteResumoColunasRegex = `c\.id, c\.cnpj, c\.razao_social, c\.segmento, c\.cidade, c\.uf, ca\.id, ca\.data_inicio, ca\.data_fim`
const clienteResumoFromRegex = ` FROM carteiras ca JOIN clientes c ON c\.id = ca\.cliente_id`

func vendedorGetRows(id int64, nome string, dataDesligamento any) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_admissao", "data_desligamento", "meta_mensal", "created_at", "updated_at"}).
		AddRow(id, nome, "Sudeste", "SP", now, dataDesligamento, 5000.0, now, now)
}

func clienteResumoRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{"id", "cnpj", "razao_social", "segmento", "cidade", "uf", "carteira_id", "data_inicio", "data_fim"}).
		AddRow(int64(1), "11.111.111/0001-11", "Cliente A", "Varejo", "São Paulo", "SP", int64(10), now, nil).
		AddRow(int64(2), "22.222.222/0001-22", "Cliente B", "Atacado", "Campinas", "SP", int64(11), now, nil)
}

func validVendedorInput() services.VendedorInput {
	return services.VendedorInput{
		Nome:         "Novo Vendedor",
		Regiao:       "Sudeste",
		UF:           "SP",
		DataAdmissao: "2024-01-15",
		MetaMensal:   5000.0,
	}
}

func vendedorTestCfg(verbose bool) *config.Config {
	return &config.Config{Verbose: verbose}
}

func newVendedorTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

var vendedorColunasRegex = `id, nome, regiao, uf\s+FROM vendedores\s+WHERE data_desligamento IS NULL\s+ORDER BY nome ASC`

func vendedorRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "nome", "regiao", "uf"}).
		AddRow(int64(1), "João Vendedor", "Sudeste", "SP").
		AddRow(int64(2), "Maria Vendedora", "Sul", "RS")
}

// ---------------------------------------------------------------------------
// ListVendedores
// ---------------------------------------------------------------------------

func TestVendedorService_ListVendedores(t *testing.T) {
	testCases := []struct {
		nome    string
		verbose bool
		mock    func(mock sqlmock.Sqlmock)
		wantErr bool
		wantLen int
	}{
		{
			nome:    "sucesso - retorna vendedores ativos ordenados por nome",
			verbose: true,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorColunasRegex).
					WillReturnRows(vendedorRows())
			},
			wantLen: 2,
		},
		{
			nome:    "sucesso - lista vazia",
			verbose: false,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorColunasRegex).
					WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf"}))
			},
			wantLen: 0,
		},
		{
			nome:    "erro do repo é propagado",
			verbose: false,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorColunasRegex).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newVendedorTestDB(t)
			tc.mock(mock)

			svc := services.NewVendedorService(db, vendedorTestCfg(tc.verbose))
			vendedores, err := svc.ListVendedores(context.Background(), db)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, vendedores)
			} else {
				require.NoError(t, err)
				assert.Len(t, vendedores, tc.wantLen)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetVendedorDetalhe
// ---------------------------------------------------------------------------

func TestVendedorService_GetVendedorDetalhe(t *testing.T) {
	testCases := []struct {
		nome     string
		id       int64
		mock     func(mock sqlmock.Sqlmock)
		wantErr  error
		wantLen  int
		checkErr func(t *testing.T, err error)
	}{
		{
			nome: "vendedor com clientes",
			id:   1,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorGetColunasRegex).
					WithArgs(int64(1)).
					WillReturnRows(vendedorGetRows(1, "João Vendedor", nil))
				mock.ExpectQuery(clienteResumoColunasRegex + clienteResumoFromRegex + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
					WithArgs(int64(1)).
					WillReturnRows(clienteResumoRows())
			},
			wantLen: 2,
		},
		{
			nome: "vendedor sem clientes",
			id:   2,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorGetColunasRegex).
					WithArgs(int64(2)).
					WillReturnRows(vendedorGetRows(2, "Maria Vendedora", nil))
				mock.ExpectQuery(clienteResumoColunasRegex + clienteResumoFromRegex + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
					WithArgs(int64(2)).
					WillReturnRows(sqlmock.NewRows([]string{"id", "cnpj", "razao_social", "segmento", "cidade", "uf", "carteira_id", "data_inicio", "data_fim"}))
			},
			wantLen: 0,
		},
		{
			nome: "vendedor inexistente",
			id:   999,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorGetColunasRegex).
					WithArgs(int64(999)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: services.ErrVendedorNaoEncontrado,
		},
		{
			nome: "erro genérico no GetByID é propagado",
			id:   1,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorGetColunasRegex).
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.NotErrorIs(t, err, services.ErrVendedorNaoEncontrado)
			},
		},
		{
			nome: "erro do repository de carteira é propagado",
			id:   1,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorGetColunasRegex).
					WithArgs(int64(1)).
					WillReturnRows(vendedorGetRows(1, "João Vendedor", nil))
				mock.ExpectQuery(clienteResumoColunasRegex + clienteResumoFromRegex + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newVendedorTestDB(t)
			tc.mock(mock)

			svc := services.NewVendedorService(db, vendedorTestCfg(true))
			detalhe, err := svc.GetVendedorDetalhe(context.Background(), db, tc.id)

			switch {
			case tc.wantErr != nil:
				assert.Nil(t, detalhe)
				assert.ErrorIs(t, err, tc.wantErr)
			case tc.checkErr != nil:
				assert.Nil(t, detalhe)
				tc.checkErr(t, err)
			default:
				require.NoError(t, err)
				require.NotNil(t, detalhe)
				assert.Len(t, detalhe.Clientes, tc.wantLen)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Validações comuns a CreateVendedor/UpdateVendedor
// ---------------------------------------------------------------------------

func vendedorValidacaoTestCases() []struct {
	nome    string
	input   func() services.VendedorInput
	wantErr error
} {
	return []struct {
		nome    string
		input   func() services.VendedorInput
		wantErr error
	}{
		{
			nome: "nome vazio",
			input: func() services.VendedorInput {
				in := validVendedorInput()
				in.Nome = "   "
				return in
			},
			wantErr: services.ErrVendedorNomeObrigatorio,
		},
		{
			nome: "regiao vazia",
			input: func() services.VendedorInput {
				in := validVendedorInput()
				in.Regiao = "  "
				return in
			},
			wantErr: services.ErrVendedorRegiaoObrigatoria,
		},
		{
			nome: "uf com tamanho inválido",
			input: func() services.VendedorInput {
				in := validVendedorInput()
				in.UF = "SAO"
				return in
			},
			wantErr: services.ErrVendedorUFInvalida,
		},
		{
			nome: "meta_mensal negativa",
			input: func() services.VendedorInput {
				in := validVendedorInput()
				in.MetaMensal = -1
				return in
			},
			wantErr: services.ErrVendedorMetaMensalInvalida,
		},
		{
			nome: "data_admissao em formato inválido",
			input: func() services.VendedorInput {
				in := validVendedorInput()
				in.DataAdmissao = "15/01/2024"
				return in
			},
			wantErr: services.ErrVendedorDataAdmissaoInvalida,
		},
	}
}

// ---------------------------------------------------------------------------
// CreateVendedor
// ---------------------------------------------------------------------------

func TestVendedorService_CreateVendedor_Validacoes(t *testing.T) {
	for _, tc := range vendedorValidacaoTestCases() {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newVendedorTestDB(t)
			svc := services.NewVendedorService(db, vendedorTestCfg(true))
			v, err := svc.CreateVendedor(context.Background(), db, tc.input())
			assert.Nil(t, v)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVendedorService_CreateVendedor_DataAdmissaoVaziaUsaHoje(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	in := validVendedorInput()
	in.DataAdmissao = ""

	mock.ExpectExec(`INSERT INTO vendedores \(nome, regiao, uf, data_admissao, data_desligamento, meta_mensal\)`).
		WithArgs("Novo Vendedor", "Sudeste", "SP", sqlmock.AnyArg(), nil, 5000.0).
		WillReturnResult(sqlmock.NewResult(1, 1))

	svc := services.NewVendedorService(db, vendedorTestCfg(true))
	v, err := svc.CreateVendedor(context.Background(), db, in)

	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, int64(1), v.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorService_CreateVendedor_Sucesso(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	mock.ExpectExec(`INSERT INTO vendedores \(nome, regiao, uf, data_admissao, data_desligamento, meta_mensal\)`).
		WithArgs("Novo Vendedor", "Sudeste", "SP", sqlmock.AnyArg(), nil, 5000.0).
		WillReturnResult(sqlmock.NewResult(5, 1))

	svc := services.NewVendedorService(db, vendedorTestCfg(true))
	v, err := svc.CreateVendedor(context.Background(), db, validVendedorInput())

	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, int64(5), v.ID)
	assert.Equal(t, "Novo Vendedor", v.Nome)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorService_CreateVendedor_ErroDoRepo(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	mock.ExpectExec(`INSERT INTO vendedores \(nome, regiao, uf, data_admissao, data_desligamento, meta_mensal\)`).
		WillReturnError(sql.ErrConnDone)

	svc := services.NewVendedorService(db, vendedorTestCfg(false))
	v, err := svc.CreateVendedor(context.Background(), db, validVendedorInput())

	assert.Nil(t, v)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// UpdateVendedor
// ---------------------------------------------------------------------------

func TestVendedorService_UpdateVendedor_Validacoes(t *testing.T) {
	for _, tc := range vendedorValidacaoTestCases() {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newVendedorTestDB(t)
			svc := services.NewVendedorService(db, vendedorTestCfg(true))
			v, err := svc.UpdateVendedor(context.Background(), db, 1, tc.input())
			assert.Nil(t, v)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVendedorService_UpdateVendedor_DataAdmissaoVaziaEhErro(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	in := validVendedorInput()
	in.DataAdmissao = ""

	svc := services.NewVendedorService(db, vendedorTestCfg(true))
	v, err := svc.UpdateVendedor(context.Background(), db, 1, in)

	assert.Nil(t, v)
	assert.ErrorIs(t, err, services.ErrVendedorDataAdmissaoInvalida)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorService_UpdateVendedor_Sucesso(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	mock.ExpectExec(`UPDATE vendedores\s+SET nome = \?, regiao = \?, uf = \?, data_admissao = \?, meta_mensal = \?\s+WHERE id = \?`).
		WithArgs("Novo Vendedor", "Sudeste", "SP", sqlmock.AnyArg(), 5000.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(vendedorGetColunasRegex).
		WithArgs(int64(1)).
		WillReturnRows(vendedorGetRows(1, "Novo Vendedor", nil))

	svc := services.NewVendedorService(db, vendedorTestCfg(true))
	v, err := svc.UpdateVendedor(context.Background(), db, 1, validVendedorInput())

	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, "Novo Vendedor", v.Nome)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorService_UpdateVendedor_NaoEncontrado(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	mock.ExpectExec(`UPDATE vendedores\s+SET nome = \?, regiao = \?, uf = \?, data_admissao = \?, meta_mensal = \?\s+WHERE id = \?`).
		WithArgs("Novo Vendedor", "Sudeste", "SP", sqlmock.AnyArg(), 5000.0, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	svc := services.NewVendedorService(db, vendedorTestCfg(false))
	v, err := svc.UpdateVendedor(context.Background(), db, 999, validVendedorInput())

	assert.Nil(t, v)
	assert.ErrorIs(t, err, services.ErrVendedorNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorService_UpdateVendedor_ErroGenericoDoRepo(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	mock.ExpectExec(`UPDATE vendedores\s+SET nome = \?, regiao = \?, uf = \?, data_admissao = \?, meta_mensal = \?\s+WHERE id = \?`).
		WillReturnError(sql.ErrConnDone)

	svc := services.NewVendedorService(db, vendedorTestCfg(false))
	v, err := svc.UpdateVendedor(context.Background(), db, 1, validVendedorInput())

	assert.Nil(t, v)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, services.ErrVendedorNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorService_UpdateVendedor_ErroNoGetByIDApósUpdate(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	mock.ExpectExec(`UPDATE vendedores\s+SET nome = \?, regiao = \?, uf = \?, data_admissao = \?, meta_mensal = \?\s+WHERE id = \?`).
		WithArgs("Novo Vendedor", "Sudeste", "SP", sqlmock.AnyArg(), 5000.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(vendedorGetColunasRegex).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	svc := services.NewVendedorService(db, vendedorTestCfg(false))
	v, err := svc.UpdateVendedor(context.Background(), db, 1, validVendedorInput())

	assert.Nil(t, v)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// DeleteVendedor
// ---------------------------------------------------------------------------

func TestVendedorService_DeleteVendedor_Sucesso(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(vendedorGetColunasRegex).
		WithArgs(int64(1)).
		WillReturnRows(vendedorGetRows(1, "João Vendedor", time.Now()))

	svc := services.NewVendedorService(db, vendedorTestCfg(true))
	v, err := svc.DeleteVendedor(context.Background(), db, 1)

	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, int64(1), v.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorService_DeleteVendedor_NaoEncontrado(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	svc := services.NewVendedorService(db, vendedorTestCfg(false))
	v, err := svc.DeleteVendedor(context.Background(), db, 999)

	assert.Nil(t, v)
	assert.ErrorIs(t, err, services.ErrVendedorNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVendedorService_DeleteVendedor_ErroGenericoDoRepo(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WillReturnError(sql.ErrConnDone)

	svc := services.NewVendedorService(db, vendedorTestCfg(false))
	v, err := svc.DeleteVendedor(context.Background(), db, 1)

	assert.Nil(t, v)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, services.ErrVendedorNaoEncontrado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// VincularCliente
// ---------------------------------------------------------------------------

const vendedorExistsByIDRegex = `SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`
const clienteGetByIDRegex = `SELECT .+ FROM clientes WHERE id = \? LIMIT 1`
const carteiraGetVinculoAtivoByClienteIDRegex = `SELECT id, carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`
const carteiraGetVinculoAtivoRegex = `SELECT id, carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`
const carteiraEncerrarVinculoRegex = `UPDATE carteiras SET data_fim = \? WHERE id = \?`
const carteiraNextCarteiraIDOrigemRegex = `SELECT COALESCE\(MAX\(carteira_id_origem\), 0\) \+ 1 FROM carteiras`
const carteiraCreateRegex = `INSERT INTO carteiras \(carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim\)\s+VALUES \(\?, \?, \?, \?, \?\)`
const carteiraGetVinculoByClienteVendedorDataRegex = `SELECT id, carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE cliente_id = \? AND vendedor_id = \? AND data_inicio = \?\s+LIMIT 1`
const carteiraReativarVinculoRegex = `UPDATE carteiras SET data_fim = NULL WHERE id = \?`

func clienteGetByIDRows(id int64, razaoSocial string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
		"data_cadastro", "ativo", "created_at", "updated_at",
	}).AddRow(id, int64(100), "11.111.111/0001-11", razaoSocial, "Varejo", "São Paulo", "SP", "Centro", now, true, now, now)
}

func carteiraVinculoRow(id, carteiraIDOrigem, clienteID, vendedorID int64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "carteira_id_origem", "cliente_id", "vendedor_id", "data_inicio", "data_fim", "created_at", "updated_at",
	}).AddRow(id, carteiraIDOrigem, clienteID, vendedorID, now, nil, now, now)
}

// carteiraVinculoRowEncerrado simula um vínculo de carteira já encerrado
// (data_fim preenchido), usado nos cenários de revinculação no mesmo dia.
func carteiraVinculoRowEncerrado(id, carteiraIDOrigem, clienteID, vendedorID int64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "carteira_id_origem", "cliente_id", "vendedor_id", "data_inicio", "data_fim", "created_at", "updated_at",
	}).AddRow(id, carteiraIDOrigem, clienteID, vendedorID, now, now, now, now)
}

func TestVendedorService_VincularCliente(t *testing.T) {
	testCases := []struct {
		nome        string
		vendedorID  int64
		clienteID   int64
		mock        func(mock sqlmock.Sqlmock)
		wantErr     error
		checkErr    func(t *testing.T, err error)
		wantVendID  int64
	}{
		{
			nome:       "sucesso simples - cliente sem vínculo anterior",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorExistsByIDRegex).WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(clienteGetByIDRegex).WithArgs(int64(10)).
					WillReturnRows(clienteGetByIDRows(10, "Cliente A"))
				mock.ExpectQuery(carteiraGetVinculoAtivoByClienteIDRegex).WithArgs(int64(10)).
					WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(carteiraGetVinculoByClienteVendedorDataRegex).WithArgs(int64(10), int64(1), sqlmock.AnyArg()).
					WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(carteiraNextCarteiraIDOrigemRegex).
					WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(501))
				mock.ExpectExec(carteiraCreateRegex).
					WillReturnResult(sqlmock.NewResult(7, 1))
			},
			wantVendID: 1,
		},
		{
			nome:       "sucesso com transferência automática de outro vendedor",
			vendedorID: 2,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorExistsByIDRegex).WithArgs(int64(2)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(clienteGetByIDRegex).WithArgs(int64(10)).
					WillReturnRows(clienteGetByIDRows(10, "Cliente A"))
				mock.ExpectQuery(carteiraGetVinculoAtivoByClienteIDRegex).WithArgs(int64(10)).
					WillReturnRows(carteiraVinculoRow(3, 300, 10, 1))
				mock.ExpectExec(carteiraEncerrarVinculoRegex).
					WithArgs(sqlmock.AnyArg(), int64(3)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(carteiraGetVinculoByClienteVendedorDataRegex).WithArgs(int64(10), int64(2), sqlmock.AnyArg()).
					WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(carteiraNextCarteiraIDOrigemRegex).
					WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(501))
				mock.ExpectExec(carteiraCreateRegex).
					WillReturnResult(sqlmock.NewResult(8, 1))
			},
			wantVendID: 2,
		},
		{
			nome:       "idempotente - já vinculado ao mesmo vendedor",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorExistsByIDRegex).WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(clienteGetByIDRegex).WithArgs(int64(10)).
					WillReturnRows(clienteGetByIDRows(10, "Cliente A"))
				mock.ExpectQuery(carteiraGetVinculoAtivoByClienteIDRegex).WithArgs(int64(10)).
					WillReturnRows(carteiraVinculoRow(3, 300, 10, 1))
			},
			wantVendID: 1,
		},
		{
			nome:       "vendedor inexistente",
			vendedorID: 999,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorExistsByIDRegex).WithArgs(int64(999)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: services.ErrVendedorNaoEncontrado,
		},
		{
			nome:       "cliente inexistente",
			vendedorID: 1,
			clienteID:  999,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorExistsByIDRegex).WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(clienteGetByIDRegex).WithArgs(int64(999)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: services.ErrClienteNaoEncontrado,
		},
		{
			nome:       "erro de repository - ExistsByID vendedor",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorExistsByIDRegex).WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.NotErrorIs(t, err, services.ErrVendedorNaoEncontrado)
			},
		},
		{
			nome:       "erro de repository - GetVinculoAtivoByClienteID",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorExistsByIDRegex).WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(clienteGetByIDRegex).WithArgs(int64(10)).
					WillReturnRows(clienteGetByIDRows(10, "Cliente A"))
				mock.ExpectQuery(carteiraGetVinculoAtivoByClienteIDRegex).WithArgs(int64(10)).
					WillReturnError(sql.ErrConnDone)
			},
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			nome:       "erro de repository - Create novo vínculo",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorExistsByIDRegex).WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(clienteGetByIDRegex).WithArgs(int64(10)).
					WillReturnRows(clienteGetByIDRows(10, "Cliente A"))
				mock.ExpectQuery(carteiraGetVinculoAtivoByClienteIDRegex).WithArgs(int64(10)).
					WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(carteiraGetVinculoByClienteVendedorDataRegex).WithArgs(int64(10), int64(1), sqlmock.AnyArg()).
					WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(carteiraNextCarteiraIDOrigemRegex).
					WillReturnRows(sqlmock.NewRows([]string{"next"}).AddRow(501))
				mock.ExpectExec(carteiraCreateRegex).
					WillReturnError(sql.ErrConnDone)
			},
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			nome:       "revincular no mesmo dia - vinculo encerrado é reativado",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorExistsByIDRegex).WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(clienteGetByIDRegex).WithArgs(int64(10)).
					WillReturnRows(clienteGetByIDRows(10, "Cliente A"))
				mock.ExpectQuery(carteiraGetVinculoAtivoByClienteIDRegex).WithArgs(int64(10)).
					WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(carteiraGetVinculoByClienteVendedorDataRegex).WithArgs(int64(10), int64(1), sqlmock.AnyArg()).
					WillReturnRows(carteiraVinculoRowEncerrado(3, 300, 10, 1))
				mock.ExpectExec(carteiraReativarVinculoRegex).
					WithArgs(int64(3)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantVendID: 1,
		},
		{
			nome:       "erro de repository - GetVinculoByClienteVendedorData",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorExistsByIDRegex).WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(clienteGetByIDRegex).WithArgs(int64(10)).
					WillReturnRows(clienteGetByIDRows(10, "Cliente A"))
				mock.ExpectQuery(carteiraGetVinculoAtivoByClienteIDRegex).WithArgs(int64(10)).
					WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(carteiraGetVinculoByClienteVendedorDataRegex).WithArgs(int64(10), int64(1), sqlmock.AnyArg()).
					WillReturnError(sql.ErrConnDone)
			},
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			nome:       "erro de repository - ReativarVinculo",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(vendedorExistsByIDRegex).WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(clienteGetByIDRegex).WithArgs(int64(10)).
					WillReturnRows(clienteGetByIDRows(10, "Cliente A"))
				mock.ExpectQuery(carteiraGetVinculoAtivoByClienteIDRegex).WithArgs(int64(10)).
					WillReturnError(sql.ErrNoRows)
				mock.ExpectQuery(carteiraGetVinculoByClienteVendedorDataRegex).WithArgs(int64(10), int64(1), sqlmock.AnyArg()).
					WillReturnRows(carteiraVinculoRowEncerrado(3, 300, 10, 1))
				mock.ExpectExec(carteiraReativarVinculoRegex).
					WithArgs(int64(3)).
					WillReturnError(sql.ErrConnDone)
			},
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newVendedorTestDB(t)
			tc.mock(mock)

			svc := services.NewVendedorService(db, vendedorTestCfg(true))
			resumo, err := svc.VincularCliente(context.Background(), db, tc.vendedorID, tc.clienteID)

			switch {
			case tc.wantErr != nil:
				assert.Nil(t, resumo)
				assert.ErrorIs(t, err, tc.wantErr)
			case tc.checkErr != nil:
				assert.Nil(t, resumo)
				tc.checkErr(t, err)
			default:
				require.NoError(t, err)
				require.NotNil(t, resumo)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// DesvincularCliente
// ---------------------------------------------------------------------------

func TestVendedorService_DesvincularCliente(t *testing.T) {
	testCases := []struct {
		nome       string
		vendedorID int64
		clienteID  int64
		mock       func(mock sqlmock.Sqlmock)
		wantErr    error
		checkErr   func(t *testing.T, err error)
	}{
		{
			nome:       "sucesso",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(carteiraGetVinculoAtivoRegex).WithArgs(int64(1), int64(10)).
					WillReturnRows(carteiraVinculoRow(3, 300, 10, 1))
				mock.ExpectExec(carteiraEncerrarVinculoRegex).
					WithArgs(sqlmock.AnyArg(), int64(3)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			nome:       "vínculo não encontrado",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(carteiraGetVinculoAtivoRegex).WithArgs(int64(1), int64(10)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: services.ErrVinculoNaoEncontrado,
		},
		{
			nome:       "vendedor/cliente inexistente (sem vínculo ativo)",
			vendedorID: 999,
			clienteID:  888,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(carteiraGetVinculoAtivoRegex).WithArgs(int64(999), int64(888)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: services.ErrVinculoNaoEncontrado,
		},
		{
			nome:       "erro de repository - GetVinculoAtivo",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(carteiraGetVinculoAtivoRegex).WithArgs(int64(1), int64(10)).
					WillReturnError(sql.ErrConnDone)
			},
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.NotErrorIs(t, err, services.ErrVinculoNaoEncontrado)
			},
		},
		{
			nome:       "erro de repository - EncerrarVinculo",
			vendedorID: 1,
			clienteID:  10,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(carteiraGetVinculoAtivoRegex).WithArgs(int64(1), int64(10)).
					WillReturnRows(carteiraVinculoRow(3, 300, 10, 1))
				mock.ExpectExec(carteiraEncerrarVinculoRegex).
					WillReturnError(sql.ErrConnDone)
			},
			checkErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.NotErrorIs(t, err, services.ErrVinculoNaoEncontrado)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newVendedorTestDB(t)
			tc.mock(mock)

			svc := services.NewVendedorService(db, vendedorTestCfg(true))
			err := svc.DesvincularCliente(context.Background(), db, tc.vendedorID, tc.clienteID)

			switch {
			case tc.wantErr != nil:
				assert.ErrorIs(t, err, tc.wantErr)
			case tc.checkErr != nil:
				tc.checkErr(t, err)
			default:
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVendedorService_DeleteVendedor_ErroNoGetByIDApósUpdate(t *testing.T) {
	db, mock := newVendedorTestDB(t)

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(vendedorGetColunasRegex).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	svc := services.NewVendedorService(db, vendedorTestCfg(false))
	v, err := svc.DeleteVendedor(context.Background(), db, 1)

	assert.Nil(t, v)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

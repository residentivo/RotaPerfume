package handlers_test

// CHORE-06 (Lote 25) pelo HTTP: a duplicidade vira 409 com a mensagem do
// domínio (SKU, estoque do dia, vínculo de carteira); 1062 em outro índice
// (PRIMARY) continua 500 "erro interno". Com o vlog ligado, nenhum log leva
// o valor duplicado nem o texto do driver (padrão do ARQ-04).

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
)

func chore06Dup(chave string) error {
	return &mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'VALOR-SECRETO-L25' for key '" + chave + "'"}
}

func TestCHORE06_HTTP_Duplicidade(t *testing.T) {
	casos := []struct {
		nome    string
		metodo  string
		path    string
		payload map[string]any
		prepara func(m sqlmock.Sqlmock, dup error)
		chave   string
		want    int
		msg     string
		logOp   string
	}{
		{"POST produtos: uk_produtos_sku → 409", http.MethodPost, "/api/produtos", nil, func(m sqlmock.Sqlmock, dup error) {
			m.ExpectExec(`INSERT INTO produtos`).WillReturnError(dup)
		}, "produtos.uk_produtos_sku", http.StatusConflict, "sku já cadastrado", "[produtos] sku duplicado recusado (uk_produtos_sku)"},
		{"POST produtos: PRIMARY → 500", http.MethodPost, "/api/produtos", nil, func(m sqlmock.Sqlmock, dup error) {
			m.ExpectExec(`INSERT INTO produtos`).WillReturnError(dup)
		}, "PRIMARY", http.StatusInternalServerError, "erro interno", ""},
		{"POST estoque: uk_estoque_data_sku → 409", http.MethodPost, "/api/estoque", validEstoquePayload(), func(m sqlmock.Sqlmock, dup error) {
			m.ExpectQuery(`SELECT 1 FROM produtos WHERE sku = \? LIMIT 1`).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			m.ExpectExec(`INSERT INTO estoque`).WillReturnError(dup)
		}, "estoque.uk_estoque_data_sku", http.StatusConflict, "registro de estoque já cadastrado para este sku nesta data",
			"[estoque] registro duplicado recusado (uk_estoque_data_sku)"},
		{"POST estoque: PRIMARY → 500", http.MethodPost, "/api/estoque", validEstoquePayload(), func(m sqlmock.Sqlmock, dup error) {
			m.ExpectQuery(`SELECT 1 FROM produtos WHERE sku = \? LIMIT 1`).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			m.ExpectExec(`INSERT INTO estoque`).WillReturnError(dup)
		}, "PRIMARY", http.StatusInternalServerError, "erro interno", ""},
		{"POST vendedores/{id}/clientes: corrida no vínculo do dia → 409", http.MethodPost, "/api/vendedores/1/clientes", map[string]any{"cliente_id": 10},
			func(m sqlmock.Sqlmock, dup error) {
				m.ExpectQuery(vendedorExistsByIDRegexH).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				m.ExpectQuery(clienteGetByIDRegexH).WithArgs(int64(10)).WillReturnRows(clienteGetByIDRowsH(10, "Cliente A"))
				expectTravaClienteVinculoH(m, 10)
				m.ExpectQuery(carteiraGetVinculoAtivoByClienteIDRegexH).WithArgs(int64(10)).WillReturnError(sql.ErrNoRows)
				m.ExpectQuery(carteiraGetVinculoByClienteVendedorDataRegexH).WithArgs(int64(10), int64(1), sqlmock.AnyArg()).WillReturnError(sql.ErrNoRows)
				m.ExpectExec(carteiraCreateRegexH).WillReturnError(dup)
				m.ExpectRollback()
			}, "carteiras.uq_carteiras_cliente_vendedor_data", http.StatusConflict, "vínculo entre cliente e vendedor já cadastrado nesta data",
			"[carteiras] vinculo duplicado recusado"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			out := capturarVlogLigado(t)
			c.prepara(mock, chore06Dup(c.chave))
			payload := c.payload
			if payload == nil {
				payload = validProdutoPayload()
			}
			status, body := doReq(t, c.metodo, server.URL+c.path, generateToken(t, testCfg(), 1, "admin"), payload)
			assert.Equal(t, c.want, status)
			assert.Equal(t, c.msg, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())

			saida := out.String()
			for _, v := range []string{"VALOR-SECRETO-L25", "Duplicate entry '", "SKU-001", "2024-06-01"} {
				assert.NotContains(t, saida, v, "o log não leva o valor duplicado")
			}
			if c.logOp != "" {
				assert.Contains(t, saida, c.logOp)
			}
		})
	}
}

// O 409 do CNPJ e do e-mail não mudam (regressão do contrato).
func TestCHORE06_HTTP_EmailDuplicado_ContinuaConflito(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	out := capturarVlogLigado(t)
	mock.ExpectExec(`INSERT INTO usuarios`).WillReturnError(chore06Dup("usuarios.uk_usuarios_email"))
	status, body := doReq(t, http.MethodPost, server.URL+"/api/usuarios", generateToken(t, testCfg(), 1, "admin"),
		map[string]any{"nome": "Ana", "email": "segredo.l25@rp.com", "role": "admin"})
	assert.Equal(t, http.StatusConflict, status, "body: %v", body)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, out.String(), "[usuarios] email duplicado recusado (uk_usuarios_email)")
	for _, v := range []string{"segredo.l25", "VALOR-SECRETO-L25"} {
		assert.NotContains(t, out.String(), v)
	}
}

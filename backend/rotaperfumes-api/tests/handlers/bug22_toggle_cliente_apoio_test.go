package handlers_test

// Apoio do BUG-22 (Lote 31): a sequência SQL do PATCH /api/clientes/{id}/inativar
// depois do toggle atômico. Numa tx: trava o cliente (FOR UPDATE), confere a
// carteira sob o lock (só no escopo de vendedor), UPDATE (NOT ativo no
// toggle, valor no body explícito) e relê para a resposta.

import (
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

const (
	reTravarClienteH   = `SELECT cliente_id_origem FROM clientes WHERE cliente_id_origem = \? FOR UPDATE`
	reToggleClienteH   = `UPDATE clientes SET ativo = NOT ativo WHERE cliente_id_origem = \?`
	reDefinirClienteH  = `UPDATE clientes SET ativo = \? WHERE cliente_id_origem = \?`
	semVendedorToggleH = int64(0)
)

// clienteRowsAtivoH é o clienteRowsForHandler com o ativo informado (a
// releitura devolve o valor gravado).
func clienteRowsAtivoH(id int64, cnpj string, ativo bool) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
		"data_cadastro", "ativo", "created_at", "updated_at",
	}).AddRow(id, cnpj, "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", now, ativo, now, now)
}

// expectToggleClienteTxH registra a tx do toggle. vendedorID 0 = admin (sem
// checagem de carteira sob o lock).
func expectToggleClienteTxH(mock sqlmock.Sqlmock, id, vendedorID int64, toggle, valorFinal bool, cnpj string) {
	mock.ExpectBegin()
	mock.ExpectQuery(reTravarClienteH).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"cliente_id_origem"}).AddRow(id))
	if vendedorID != semVendedorToggleH {
		expectCarteiraAtivaH(mock, vendedorID, id)
	}
	if toggle {
		mock.ExpectExec(reToggleClienteH).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
	} else {
		mock.ExpectExec(reDefinirClienteH).WithArgs(valorFinal, id).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectQuery(reSelectClienteH).WithArgs(id).WillReturnRows(clienteRowsAtivoH(id, cnpj, valorFinal))
	mock.ExpectCommit()
}

// expectToggleUsuarioTxH registra a tx do PATCH /api/usuarios/{id}/inativar
// (BUG-22): UPDATE (NOT ativo no toggle; valor, com o corte se false, no
// body explícito) → releitura → se ficou inativo, revogação dos refresh
// tokens (SEC-06) e corte de sessão (SEC-08) → commit.
func expectToggleUsuarioTxH(mock sqlmock.Sqlmock, id int64, toggle, valorFinal bool) {
	mock.ExpectBegin()
	expectUpdateAtivoUsuarioH(mock, id, toggle, valorFinal)
	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(id).WillReturnRows(usuarioRowsForHandler(id, valorFinal))
	if !valorFinal {
		mock.ExpectExec(revokeAllByUserRegex).
			WithArgs(sqlmock.AnyArg(), "inativacao", id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(invalidarSessoesSQL).WithArgs(sqlmock.AnyArg(), id).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()
}

// expectUpdateAtivoUsuarioH é só o UPDATE do toggle de usuário.
func expectUpdateAtivoUsuarioH(mock sqlmock.Sqlmock, id int64, toggle, valorFinal bool) *sqlmock.ExpectedExec {
	switch {
	case toggle:
		return mock.ExpectExec(`UPDATE usuarios SET ativo = NOT ativo WHERE id = \?`).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
	case valorFinal:
		return mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).WithArgs(true, id).WillReturnResult(sqlmock.NewResult(0, 1))
	default:
		return mock.ExpectExec(`UPDATE usuarios SET ativo = \?, tokens_validos_desde = \? WHERE id = \?`).
			WithArgs(false, sqlmock.AnyArg(), id).WillReturnResult(sqlmock.NewResult(0, 1))
	}
}

// BUG-22: a carteira é conferida de novo sob o lock do cliente. Se o
// cliente foi transferido entre a checagem inicial e o lock, o vendedor
// antigo recebe 404 e nada é gravado.
func TestBUG22_ToggleCliente_TransferidoAntesDoLock_404SemUpdate(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	expectEscopoUsuarioH(mock, 2, 10)
	expectCarteiraAtivaH(mock, 10, 1) // checagem antes do body: ainda na carteira
	mock.ExpectBegin()
	mock.ExpectQuery(reTravarClienteH).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"cliente_id_origem"}).AddRow(int64(1)))
	mock.ExpectQuery(`FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL`).
		WithArgs(int64(10), int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"carteira_id_origem", "cliente_id", "vendedor_id", "data_inicio", "data_fim", "created_at", "updated_at"}))
	mock.ExpectRollback()

	status, body := doClienteReq(t, server.URL, "PATCH", "/api/clientes/1/inativar", nil)

	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, msgClienteNaoEncontradoH, body["error"])
	assert.NoError(t, mock.ExpectationsWereMet(), "nenhum UPDATE depois da carteira recusada sob o lock")
}

// BUG-22: o toggle de usuário inexistente continua 404 (o UPDATE atômico
// não casa linha) e a tx é desfeita.
func TestBUG22_ToggleUsuario_SemLinha404(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE usuarios SET ativo = \?, tokens_validos_desde = \? WHERE id = \?`).
		WithArgs(false, sqlmock.AnyArg(), int64(77)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	status, body := patchInativarUsuario(t, server.URL+"/api/usuarios/77/inativar", map[string]any{"ativo": false})

	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "usuário não encontrado", body["error"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

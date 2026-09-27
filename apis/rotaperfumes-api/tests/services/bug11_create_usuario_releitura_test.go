package services_test

// BUG-11: UsuarioRepository.Create passou a fazer só o INSERT (preenche u.ID)
// e UsuarioService.CreateUsuario relê o registro depois de enviar o e-mail com
// a senha inicial. Ordem garantida: INSERT -> e-mail -> SELECT (GetByID).
// Falha só na releitura não vira erro: loga "[usuarios] criado, mas falhou a
// releitura: id=%d: %v" e devolve o objeto em memória, com email_enviado
// refletindo o envio real. Mesmo padrão do BUG-09 (bug09_creates_timestamps_test.go).

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/models"
)

const (
	bug11ReInsert  = `INSERT INTO usuarios \(nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha\)`
	bug11ReGetByID = `SELECT ` + usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`
	bug11MsgFalha  = "[usuarios] criado, mas falhou a releitura: id="
)

// bug11Linha registra, em ordem, os eventos observados (SQL e e-mail).
type bug11Linha struct {
	mu      sync.Mutex
	eventos []string
}

func (l *bug11Linha) marcar(ev string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.eventos = append(l.eventos, ev)
}

func (l *bug11Linha) lista() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.eventos...)
}

// bug11ArgEvento é um sqlmock.Argument que confere o valor e registra o
// momento em que o driver recebeu o comando (prova de ordem).
type bug11ArgEvento struct {
	linha    *bug11Linha
	evento   string
	esperado driver.Value
}

func (a bug11ArgEvento) Match(v driver.Value) bool {
	if v != a.esperado {
		return false
	}
	a.linha.marcar(a.evento)
	return true
}

// bug11Email é um mock de EmailService que registra o momento do envio.
type bug11Email struct {
	linha    *bug11Linha
	err      error
	chamadas int
	destino  string
	nome     string
	senha    string
}

func (e *bug11Email) EnviarSenhaInicial(_ context.Context, destinatario, nomeUsuario, senha string) error {
	e.chamadas++
	e.destino, e.nome, e.senha = destinatario, nomeUsuario, senha
	e.linha.marcar("email")
	return e.err
}

// bug11CapturarLog redireciona o log padrão para um buffer durante o teste.
func bug11CapturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	anterior := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(anterior) })
	return &buf
}

type bug11Input = struct {
	Nome       string
	Email      string
	Role       string
	IDVendedor *int64
}

// bug11EsperarInsert registra a validação do vendedor (se houver) e o INSERT,
// marcando o momento do INSERT na linha de eventos.
func bug11EsperarInsert(mock sqlmock.Sqlmock, linha *bug11Linha, id int64, idVendedor *int64) {
	var vend any = nil
	if idVendedor != nil {
		vend = *idVendedor
		mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
			WithArgs(*idVendedor).
			WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	}
	mock.ExpectExec(bug11ReInsert).
		WithArgs(bug11ArgEvento{linha, "insert", "Fulano"}, "fulano@test.com", sqlmock.AnyArg(), models.RoleNormal, vend, true, true).
		WillReturnResult(sqlmock.NewResult(id, 1))
}

func TestBUG11_CreateUsuario_ReleituraOK_DevolveLinhaDoBanco(t *testing.T) {
	criado := time.Date(2026, 9, 26, 10, 30, 0, 0, time.Local)
	atualizado := time.Date(2026, 9, 26, 10, 31, 0, 0, time.Local)
	idVendedor := int64(5)

	casos := []struct {
		nome         string
		idVendedor   *int64
		vendedorNome any
		emailErr     error
		verbose      bool
	}{
		{"sem vendedor/email ok/quiet", nil, nil, nil, false},
		{"sem vendedor/email ok/verbose", nil, nil, nil, true},
		{"com vendedor/email ok/quiet", &idVendedor, "Vendedor Do Banco", nil, false},
		{"com vendedor/email ok/verbose", &idVendedor, "Vendedor Do Banco", nil, true},
		{"com vendedor/email falhou", &idVendedor, "Vendedor Do Banco", errors.New("smtp fora"), false},
		{"sem vendedor/email falhou/verbose", nil, nil, errors.New("smtp fora"), true},
	}

	for _, tc := range casos {
		tc := tc
		t.Run(tc.nome, func(t *testing.T) {
			logs := bug11CapturarLog(t)
			db, mock := newUsuarioTestDB(t)
			linha := &bug11Linha{}
			const id = int64(7)

			bug11EsperarInsert(mock, linha, id, tc.idVendedor)
			var vendCol any = nil
			if tc.idVendedor != nil {
				vendCol = *tc.idVendedor
			}
			mock.ExpectQuery(bug11ReGetByID).
				WithArgs(bug11ArgEvento{linha, "select", id}).
				WillReturnRows(sqlmock.NewRows(usuarioColunasHeader()).
					AddRow(id, "Fulano", "fulano@test.com", "hash-do-banco", models.RoleNormal, vendCol, true, true, criado, atualizado, nil, tc.vendedorNome))

			email := &bug11Email{linha: linha, err: tc.emailErr}
			svc := services.NewUsuarioService(db, usuarioTestCfg(tc.verbose), email)
			u, enviado, err := svc.CreateUsuario(context.Background(), db, bug11Input{
				Nome: "Fulano", Email: "  Fulano@Test.COM ", Role: models.RoleNormal, IDVendedor: tc.idVendedor,
			})

			require.NoError(t, err)
			require.NotNil(t, u)
			assert.Equal(t, tc.emailErr == nil, enviado, "email_enviado reflete o envio real")
			assert.Equal(t, id, u.ID)
			assert.True(t, u.CreatedAt.Equal(criado), "created_at deve vir do GetByID")
			assert.True(t, u.UpdatedAt.Equal(atualizado), "updated_at deve vir do GetByID")
			assert.Equal(t, "hash-do-banco", u.PasswordHash, "resposta deve ser a linha relida")
			if tc.vendedorNome != nil {
				require.NotNil(t, u.VendedorNome, "vendedor_nome só existe na releitura (JOIN)")
				assert.Equal(t, tc.vendedorNome, *u.VendedorNome)
			} else {
				assert.Nil(t, u.VendedorNome)
			}

			assert.Equal(t, []string{"insert", "email", "select"}, linha.lista(), "ordem INSERT -> e-mail -> SELECT")
			assert.Equal(t, 1, email.chamadas)
			assert.Equal(t, "fulano@test.com", email.destino, "e-mail normalizado")
			assert.Equal(t, "Fulano", email.nome)
			assert.Len(t, email.senha, 16, "senha gerada enviada ao usuário")
			assert.NotContains(t, logs.String(), email.senha, "senha nunca vai para o log")
			assert.NotContains(t, logs.String(), bug11MsgFalha)
			if tc.verbose {
				assert.Contains(t, logs.String(), fmt.Sprintf("[usuarios] criado: id=%d email=fulano@test.com role=normal email_enviado=%t", id, enviado))
			} else {
				assert.NotContains(t, logs.String(), "[usuarios] criado:")
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBUG11_CreateUsuario_FalhaNaReleitura_DevolveObjetoEmMemoriaSemErro(t *testing.T) {
	idVendedor := int64(5)
	falhas := []struct {
		nome string
		err  error
	}{
		{"erro de conexão", errors.New("db down")},
		{"linha não encontrada", sql.ErrNoRows},
		{"conexão fechada", sql.ErrConnDone},
	}
	emails := []struct {
		nome string
		err  error
	}{
		{"email ok", nil},
		{"email falhou", errors.New("smtp fora")},
	}
	vendedores := []struct {
		nome string
		id   *int64
	}{
		{"sem vendedor", nil},
		{"com vendedor", &idVendedor},
	}

	for _, f := range falhas {
		for _, e := range emails {
			for _, v := range vendedores {
				f, e, v := f, e, v
				t.Run(f.nome+"/"+e.nome+"/"+v.nome, func(t *testing.T) {
					logs := bug11CapturarLog(t)
					db, mock := newUsuarioTestDB(t)
					linha := &bug11Linha{}
					const id = int64(42)

					bug11EsperarInsert(mock, linha, id, v.id)
					mock.ExpectQuery(bug11ReGetByID).
						WithArgs(bug11ArgEvento{linha, "select", id}).
						WillReturnError(f.err)

					email := &bug11Email{linha: linha, err: e.err}
					svc := services.NewUsuarioService(db, usuarioTestCfg(false), email)
					u, enviado, err := svc.CreateUsuario(context.Background(), db, bug11Input{
						Nome: "Fulano", Email: "fulano@test.com", Role: models.RoleNormal, IDVendedor: v.id,
					})

					require.NoError(t, err, "INSERT confirmado não pode virar erro por falha só na releitura")
					require.NotNil(t, u)
					assert.Equal(t, e.err == nil, enviado, "email_enviado reflete o envio real")
					assert.Equal(t, 1, email.chamadas, "e-mail sai mesmo com a releitura falhando")
					assert.Equal(t, "fulano@test.com", email.destino)

					// Fallback: objeto em memória.
					assert.Equal(t, id, u.ID, "fallback mantém o id gerado no INSERT")
					assert.Equal(t, "Fulano", u.Nome)
					assert.Equal(t, "fulano@test.com", u.Email)
					assert.Equal(t, models.RoleNormal, u.Role)
					assert.Equal(t, v.id, u.IDVendedor)
					assert.True(t, u.Ativo)
					assert.True(t, u.DeveTrocarSenha)
					assert.True(t, u.CreatedAt.IsZero(), "fallback é o objeto em memória")
					assert.True(t, u.UpdatedAt.IsZero(), "fallback é o objeto em memória")
					assert.Nil(t, u.VendedorNome, "sem JOIN no fallback")

					assert.Equal(t, []string{"insert", "email", "select"}, linha.lista(), "ordem INSERT -> e-mail -> SELECT")
					assert.Contains(t, logs.String(), fmt.Sprintf("%s%d: ", bug11MsgFalha, id))
					if !errors.Is(f.err, sql.ErrNoRows) { // ErrNoRows vira ErrNotFound no repo
						assert.Contains(t, logs.String(), f.err.Error())
					}
					assert.NotContains(t, logs.String(), email.senha, "senha nunca vai para o log")
					assert.NoError(t, mock.ExpectationsWereMet())
				})
			}
		}
	}
}

func TestBUG11_CreateUsuario_FalhaNoInsert_NaoEnviaEmailNemRele(t *testing.T) {
	casos := []struct {
		nome        string
		configurar  func(e *sqlmock.ExpectedExec)
		errEsperado error // nil = qualquer erro que não seja ErrEmailDuplicado
	}{
		{
			"duplicado (sentinela do repo)",
			func(e *sqlmock.ExpectedExec) {
				e.WillReturnError(errors.New("Error 1062 (23000): Duplicate entry 'fulano@test.com' for key 'usuarios.email'"))
			},
			services.ErrEmailDuplicado,
		},
		{
			"duplicado (só Duplicate entry)",
			func(e *sqlmock.ExpectedExec) { e.WillReturnError(errors.New("Duplicate entry 'x' for key 'email'")) },
			services.ErrEmailDuplicado,
		},
		{
			"erro genérico no INSERT",
			func(e *sqlmock.ExpectedExec) { e.WillReturnError(errors.New("db down")) },
			nil,
		},
		{
			"erro no LastInsertId",
			func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewErrorResult(errors.New("sem id"))) },
			nil,
		},
	}

	for _, tc := range casos {
		tc := tc
		t.Run(tc.nome, func(t *testing.T) {
			logs := bug11CapturarLog(t)
			db, mock := newUsuarioTestDB(t)
			linha := &bug11Linha{}
			exp := mock.ExpectExec(bug11ReInsert).
				WithArgs(bug11ArgEvento{linha, "insert", "Fulano"}, "fulano@test.com", sqlmock.AnyArg(), models.RoleNormal, nil, true, true)
			tc.configurar(exp)

			email := &bug11Email{linha: linha}
			svc := services.NewUsuarioService(db, usuarioTestCfg(true), email)
			u, enviado, err := svc.CreateUsuario(context.Background(), db, bug11Input{
				Nome: "Fulano", Email: "fulano@test.com", Role: models.RoleNormal,
			})

			require.Error(t, err)
			if tc.errEsperado != nil {
				assert.ErrorIs(t, err, tc.errEsperado)
			} else {
				assert.NotErrorIs(t, err, services.ErrEmailDuplicado)
			}
			assert.Nil(t, u)
			assert.False(t, enviado)
			assert.Zero(t, email.chamadas, "sem INSERT confirmado não há e-mail")
			assert.Equal(t, []string{"insert"}, linha.lista(), "sem releitura após falha no INSERT")
			assert.NotContains(t, logs.String(), "[usuarios] criado")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

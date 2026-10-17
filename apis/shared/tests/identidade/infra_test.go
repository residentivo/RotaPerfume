package identidade_test

// ARQ-03: ramos de infraestrutura de identidadecrypto e identidademysql que
// os testes portados não alcançam (montagem com nil, LastInsertId, 1062 do
// driver, todas as colunas de ordenação, token sem iat).

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/adapters/ratelimit"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/kernel"
)

var cfgJWT = &config.Config{JWTSecret: "segredo-infra", JWTIssuer: "rp", JWTTTL: time.Hour}

func TestJWT_MontagemTTLEClaimsOpcionais(t *testing.T) {
	assert.Panics(t, func() { identidadecrypto.NovoJWT(nil) })
	j := identidadecrypto.NovoJWT(cfgJWT)
	assert.Equal(t, time.Hour, j.TTL())

	tok, err := j.Emitir(7, kernel.RoleNormal)
	require.NoError(t, err)
	c, err := j.ValidarAcesso(tok)
	require.NoError(t, err)
	assert.Equal(t, int64(7), c.UserID)
	assert.Equal(t, kernel.RoleNormal, c.Role)
	require.NotNil(t, c.EmitidoEm)
	require.NotNil(t, c.ExpiraEm)

	// Token válido sem iat: EmitidoEm nil (o corte de sessão recusa, I-04).
	semIat, err := jwt.NewWithClaims(jwt.SigningMethodHS256, identidadecrypto.Claims{UserID: 7, Role: "normal",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString([]byte(cfgJWT.JWTSecret))
	require.NoError(t, err)
	c, err = j.ValidarAcesso(semIat)
	require.NoError(t, err)
	assert.Nil(t, c.EmitidoEm)

	_, err = j.ValidarAcesso("lixo")
	assert.ErrorIs(t, err, identidadecrypto.ErrTokenInvalido)

	// Só HS256 (I-01): HS512 com o mesmo segredo é recusado.
	hs512, err := jwt.NewWithClaims(jwt.SigningMethodHS512, identidadecrypto.Claims{UserID: 7}).SignedString([]byte(cfgJWT.JWTSecret))
	require.NoError(t, err)
	_, err = j.ValidarAcesso(hs512)
	assert.ErrorIs(t, err, identidadecrypto.ErrTokenInvalido)
}

func TestGeradores(t *testing.T) {
	s, err := identidadecrypto.GeradorSenha{}.Gerar()
	require.NoError(t, err)
	assert.Len(t, s.Revelar(), identidadecrypto.TamanhoSenhaGerada)
	assert.NoError(t, identidade.ValidarForca(s), "a senha gerada atende a política")

	g := identidadecrypto.GeradorRefresh{}
	a, b := g.Novo(), g.Novo()
	assert.Len(t, a.Revelar(), identidadecrypto.BytesRefresh*2)
	assert.NotEqual(t, a.Revelar(), b.Revelar())
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{64}$`), g.Hash(a))
	assert.NotEqual(t, a.Revelar(), g.Hash(a), "o banco guarda só o hash")
}

func TestMySQL_MontagemComNilPanica(t *testing.T) {
	assert.Panics(t, func() { identidademysql.NovoUsuarios(nil) })
	assert.Panics(t, func() { identidademysql.NovoRefreshTokens(nil) })
	assert.Panics(t, func() { identidademysql.NovoHistoricoSenhas(nil) })
}

func TestMySQL_LastInsertIdFalha(t *testing.T) {
	errID := errors.New("sem last insert id")
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	for i := 0; i < 3; i++ {
		mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewErrorResult(errID))
	}
	ctx := context.Background()
	assert.ErrorIs(t, identidademysql.NovoUsuarios(db).Inserir(ctx, &identidade.Usuario{}), errID)
	assert.ErrorIs(t, identidademysql.NovoRefreshTokens(db).Inserir(ctx, &identidade.RefreshToken{}), errID)
	assert.ErrorIs(t, identidademysql.NovoHistoricoSenhas(db).Registrar(ctx, &identidade.RegistroSenha{}), errID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// O-6 iv: o 1062 do driver vira o sentinela SEM embrulhar o erro (que traz o
// e-mail duplicado); outro erro do MySQL é embrulhado.
func TestMySQL_ChaveDuplicadaDoDriver(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec("INSERT INTO usuarios").WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'ana@rp.com' for key 'usuarios.email'"})
	mock.ExpectExec("INSERT INTO usuarios").WillReturnError(&mysql.MySQLError{Number: 1452, Message: "fk"})
	r := identidademysql.NovoUsuarios(db)

	err = r.Inserir(context.Background(), &identidade.Usuario{})
	assert.Equal(t, identidade.ErrEmailDuplicado, err)
	assert.NotContains(t, err.Error(), "ana@rp.com")

	err = r.Inserir(context.Background(), &identidade.Usuario{})
	var me *mysql.MySQLError
	assert.ErrorAs(t, err, &me)
	assert.NotErrorIs(t, err, identidade.ErrEmailDuplicado)
}

func TestMySQL_OrdenacaoDeUsuarios_TodasAsColunas(t *testing.T) {
	colunas := map[identidade.CampoUsuario]string{
		identidade.OrdemUsuarioID: "u.id", identidade.OrdemUsuarioNome: "u.nome", identidade.OrdemUsuarioEmail: "u.email",
		identidade.OrdemUsuarioRole: "u.role", identidade.OrdemUsuarioAtivo: "u.ativo", identidade.OrdemUsuarioCriadoEm: "u.created_at",
		identidade.OrdemUsuarioAtualizadoEm: "u.updated_at", identidade.OrdemUsuarioUltimoLogin: "u.ultimo_login_at",
	}
	for campo, coluna := range colunas {
		t.Run(string(campo), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM usuarios`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
			mock.ExpectQuery(`ORDER BY ` + regexp.QuoteMeta(coluna) + ` DESC\s+LIMIT`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			_, err = identidademysql.NovoUsuarios(db).Listar(context.Background(), kernel.NovaOrdenacao(campo, kernel.Desc), kernel.NovaPagina(1, 20))
			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestMySQL_OrdenacaoDoHistorico_TodasAsColunas(t *testing.T) {
	colunas := map[identidade.CampoHistorico]string{
		identidade.OrdemHistoricoUsuarioID: "sh.usuario_id", identidade.OrdemHistoricoTipo: "sh.tipo_reset",
		identidade.OrdemHistoricoCriadoEm: "sh.created_at", identidade.OrdemHistoricoIP: "sh.ip_origem",
	}
	for campo, coluna := range colunas {
		t.Run(string(campo), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
			mock.ExpectQuery(`ORDER BY ` + regexp.QuoteMeta(coluna) + ` ASC, sh\.id DESC\s+LIMIT`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			_, err = identidademysql.NovoHistoricoSenhas(db).Todos(context.Background(), kernel.NovaOrdenacao(campo, kernel.Asc), kernel.NovaPagina(1, 20))
			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestMySQL_RegistrarLoginFalha(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec("UPDATE usuarios SET ultimo_login_at").WillReturnError(errInfra)
	err = identidademysql.NovoUsuarios(db).RegistrarLogin(context.Background(), 1, time.Now())
	assert.ErrorIs(t, err, errInfra)
	assert.Contains(t, err.Error(), "update ultimo_login")
}

func TestRateLimit_LimparVencidas(t *testing.T) {
	m := ratelimit.Nova(1, time.Minute, time.Minute)
	m.RegistrarFalha("a")
	m.RegistrarFalha("b")
	agora := time.Now()
	assert.Equal(t, 2, m.LimparVencidas(agora), "janela e bloqueio ainda valendo")
	assert.Equal(t, 0, m.LimparVencidas(agora.Add(2*time.Minute)), "tudo vencido sai do mapa")
	bloqueado, _ := m.Bloqueado("a")
	assert.False(t, bloqueado)
}

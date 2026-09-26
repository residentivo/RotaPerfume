package middleware

import (
	"context"
	"database/sql"
	"errors"

	"github.com/rotaperfumes/shared/repositories"
)

// msgUsuarioInativo é a mensagem do 401 para usuário inativo ou inexistente
// (mesma mensagem nos dois casos para não revelar se o id existe).
const msgUsuarioInativo = "usuário inativo"

// ErrUserNotFound indica que o usuário do token não existe mais no banco.
// Implementações de UserStatusChecker devem devolvê-lo (ou embrulhá-lo) nesse
// caso; qualquer outro erro é tratado como falha de infraestrutura (500).
var ErrUserNotFound = errors.New("middleware: usuário não encontrado")

// UserStatus é o estado vigente do usuário no banco.
type UserStatus struct {
	Ativo bool
	Role  string
}

// UserStatusChecker consulta o estado atual de um usuário a cada request
// protegido (SEC-06). Interface para permitir mock nos testes.
type UserStatusChecker interface {
	CheckUserStatus(ctx context.Context, userID int64) (UserStatus, error)
}

// DBUserStatusChecker implementa UserStatusChecker consultando a tabela
// usuarios (SELECT ativo, role ... WHERE id = ?), sem cache.
type DBUserStatusChecker struct {
	db   *sql.DB
	repo *repositories.UsuarioRepository
}

// NewDBUserStatusChecker cria o verificador sobre o pool de conexão da API.
func NewDBUserStatusChecker(db *sql.DB) *DBUserStatusChecker {
	return &DBUserStatusChecker{db: db, repo: repositories.NewUsuarioRepository()}
}

// CheckUserStatus devolve ativo/role do usuário; ErrUserNotFound se ele não
// existir; demais erros de banco são repassados.
func (c *DBUserStatusChecker) CheckUserStatus(ctx context.Context, userID int64) (UserStatus, error) {
	st, err := c.repo.GetStatusByID(ctx, c.db, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return UserStatus{}, ErrUserNotFound
		}
		return UserStatus{}, err
	}
	return UserStatus{Ativo: st.Ativo, Role: st.Role}, nil
}

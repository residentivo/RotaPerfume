package handlers_test

import (
	"context"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
)

// fakeUserStatusChecker é o UserStatusChecker (SEC-06) dos testes de
// handlers que montam o router real: não toca o sqlmock (mantém intactas as
// expectativas de cada teste) e segue a convenção de usuários de teste —
// user_id 1 é admin, os demais são normal, todos ativos.
type fakeUserStatusChecker struct{}

func (fakeUserStatusChecker) CheckUserStatus(_ context.Context, userID int64) (middleware.UserStatus, error) {
	if userID == 1 {
		return middleware.UserStatus{Ativo: true, Role: "admin"}, nil
	}
	return middleware.UserStatus{Ativo: true, Role: "normal"}, nil
}

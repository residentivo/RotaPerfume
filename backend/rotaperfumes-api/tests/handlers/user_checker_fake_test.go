package handlers_test

import (
	"context"

	"github.com/rotaperfumes/shared/identidade"
)

// fakeUserStatusChecker é o UserStatusChecker (SEC-06) dos testes de
// handlers que montam o router real: não toca o sqlmock (mantém intactas as
// expectativas de cada teste) e segue a convenção de usuários de teste —
// user_id 1 é admin, os demais são normal, todos ativos.
type fakeUserStatusChecker struct{}

func (fakeUserStatusChecker) StatusParaAutenticacao(_ context.Context, userID int64) (identidade.StatusAcesso, error) {
	if userID == 1 {
		return identidade.StatusAcesso{Ativo: true, Role: "admin"}, nil
	}
	return identidade.StatusAcesso{Ativo: true, Role: "normal"}, nil
}

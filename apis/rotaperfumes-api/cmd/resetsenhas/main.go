// Command resetsenhas redefine a senha de TODOS os usuários ativos (OPS-01),
// com o mesmo fluxo do POST /api/admin/reset-password: senha aleatória, hash
// Argon2id com o PASSWORD_PEPPER, troca obrigatória no próximo login, e-mail
// com a nova senha, registro em senha_historico e revogação das sessões.
// A lógica fica em internal/resetsenhas; este main é só a composition root.
//
// Uso (a partir de apis/rotaperfumes-api):
//
//	go run ./cmd/resetsenhas             # simulação: só lista quem seria atingido
//	go run ./cmd/resetsenhas -executar   # grava e envia os e-mails
//
// Recusa rodar sem SMTP configurado: a senha nunca é impressa nem logada, então
// sem e-mail ela se perderia. Para no primeiro e-mail que falhar.
package main

import (
	"os"

	"github.com/rotaperfumes/rotaperfumes-api/internal/resetsenhas"
	"github.com/rotaperfumes/shared/cmdutil"
)

func main() {
	os.Exit(cmdutil.Executar(resetsenhas.Tag, resetsenhas.CLI(resetsenhas.DepsPadrao()), os.Args[1:], cmdutil.AmbientePadrao()))
}

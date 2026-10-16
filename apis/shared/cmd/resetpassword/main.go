// Command resetpassword cria ou atualiza usuários no banco, com hash Argon2id
// válido (pepper PASSWORD_PEPPER do .env). A lógica (flags, fonte da senha,
// conexão e ações) fica em tools/resetpassword; este main é só a composition
// root.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/resetpassword -list
//	cd apis/shared && go run ./cmd/resetpassword -email=admin@rotaperfumes.com.br -password-prompt -role=admin
//	printf '%s\n' "$NOVA_SENHA" | go run ./cmd/resetpassword -all-users -password-stdin
//	cd apis/shared && go run ./cmd/resetpassword -create-admin   # SEED_ADMIN_PASSWORD do .env ou aleatória
//
// Exige DB_USUARIO e DB_SENHA no .env (e PASSWORD_PEPPER, salvo em -list). -password=... ainda funciona, mas é
// depreciada (fica no histórico do shell) e gera aviso no stderr. No Git Bash
// (mintty), -password-prompt precisa de `winpty go run ...`.
package main

import (
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/tools/resetpassword"
	// Este binário não usa config.DSN(); importa tz diretamente para fixar
	// time.Local em -03:00 antes do sql.Open (RISCO-01).
	_ "github.com/rotaperfumes/shared/tz"
)

func main() {
	os.Exit(cmdutil.Executar("", resetpassword.CLI(resetpassword.DepsPadrao()), os.Args[1:], cmdutil.AmbientePadrao()))
}

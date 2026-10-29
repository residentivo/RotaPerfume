// Command seedusers gera hashes Argon2id (com pepper) para os placeholders dos SQLs de seed,
// grava cópias renderizadas em <raiz>/tmp/seed (sql/ nunca é alterado) e as
// executa contra o MariaDB (via cliente mariadb). A lógica fica em tools/seedusers; este main é só a
// composition root.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/seedusers            # exige DB_USUARIO/DB_SENHA no .env; apaga tmp/seed ao final
//	cd apis/shared && go run ./cmd/seedusers -no-exec   # mantém os SQLs em tmp/seed (apague depois)
//	cd apis/shared && go run ./cmd/seedusers -dry-run   # só imprime os hashes (mascarados; completos com -show-password)
package main

import (
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/tools/seedusers"
)

func main() {
	os.Exit(cmdutil.Executar(seedusers.Tag, seedusers.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}

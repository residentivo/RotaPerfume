package db

import "strings"

// escapeLike troca os curingas do LIKE por literais. O caractere de escape é
// '!', não '\': com '\' o resultado dependeria do sql_mode
// NO_BACKSLASH_ESCAPES do servidor (SEC-25).
var escapeLike = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")

// TermoContem devolve o argumento de uma busca "contém" com '%' e '_'
// tratados como literais: "%" + termo escapado + "%". O SQL tem de usar
// `coluna LIKE ? ESCAPE '!'` (SEC-25).
func TermoContem(termo string) string {
	return "%" + escapeLike.Replace(termo) + "%"
}

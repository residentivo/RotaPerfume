package repositories

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// mysqlErrDuplicateEntry é o código MySQL de violação de chave única
// (ER_DUP_ENTRY).
const mysqlErrDuplicateEntry = 1062

// uqClientesCNPJ é o nome do índice UNIQUE em clientes.cnpj (NEG-01).
const uqClientesCNPJ = "uq_clientes_cnpj"

// ErrCNPJDuplicado é devolvido por ClienteRepository.Create/Update quando o
// INSERT/UPDATE viola o índice UNIQUE uq_clientes_cnpj.
var ErrCNPJDuplicado = errors.New("repositories: cnpj duplicado")

// isDuplicateKey reporta se err é um erro MySQL 1062 (duplicate entry) para
// o índice keyName. O nome do índice aparece na mensagem do MySQL
// ("Duplicate entry '...' for key 'clientes.uq_clientes_cnpj'").
func isDuplicateKey(err error, keyName string) bool {
	var myErr *mysql.MySQLError
	if !errors.As(err, &myErr) {
		return false
	}
	return myErr.Number == mysqlErrDuplicateEntry && strings.Contains(myErr.Message, keyName)
}

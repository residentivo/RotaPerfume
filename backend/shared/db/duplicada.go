package db

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// codigoChaveDuplicada é o ER_DUP_ENTRY do MariaDB/MySQL.
const codigoChaveDuplicada = 1062

// ChaveDuplicada informa se err é um 1062 do MariaDB/MySQL. Com indice != "", exige
// também que a violação seja daquele índice (o nome aparece na mensagem:
// "Duplicate entry '…' for key 'clientes.uq_clientes_cnpj'").
//
// O-6 iv / N-8: o adapter que recebe true devolve o sentinela do domínio SEM
// embrulhar err, porque a mensagem do driver traz o valor duplicado (CNPJ,
// SKU) e iria parar no log do mapper HTTP.
func ChaveDuplicada(err error, indice string) bool {
	var me *mysql.MySQLError
	if !errors.As(err, &me) || me.Number != codigoChaveDuplicada {
		return false
	}
	return strings.Contains(me.Message, indice)
}

// SemValorDuplicado devolve err sem o VALOR duplicado quando ele é um 1062
// (N2 do ARQ-04): no lugar do erro original vai um *mysql.MySQLError com o
// mesmo código e só o nome do índice ("Duplicate entry (valor omitido) for
// key 'clientes.uq_clientes_cnpj'"). Quem confere tipo, código ou índice
// (errors.As, ChaveDuplicada) continua funcionando e o valor não chega ao
// log. Qualquer outro erro volta inalterado.
func SemValorDuplicado(err error) error {
	var me *mysql.MySQLError
	if !errors.As(err, &me) || me.Number != codigoChaveDuplicada {
		return err
	}
	indice := ""
	if i := strings.LastIndex(me.Message, " for key "); i >= 0 {
		indice = me.Message[i:]
	}
	return &mysql.MySQLError{Number: codigoChaveDuplicada, SQLState: me.SQLState, Message: "Duplicate entry (valor omitido)" + indice}
}

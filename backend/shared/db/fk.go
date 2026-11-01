package db

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// codigoViolacaoFK é o ER_NO_REFERENCED_ROW_2 do MariaDB/MySQL: o INSERT ou
// UPDATE aponta para uma linha pai que não existe.
const codigoViolacaoFK = 1452

// ViolacaoFK informa se err é um 1452 do MariaDB/MySQL. Com constraint != "",
// exige também que a violação seja daquela FK (o nome aparece na mensagem:
// "Cannot add or update a child row: a foreign key constraint fails (…
// CONSTRAINT `fk_pagamentos_pedido` …)").
//
// Como em ChaveDuplicada: o adapter que recebe true devolve o sentinela do
// domínio SEM embrulhar err (a mensagem do driver traz o schema e a
// definição da FK, que não vão para o log do mapper HTTP).
func ViolacaoFK(err error, constraint string) bool {
	var me *mysql.MySQLError
	if !errors.As(err, &me) || me.Number != codigoViolacaoFK {
		return false
	}
	return strings.Contains(me.Message, constraint)
}

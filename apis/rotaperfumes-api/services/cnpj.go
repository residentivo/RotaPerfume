package services

import (
	"errors"
	"strings"

	"github.com/rotaperfumes/shared/cnpj"
	"github.com/rotaperfumes/shared/vlog"
)

// ErrCNPJInvalido indica CNPJ fora do formato (≠ 14 caracteres, caracteres
// fora de [0-9A-Z] nas 12 primeiras posições, DV não numérico, todos os
// caracteres iguais) ou com dígito verificador inválido. Aceita CNPJ numérico
// e alfanumérico (NEG-02); a regra fica em github.com/rotaperfumes/shared/cnpj.
var ErrCNPJInvalido = errors.New("cnpj inválido")

// ErrCNPJDuplicado indica que já existe outro cliente com o mesmo CNPJ
// (índice UNIQUE uq_clientes_cnpj). A mensagem é genérica de propósito: não
// revela id, vendedor nem razão social do cliente existente.
var ErrCNPJDuplicado = errors.New("cnpj já cadastrado")

// cnpjMascara são os caracteres de máscara aceitos na entrada.
const cnpjMascara = "./-"

// normalizarCNPJ remove a máscara, converte para maiúsculas e checa o
// formato (14 caracteres, 12 em [0-9A-Z] + 2 DVs numéricos). ok=false se o
// valor tiver caractere inválido ou formato errado. Não valida o DV.
func normalizarCNPJ(raw string) (normalizado string, ok bool) {
	vlog.Printf("cnpj.go", "normalizarCNPJ", "chamando cnpj.Normalizar e atribuindo a normalizado, ok")
	normalizado, ok = cnpj.Normalizar(raw)
	vlog.Printf("cnpj.go", "normalizarCNPJ", "verificando condição !ok || !cnpj.FormatoValido(normalizado)")
	if !ok || !cnpj.FormatoValido(normalizado) {
		return "", false
	}
	return normalizado, true
}

// cnpjDigitosValidos reporta se o CNPJ normalizado tem formato válido, não é
// uma sequência de caracteres iguais e tem os dois DVs corretos (módulo 11).
func cnpjDigitosValidos(normalizado string) bool {
	return cnpj.Valido(normalizado)
}

// termoBuscaCNPJ devolve o termo a usar na busca por cnpj quando q parece um
// CNPJ digitado com máscara (ex.: "12.ABC.345/01DE-35" ou "12.abc"): sem a
// máscara e em maiúsculas, pois o banco grava o CNPJ assim. Devolve "" quando
// q não tem máscara ou tem caracteres que não pertencem a um CNPJ — nesse caso
// a busca por cnpj usa o próprio q (a collation do banco ignora caixa).
func termoBuscaCNPJ(q string) string {
	vlog.Printf("cnpj.go", "termoBuscaCNPJ", "verificando condição !strings.ContainsAny(q, cnpjMascara)")
	if !strings.ContainsAny(q, cnpjMascara) {
		return ""
	}
	vlog.Printf("cnpj.go", "termoBuscaCNPJ", "chamando cnpj.Normalizar e declarando normalizado, ok")
	normalizado, ok := cnpj.Normalizar(q)
	vlog.Printf("cnpj.go", "termoBuscaCNPJ", "verificando condição !ok")
	if !ok {
		return ""
	}
	return normalizado
}

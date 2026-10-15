package comercial

import (
	"strings"

	"github.com/rotaperfumes/shared/cnpj"
)

// CNPJ é o VO do CNPJ do cliente (NEG-01/NEG-02): sem máscara e em
// maiúsculas, numérico ou alfanumérico. Nunca vai para log (M-04).
//
// A validação tem duas etapas, na ordem do legado: o formato é conferido
// junto com os obrigatórios (NovoCNPJ) e o dígito verificador só no fim
// (DigitosValidos), depois de segmento, cidade, UF e data.
type CNPJ struct{ valor string }

// NovoCNPJ apara, exige preenchimento (ErrCNPJObrigatorio), remove a máscara
// e confere o formato (ErrCNPJInvalido). Não confere o DV.
func NovoCNPJ(bruto string) (CNPJ, error) {
	v := strings.TrimSpace(bruto)
	if v == "" {
		return CNPJ{}, ErrCNPJObrigatorio
	}
	normalizado, ok := cnpj.Normalizar(v)
	if !ok || !cnpj.FormatoValido(normalizado) {
		return CNPJ{}, ErrCNPJInvalido
	}
	return CNPJ{valor: normalizado}, nil
}

// DigitosValidos confere o DV (módulo 11) e recusa sequências iguais
// (NEG-04: exigido em toda gravação, inclusive sem alteração do CNPJ).
func (c CNPJ) DigitosValidos() bool { return cnpj.Valido(c.valor) }

// String devolve o CNPJ normalizado (o valor gravado no banco).
func (c CNPJ) String() string { return c.valor }

// mascaraCNPJ são os caracteres de máscara aceitos na busca.
const mascaraCNPJ = "./-"

// TermoBuscaCNPJ devolve o termo da busca em cnpj quando q parece um CNPJ
// digitado com máscara ("12.ABC.345/01DE-35"): sem a máscara e em
// maiúsculas, como o banco grava. Devolve "" quando q não tem máscara ou tem
// caracteres de fora do CNPJ: aí a busca em cnpj usa o próprio q.
func TermoBuscaCNPJ(q string) string {
	if !strings.ContainsAny(q, mascaraCNPJ) {
		return ""
	}
	normalizado, ok := cnpj.Normalizar(q)
	if !ok {
		return ""
	}
	return normalizado
}

// normalizarUF apara, passa a maiúsculas e exige 2 bytes (regra legada
// len(uf) != 2, comum a cliente e vendedor).
func normalizarUF(s string) (string, error) {
	uf := strings.ToUpper(strings.TrimSpace(s))
	if len(uf) != 2 {
		return "", ErrUFInvalida
	}
	return uf, nil
}

package kernel

import "strings"

// Direcao é a direção de ordenação. O zero value é indefinido e só aparece
// em Ordenacao zero value (ParseOrdenacao sempre resolve para Asc ou Desc).
type Direcao uint8

// Direções.
const (
	direcaoIndefinida Direcao = iota
	Asc
	Desc
)

// String devolve "asc", "desc" ou "" (indefinida).
func (d Direcao) String() string {
	switch d {
	case Asc:
		return "asc"
	case Desc:
		return "desc"
	default:
		return ""
	}
}

// ParseDirecao aceita "asc"/"desc" (sem diferenciar maiúsculas e com
// espaços nas pontas). ok=false para qualquer outro valor, incluindo vazio.
func ParseDirecao(s string) (Direcao, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "asc":
		return Asc, true
	case "desc":
		return Desc, true
	default:
		return direcaoIndefinida, false
	}
}

// Ordenacao é a ordenação de uma listagem sobre um enum de campos C, próprio
// de cada entidade (C-02). Nenhuma assinatura de repositório recebe string
// livre de coluna: o adapter faz switch sobre C.
type Ordenacao[C ~string] struct {
	campo   C
	direcao Direcao
}

// NovaOrdenacao monta uma ordenação (use para o default de cada entidade).
func NovaOrdenacao[C ~string](campo C, d Direcao) Ordenacao[C] {
	return Ordenacao[C]{campo: campo, direcao: d}
}

// ParseOrdenacao lê order_by/order_dir contra a lista de campos válidos.
// Campo fora da lista (ou vazio) cai no campo padrão; direção inválida (ou
// vazia) cai na direção padrão. A comparação de campo ignora maiúsculas e
// espaços nas pontas, como o resolveOrderColumn legado.
func ParseOrdenacao[C ~string](campo, direcao string, validos []C, padrao Ordenacao[C]) Ordenacao[C] {
	out := padrao
	normalizado := strings.ToLower(strings.TrimSpace(campo))
	for _, v := range validos {
		if string(v) == normalizado {
			out.campo = v
			break
		}
	}
	if d, ok := ParseDirecao(direcao); ok {
		out.direcao = d
	}
	return out
}

// Campo devolve o campo de ordenação.
func (o Ordenacao[C]) Campo() C { return o.campo }

// Direcao devolve a direção.
func (o Ordenacao[C]) Direcao() Direcao { return o.direcao }

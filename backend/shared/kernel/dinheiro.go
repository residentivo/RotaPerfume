package kernel

import "math"

// Dinheiro é um valor monetário ≥ 0. P3: float64 encapsulado, com a mesma
// aritmética de hoje; a troca para centavos é o card ARQ-09. As operações de
// cada contexto (ex.: valor bruto de item de pedido) entram com o contexto.
type Dinheiro struct {
	valor float64
}

// ErrDinheiroInvalido sinaliza valor negativo, NaN ou infinito.
var ErrDinheiroInvalido = NovoErro(KindInvalido, "valor monetário inválido")

// NovoDinheiro valida o valor.
func NovoDinheiro(v float64) (Dinheiro, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return Dinheiro{}, ErrDinheiroInvalido
	}
	return Dinheiro{valor: v}, nil
}

// Float64 devolve o valor (para DTO e SQL).
func (d Dinheiro) Float64() float64 { return d.valor }

// Somar devolve d + o.
func (d Dinheiro) Somar(o Dinheiro) Dinheiro { return Dinheiro{valor: d.valor + o.valor} }

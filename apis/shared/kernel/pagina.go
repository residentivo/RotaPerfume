package kernel

// Limites de paginação (F-10).
const (
	LimitePadrao = 20
	LimiteMaximo = 100
)

// Pagina é a janela de uma listagem: page ≥ 1; limit < 1 vira 20; limit > 100
// vira 100. O zero value equivale a NovaPagina(0, 0) (página 1, limite 20).
type Pagina struct {
	numero int
	limite int
}

// NovaPagina normaliza page e limit (F-10).
func NovaPagina(numero, limite int) Pagina {
	if numero < 1 {
		numero = 1
	}
	if limite < 1 {
		limite = LimitePadrao
	}
	if limite > LimiteMaximo {
		limite = LimiteMaximo
	}
	return Pagina{numero: numero, limite: limite}
}

// Numero devolve a página (≥ 1).
func (p Pagina) Numero() int { return NovaPagina(p.numero, p.limite).numero }

// Limite devolve o tamanho da página (1..100).
func (p Pagina) Limite() int { return NovaPagina(p.numero, p.limite).limite }

// Offset devolve (page-1)*limit.
func (p Pagina) Offset() int { return (p.Numero() - 1) * p.Limite() }

// Resultado é uma página de itens com o total da consulta.
type Resultado[T any] struct {
	Itens  []T
	Pagina Pagina
	Total  int
}

// NovoResultado monta um Resultado.
func NovoResultado[T any](itens []T, p Pagina, total int) Resultado[T] {
	return Resultado[T]{Itens: itens, Pagina: p, Total: total}
}

// Paginas devolve o número de páginas (teto de total/limit; 0 sem itens).
func (r Resultado[T]) Paginas() int {
	limite := r.Pagina.Limite()
	paginas := r.Total / limite
	if r.Total%limite != 0 {
		paginas++
	}
	return paginas
}

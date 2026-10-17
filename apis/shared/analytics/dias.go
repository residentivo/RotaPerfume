package analytics

// Limites da janela da série de vendas (D-03, F-10).
const (
	DiasPadrao = 30
	DiasMaximo = 365
)

// Dias é a janela da série de vendas: sempre de 1 a 365 (D-03). O zero
// value vale DiasPadrao (30), então nenhum caminho leva 0 ou negativo ao
// INTERVAL ? DAY.
type Dias struct{ n int }

// DiasDe aceita n de 1 a 365; fora disso, DiasPadrao (o legado não
// responde 400: ignora o valor e usa 30).
func DiasDe(n int) Dias {
	if n < 1 || n > DiasMaximo {
		return Dias{}
	}
	return Dias{n: n}
}

// ParseDias lê o ?dias= do contrato com a regra legada: vazio, com algum
// caractere que não seja dígito ASCII (sinal, espaço, ponto) ou fora de
// 1..365 → 30. O acumulador é o do parse legado (int, sem checagem de
// overflow), para que nenhum valor mude de janela na migração.
func ParseDias(s string) Dias {
	if s == "" {
		return Dias{}
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return Dias{}
		}
		n = n*10 + int(c-'0')
	}
	return DiasDe(n)
}

// Int devolve a janela em dias (1..365).
func (d Dias) Int() int {
	if d.n == 0 {
		return DiasPadrao
	}
	return d.n
}

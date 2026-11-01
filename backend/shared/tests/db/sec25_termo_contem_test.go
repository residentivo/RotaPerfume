package db_test

// SEC-25 (Lote 27): db.TermoContem escapa '!', '%' e '_' com '!' (o
// caractere de ESCAPE do LIKE) e envolve o termo em '%'.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/db"
)

func TestSEC25_TermoContem(t *testing.T) {
	casos := []struct {
		termo, want string
	}{
		{"", "%%"},
		{"abc", "%abc%"},
		{"%", "%!%%"},
		{"_", "%!_%"},
		{"!", "%!!%"},
		{"a%b_c!d", "%a!%b!_c!!d%"},
		{"!%", "%!!!%%"},
		{"%%__!!", "%!%!%!_!_!!!!%"},
		{`a\b`, `%a\b%`}, // '\' não é escape: fica como está
		{"perfumaria ção", "%perfumaria ção%"},
		{"ç_ã%é", "%ç!_ã!%é%"},
		{"香水_100%", "%香水!_100!%%"},
	}
	for _, c := range casos {
		t.Run(c.termo, func(t *testing.T) {
			assert.Equal(t, c.want, db.TermoContem(c.termo))
		})
	}
}

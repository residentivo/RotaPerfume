package vlog_test

import (
	"testing"

	"github.com/rotaperfumes/shared/vlog"
)

// Testes da API pública de vlog (movidos de vlog/vlog_test.go para seguir a
// convenção de pasta separada / pacote externo).

func TestPrintf(t *testing.T) {
	casos := []struct {
		nome   string
		ligado bool
		format string
		args   []any
		want   string
	}{
		{"desligado não emite", false, "msg %d", []any{1}, ""},
		{"ligado sem args", true, "msg", nil, "[a.go] [F] msg\n"},
		{"ligado com args", true, "page=%d ok=%v", []any{2, true}, "[a.go] [F] page=2 ok=true\n"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out := capturar(t, c.ligado, func() { vlog.Printf("a.go", "F", c.format, c.args...) })
			if out != c.want {
				t.Fatalf("saída = %q, want %q", out, c.want)
			}
			if vlog.Enabled() != c.ligado {
				t.Fatalf("Enabled() = %v, want %v", vlog.Enabled(), c.ligado)
			}
		})
	}
}

package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/kernel"
)

type campoTeste string

const (
	campoID   campoTeste = "id"
	campoNome campoTeste = "nome"
)

var camposTeste = []campoTeste{campoID, campoNome}

func TestParseDirecao(t *testing.T) {
	casos := []struct {
		in   string
		want kernel.Direcao
		ok   bool
	}{
		{"asc", kernel.Asc, true},
		{"DESC", kernel.Desc, true},
		{" Asc ", kernel.Asc, true},
		{"", 0, false},
		{"desc; DROP TABLE x", 0, false},
		{"ascending", 0, false},
	}
	for _, c := range casos {
		d, ok := kernel.ParseDirecao(c.in)
		assert.Equal(t, c.ok, ok, "in=%q", c.in)
		assert.Equal(t, c.want, d, "in=%q", c.in)
	}
	assert.Equal(t, "asc", kernel.Asc.String())
	assert.Equal(t, "desc", kernel.Desc.String())
	assert.Equal(t, "", kernel.Direcao(0).String())
}

// C-02: campo fora do enum cai no default; direção inválida cai no default.
func TestParseOrdenacao(t *testing.T) {
	padrao := kernel.NovaOrdenacao(campoID, kernel.Desc)
	casos := []struct {
		nome       string
		campo, dir string
		wantCampo  campoTeste
		wantDir    kernel.Direcao
	}{
		{"vazio usa default", "", "", campoID, kernel.Desc},
		{"campo válido, dir ausente usa dir default", "nome", "", campoNome, kernel.Desc},
		{"campo válido asc", "nome", "asc", campoNome, kernel.Asc},
		{"campo com maiúsculas e espaços", " NOME ", "ASC", campoNome, kernel.Asc},
		{"campo inválido mantém dir válida", "senha_hash", "asc", campoID, kernel.Asc},
		{"injeção no campo", "nome; DROP TABLE usuarios;--", "desc", campoID, kernel.Desc},
		{"injeção na direção", "nome", "desc; DROP TABLE x", campoNome, kernel.Desc},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			o := kernel.ParseOrdenacao(c.campo, c.dir, camposTeste, padrao)
			assert.Equal(t, c.wantCampo, o.Campo())
			assert.Equal(t, c.wantDir, o.Direcao())
		})
	}
}

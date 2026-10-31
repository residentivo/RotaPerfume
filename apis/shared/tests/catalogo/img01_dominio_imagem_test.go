package catalogo_test

// IMG-01 (Lote 26, TestBrain): regras puras da imagem do produto
// (catalogo/imagem.go): regex do nome <uuid-v4><ext>, mapa fixo
// extensão↔tipo e limite de dimensões.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/catalogo"
)

const img01UUID = "3f2b8c1e-9a4d-4c7b-8e2f-1a2b3c4d5e6f"

func TestIMG01_NomeImagemValido_e_TipoDoNome(t *testing.T) {
	casos := []struct {
		nome   string
		valido bool
		tipo   string
	}{
		{img01UUID + ".jpg", true, catalogo.TipoImagemJPEG},
		{img01UUID + ".png", true, catalogo.TipoImagemPNG},
		{img01UUID + ".webp", true, catalogo.TipoImagemWebP},
		{"3f2b8c1e-9a4d-4c7b-9e2f-1a2b3c4d5e6f.png", true, catalogo.TipoImagemPNG}, // variante 9
		{"3f2b8c1e-9a4d-4c7b-ae2f-1a2b3c4d5e6f.png", true, catalogo.TipoImagemPNG}, // variante a
		{"3f2b8c1e-9a4d-4c7b-be2f-1a2b3c4d5e6f.png", true, catalogo.TipoImagemPNG}, // variante b
		{"", false, ""},
		{img01UUID, false, ""},
		{img01UUID + ".jpeg", false, ""},
		{img01UUID + ".svg", false, ""},
		{img01UUID + ".html", false, ""},
		{img01UUID + ".gif", false, ""},
		{img01UUID + ".PNG", false, ""},
		{"3F2B8C1E-9A4D-4C7B-8E2F-1A2B3C4D5E6F.png", false, ""}, // maiúsculas
		{"3f2b8c1e-9a4d-1c7b-8e2f-1a2b3c4d5e6f.png", false, ""}, // UUID v1
		{"3f2b8c1e-9a4d-4c7b-7e2f-1a2b3c4d5e6f.png", false, ""}, // variante inválida
		{"3f2b8c1e-9a4d-4c7b-ce2f-1a2b3c4d5e6f.png", false, ""}, // variante inválida
		{"3f2b8c1e9a4d4c7b8e2f1a2b3c4d5e6f.png", false, ""},     // sem hífens
		{".tmp-" + img01UUID, false, ""},                        // temporário
		{".tmp-" + img01UUID + ".png", false, ""},               // temporário com ext
		{".tmp-probe-" + img01UUID, false, ""},                  // probe
		{"../" + img01UUID + ".png", false, ""},                 // traversal
		{"../x", false, ""},                                     //
		{img01UUID + ".png/../../etc/passwd", false, ""},        //
		{"a/" + img01UUID + ".png", false, ""},                  //
		{img01UUID + ".png\n", false, ""},                       // \n final ($ do Go não aceita)
		{img01UUID + ".png.html", false, ""},                    //
		{img01UUID + ".png\x00.html", false, ""},                //
		{" " + img01UUID + ".png", false, ""},                   //
		{"g" + img01UUID[1:] + ".png", false, ""},               // hex inválido
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			assert.Equal(t, c.valido, catalogo.NomeImagemValido(c.nome))
			assert.Equal(t, c.tipo, catalogo.TipoDoNome(c.nome))
		})
	}
}

func TestIMG01_ExtensaoDoTipo(t *testing.T) {
	casos := map[string]string{
		catalogo.TipoImagemJPEG: ".jpg",
		catalogo.TipoImagemPNG:  ".png",
		catalogo.TipoImagemWebP: ".webp",
		"image/gif":             "",
		"image/svg+xml":         "",
		"text/html":             "",
		"":                      "",
	}
	for tipo, ext := range casos {
		t.Run(tipo, func(t *testing.T) { assert.Equal(t, ext, catalogo.ExtensaoDoTipo(tipo)) })
	}
}

func TestIMG01_DimensoesImagemValidas(t *testing.T) {
	casos := []struct {
		w, h int
		ok   bool
	}{
		{1, 1, true},
		{4096, 1, true},
		{1, 4096, true},
		{4000, 4000, true}, // 16.000.000 exatos
		{4096, 3906, true}, // 15.998.976
		{4096, 3907, false},
		{4096, 4096, false},
		{0, 10, false},
		{10, 0, false},
		{-1, 10, false},
		{10, -1, false},
		{4097, 10, false},
		{10, 4097, false},
		{50000, 50000, false},
	}
	for _, c := range casos {
		assert.Equal(t, c.ok, catalogo.DimensoesImagemValidas(c.w, c.h), "%dx%d", c.w, c.h)
	}
}

func TestIMG01_LimitesEMensagens(t *testing.T) {
	assert.Equal(t, 1<<20, catalogo.MaxImagemBytes)
	assert.Equal(t, 4096, catalogo.MaxLadoImagemPx)
	assert.Equal(t, 16_000_000, catalogo.MaxPixelsImagem)
	assert.Equal(t, "arquivo de imagem inválido (use JPEG, PNG ou WebP)", catalogo.ErrImagemInvalida.Error())
	assert.Equal(t, "imagem excede 1 MB", catalogo.ErrImagemMuitoGrande.Error())
	assert.Equal(t, "imagem não encontrada", catalogo.ErrImagemNaoEncontrada.Error())
	assert.Equal(t, "erro interno", catalogo.ErrImagensIndisponiveis.Error())
}

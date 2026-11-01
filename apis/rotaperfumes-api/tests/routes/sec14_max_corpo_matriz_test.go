package routes_test

// SEC-14 (Lote 27): só o upload de imagem declara Rota.MaxCorpo próprio
// (catalogohttp.MaxCorpoUploadBytes); todas as outras rotas ficam no teto
// padrão de 256 KiB (MaxCorpo = 0).

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/internal/catalogohttp"
)

func TestSEC14_Matriz_SoOUploadTemMaxCorpoProprio(t *testing.T) {
	proprios := map[string]int64{}
	for _, rt := range arq00RotasRegistradas(t) {
		if rt.MaxCorpo != 0 {
			proprios[rt.Metodo+" "+rt.Padrao] = rt.MaxCorpo
		}
	}
	assert.Equal(t, map[string]int64{"POST /api/produtos/{id}/imagem": catalogohttp.MaxCorpoUploadBytes}, proprios)
}

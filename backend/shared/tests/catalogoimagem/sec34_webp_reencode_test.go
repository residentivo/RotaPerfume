package catalogoimagem_test

// SEC-34 (Lote 31): o WebP é reencodado (PNG se tiver pixel não opaco, JPEG
// q90 se for todo opaco), então EXIF/XMP e bytes anexados não chegam ao
// arquivo gravado. O tipo e a extensão seguem a saída. O teto de saída é
// catalogo.MaxImagemSaidaBytes (4 MiB) para todos os formatos.

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoimagem"
)

// sec34ComChunkEAnexo devolve o WebP com um chunk EXIF depois do VP8L
// (dentro do RIFF, tamanho do RIFF ajustado) e HTML anexado depois do RIFF.
func sec34ComChunkEAnexo(webp []byte, exif, anexo string) []byte {
	var chunk bytes.Buffer
	chunk.WriteString("EXIF")
	_ = binary.Write(&chunk, binary.LittleEndian, uint32(len(exif)))
	chunk.WriteString(exif)
	if len(exif)%2 == 1 {
		chunk.WriteByte(0)
	}
	out := append([]byte{}, webp...)
	out = append(out, chunk.Bytes()...)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return append(out, []byte(anexo)...)
}

func TestSEC34_WebPComEXIFEHTMLAnexado_SaidaSemEssesBytes(t *testing.T) {
	const exif = "Exif\x00\x00segredo-gps-SEC34"
	const html = "<html><script>alert('sec34')</script></html>"
	for nome, base := range map[string][]byte{
		"opaco → JPEG":       imgWebP(6, 6),
		"transparente → PNG": imgWebPAlfa(6, 6, 0x10),
	} {
		t.Run(nome, func(t *testing.T) {
			entrada := sec34ComChunkEAnexo(base, exif, html)
			require.Contains(t, string(entrada), "segredo-gps-SEC34", "pré-condição: o EXIF está na entrada")

			out, err := catalogoimagem.Novo().Normalizar(entrada)
			require.NoError(t, err)

			for _, proibido := range []string{"segredo-gps-SEC34", "Exif", "EXIF", "<html", "<script", "RIFF", "WEBP", "VP8L"} {
				assert.NotContains(t, string(out.Dados), proibido)
			}
			assert.Equal(t, out.Tipo, http.DetectContentType(out.Dados), "o tipo gravado é o da saída")
			assert.Equal(t, catalogo.ExtensaoDoTipo(out.Tipo), out.Extensao)
			assert.NotEqual(t, catalogo.TipoImagemWebP, out.Tipo, "WebP nunca é gravado como veio")
		})
	}
}

func TestSEC34_WebPTransparente_PreservaAlfa(t *testing.T) {
	out, err := catalogoimagem.Novo().Normalizar(imgWebPAlfa(3, 2, 0x40))
	require.NoError(t, err)
	require.Equal(t, catalogo.TipoImagemPNG, out.Tipo)
	img, err := png.Decode(bytes.NewReader(out.Dados))
	require.NoError(t, err)
	_, _, _, a := img.At(0, 0).RGBA()
	assert.Less(t, a, uint32(0xffff), "o alfa não opaco sobrevive na conversão para PNG")
}

func TestSEC34_TetoDeSaida(t *testing.T) {
	assert.Equal(t, 4<<20, catalogo.MaxImagemSaidaBytes)
	assert.GreaterOrEqual(t, catalogo.MaxImagemSaidaBytes, catalogo.MaxImagemBytes)
}

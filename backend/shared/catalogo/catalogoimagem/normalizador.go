// Package catalogoimagem é o adapter de validação de conteúdo da imagem do
// produto (IMG-01, porta catalogoapp.NormalizadorImagens). Ignora filename,
// extensão e Content-Type do cliente: o tipo vem dos magic bytes
// (http.DetectContentType), confirmado pelo DecodeConfig do PRÓPRIO formato,
// com limite de dimensões antes do decode completo (decompression bomb) e
// reencode de tudo o que é gravado (remove EXIF/XMP e qualquer payload
// anexado; resolve arquivos poliglotas). SEC-34: o WebP (a x/image só
// decodifica) é convertido para PNG se tiver algum pixel não opaco e para
// JPEG q90 se for todo opaco; o tipo e a extensão gravados seguem a saída.
// A saída tem teto de catalogo.MaxImagemSaidaBytes.
//
// Só os decoders de JPEG, PNG e WebP são usados, chamados diretamente (sem o
// registro global do pacote image): um decoder registrado por outra
// dependência (ex.: image/gif) nunca é alcançado por aqui.
package catalogoimagem

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"

	"golang.org/x/image/webp"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/vlog"
)

const (
	arqNormalizador = "normalizador.go"
	bytesSniff      = 512
	qualidadeJPEG   = 90
)

// codec são as funções do formato detectado.
type codec struct {
	config func(io.Reader) (image.Config, error)
	decode func(io.Reader) (image.Image, error)
	// saida escolhe o formato gravado pela imagem decodificada.
	saida func(image.Image) string
}

var codecs = map[string]codec{
	catalogo.TipoImagemJPEG: {config: jpeg.DecodeConfig, decode: jpeg.Decode, saida: fixo(catalogo.TipoImagemJPEG)},
	catalogo.TipoImagemPNG:  {config: png.DecodeConfig, decode: png.Decode, saida: fixo(catalogo.TipoImagemPNG)},
	catalogo.TipoImagemWebP: {config: webp.DecodeConfig, decode: webp.Decode, saida: saidaDoWebP},
}

// encoders são os formatos que o adapter grava (sem WebP: a x/image não
// tem encoder).
var encoders = map[string]func(io.Writer, image.Image) error{
	catalogo.TipoImagemJPEG: encodeJPEG,
	catalogo.TipoImagemPNG:  png.Encode,
}

func fixo(tipo string) func(image.Image) string {
	return func(image.Image) string { return tipo }
}

// saidaDoWebP (SEC-34): PNG se algum pixel não for opaco (preserva a
// transparência); JPEG q90 se a imagem for toda opaca.
func saidaDoWebP(img image.Image) string {
	if opaca(img) {
		return catalogo.TipoImagemJPEG
	}
	return catalogo.TipoImagemPNG
}

// opaca usa o Opaque() do tipo concreto quando existe; senão confere o
// alfa pixel a pixel.
func opaca(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return false
			}
		}
	}
	return true
}

func encodeJPEG(w io.Writer, img image.Image) error {
	return jpeg.Encode(w, img, &jpeg.Options{Quality: qualidadeJPEG})
}

// Normalizador satisfaz catalogoapp.NormalizadorImagens. Sem estado.
type Normalizador struct{}

// Novo devolve o normalizador.
func Novo() Normalizador { return Normalizador{} }

// Normalizar valida dados e devolve a imagem pronta para gravar. Vazio ou
// conteúdo fora de JPEG/PNG/WebP → catalogo.ErrImagemInvalida; entrada
// acima de catalogo.MaxImagemBytes ou saída (depois da conversão) acima de
// catalogo.MaxImagemSaidaBytes → catalogo.ErrImagemMuitoGrande. O erro do
// decoder nunca sobe (só o sentinela).
func (Normalizador) Normalizar(dados []byte) (catalogo.ImagemNormalizada, error) {
	switch {
	case len(dados) == 0:
		return catalogo.ImagemNormalizada{}, catalogo.ErrImagemInvalida
	case len(dados) > catalogo.MaxImagemBytes:
		return catalogo.ImagemNormalizada{}, catalogo.ErrImagemMuitoGrande
	}
	tipo := http.DetectContentType(dados[:min(bytesSniff, len(dados))])
	c, ok := codecs[tipo]
	if !ok {
		vlog.Printf(arqNormalizador, "Normalizar", "tipo detectado recusado bytes=%d", len(dados))
		return catalogo.ImagemNormalizada{}, catalogo.ErrImagemInvalida
	}
	img, err := decodificar(c, dados, tipo)
	if err != nil {
		return catalogo.ImagemNormalizada{}, err
	}
	tipoSaida := c.saida(img)
	saida, err := reencodar(tipoSaida, img)
	if err != nil {
		return catalogo.ImagemNormalizada{}, err
	}
	if len(saida) > catalogo.MaxImagemSaidaBytes {
		vlog.Printf(arqNormalizador, "Normalizar", "saída acima do teto tipo=%s bytes_saida=%d", tipoSaida, len(saida))
		return catalogo.ImagemNormalizada{}, catalogo.ErrImagemMuitoGrande
	}
	vlog.Printf(arqNormalizador, "Normalizar", "imagem aceita tipo=%s tipo_saida=%s bytes_entrada=%d bytes_saida=%d", tipo, tipoSaida, len(dados), len(saida))
	return catalogo.ImagemNormalizada{Tipo: tipoSaida, Extensao: catalogo.ExtensaoDoTipo(tipoSaida), Dados: saida}, nil
}

// decodificar confere as dimensões pelo cabeçalho e só então decodifica a
// imagem inteira (truncada ou corrompida → inválida).
func decodificar(c codec, dados []byte, tipo string) (image.Image, error) {
	cfg, err := c.config(bytes.NewReader(dados))
	if err != nil {
		vlog.Printf(arqNormalizador, "decodificar", "cabeçalho inválido tipo=%s", tipo)
		return nil, catalogo.ErrImagemInvalida
	}
	if !catalogo.DimensoesImagemValidas(cfg.Width, cfg.Height) {
		vlog.Printf(arqNormalizador, "decodificar", "dimensões recusadas tipo=%s largura=%d altura=%d", tipo, cfg.Width, cfg.Height)
		return nil, catalogo.ErrImagemInvalida
	}
	img, err := c.decode(bytes.NewReader(dados))
	if err != nil {
		vlog.Printf(arqNormalizador, "decodificar", "decode completo falhou tipo=%s", tipo)
		return nil, catalogo.ErrImagemInvalida
	}
	return img, nil
}

// reencodar gera os bytes gravados no formato de saída (D1, SEC-34): nada
// do arquivo original passa adiante, só os pixels decodificados.
func reencodar(tipoSaida string, img image.Image) ([]byte, error) {
	encode, ok := encoders[tipoSaida]
	if !ok {
		return nil, catalogo.ErrImagemInvalida
	}
	var buf bytes.Buffer
	if err := encode(&buf, img); err != nil {
		vlog.Printf(arqNormalizador, "reencodar", "reencode falhou")
		return nil, catalogo.ErrImagemInvalida
	}
	return buf.Bytes(), nil
}

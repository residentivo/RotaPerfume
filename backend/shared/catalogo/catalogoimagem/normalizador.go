// Package catalogoimagem é o adapter de validação de conteúdo da imagem do
// produto (IMG-01, porta catalogoapp.NormalizadorImagens). Ignora filename,
// extensão e Content-Type do cliente: o tipo vem dos magic bytes
// (http.DetectContentType), confirmado pelo DecodeConfig do PRÓPRIO formato,
// com limite de dimensões antes do decode completo (decompression bomb) e
// reencode de JPEG/PNG (remove EXIF e qualquer payload anexado; resolve
// arquivos poliglotas). WebP é decodificado por inteiro e gravado como veio.
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
	// encode nil: grava os bytes originais (WebP, sem encoder na x/image).
	encode func(io.Writer, image.Image) error
}

var codecs = map[string]codec{
	catalogo.TipoImagemJPEG: {config: jpeg.DecodeConfig, decode: jpeg.Decode, encode: encodeJPEG},
	catalogo.TipoImagemPNG:  {config: png.DecodeConfig, decode: png.Decode, encode: png.Encode},
	catalogo.TipoImagemWebP: {config: webp.DecodeConfig, decode: webp.Decode},
}

func encodeJPEG(w io.Writer, img image.Image) error {
	return jpeg.Encode(w, img, &jpeg.Options{Quality: qualidadeJPEG})
}

// Normalizador satisfaz catalogoapp.NormalizadorImagens. Sem estado.
type Normalizador struct{}

// Novo devolve o normalizador.
func Novo() Normalizador { return Normalizador{} }

// Normalizar valida dados e devolve a imagem pronta para gravar. Vazio ou
// conteúdo fora de JPEG/PNG/WebP → catalogo.ErrImagemInvalida; acima de
// catalogo.MaxImagemBytes → catalogo.ErrImagemMuitoGrande. O erro do
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
	saida, err := reencodar(c, img, dados)
	if err != nil {
		return catalogo.ImagemNormalizada{}, err
	}
	vlog.Printf(arqNormalizador, "Normalizar", "imagem aceita tipo=%s bytes_entrada=%d bytes_saida=%d", tipo, len(dados), len(saida))
	return catalogo.ImagemNormalizada{Tipo: tipo, Extensao: catalogo.ExtensaoDoTipo(tipo), Dados: saida}, nil
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

// reencodar gera os bytes gravados: JPEG/PNG reencodados (D1); WebP como
// veio (já decodificado por inteiro).
func reencodar(c codec, img image.Image, original []byte) ([]byte, error) {
	if c.encode == nil {
		return original, nil
	}
	var buf bytes.Buffer
	if err := c.encode(&buf, img); err != nil {
		vlog.Printf(arqNormalizador, "reencodar", "reencode falhou")
		return nil, catalogo.ErrImagemInvalida
	}
	return buf.Bytes(), nil
}

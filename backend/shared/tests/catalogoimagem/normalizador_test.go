package catalogoimagem_test

// IMG-01 (Lote 26, TestBrain): adapter catalogoimagem.Normalizador (spec §3).
//   - tipo pelos magic bytes (filename/Content-Type do cliente não chegam
//     aqui); SVG/HTML/texto/GIF/PDF/ZIP → ErrImagemInvalida;
//   - DecodeConfig do próprio formato, limite de dimensões ANTES do decode
//     (bomb 50000x50000 recusado rápido), decode completo (truncado → 400);
//   - JPEG/PNG reencodados (sem EXIF, sem chunks ancilares e sem payload
//     depois do IEND: poliglota); WebP gravado como veio;
//   - vazio → inválida; > 1 MB → ErrImagemMuitoGrande.

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/webp"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoimagem"
)

func imgGIF(t *testing.T) []byte {
	t.Helper()
	pal := image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White})
	var b bytes.Buffer
	require.NoError(t, gif.Encode(&b, pal, nil))
	return b.Bytes()
}

func TestIMG01_Normalizador_Aceita(t *testing.T) {
	webp1 := imgWebP(1, 1)
	casos := []struct {
		nome         string
		dados        []byte
		tipo, ext    string
		w, h         int
		mesmosBytes  bool
		semConteudos []string
	}{
		{"PNG", imgPNG(t, 20, 10), catalogo.TipoImagemPNG, ".png", 20, 10, false, nil},
		{"JPEG", imgJPEG(t, 32, 24), catalogo.TipoImagemJPEG, ".jpg", 32, 24, false, nil},
		{"PNG paletado (reencodado como PNG)", imgPNGPaletado(t), catalogo.TipoImagemPNG, ".png", 4, 4, false, nil},
		{"WebP lossless 1x1 (gravado como veio)", webp1, catalogo.TipoImagemWebP, ".webp", 1, 1, true, nil},
		{"WebP 300x200", imgWebP(300, 200), catalogo.TipoImagemWebP, ".webp", 300, 200, true, nil},
		{"poliglota PNG + HTML depois do IEND (reencodado, sem payload)",
			append(imgPNG(t, 8, 8), []byte("<html><script>alert('xss')</script></html>")...),
			catalogo.TipoImagemPNG, ".png", 8, 8, false, []string{"<html", "<script", "alert("}},
		{"poliglota JPEG + HTML depois do EOI",
			append(imgJPEG(t, 8, 8), []byte("<html><script>alert(1)</script>")...),
			catalogo.TipoImagemJPEG, ".jpg", 8, 8, false, []string{"<script"}},
		{"PNG com tEXt (metadado removido)",
			inserirAntesDoIEND(imgPNG(t, 8, 8), chunkPNG("tEXt", []byte("Comment\x00segredo-gps-123"))),
			catalogo.TipoImagemPNG, ".png", 8, 8, false, []string{"segredo-gps-123", "tEXt"}},
		{"JPEG com EXIF (APP1 removido)", jpegComEXIF(t, "segredo-exif-456"),
			catalogo.TipoImagemJPEG, ".jpg", 16, 16, false, []string{"segredo-exif-456", "Exif"}},
		{"PNG no limite de lado 4096x1", imgPNGGray(t, 4096, 1), catalogo.TipoImagemPNG, ".png", 4096, 1, false, nil},
		{"PNG 1x4096", imgPNGGray(t, 1, 4096), catalogo.TipoImagemPNG, ".png", 1, 4096, false, nil},
		{"PNG 4000x4000 (exatamente 16.000.000 px)", imgPNGGray(t, 4000, 4000), catalogo.TipoImagemPNG, ".png", 4000, 4000, false, nil},
		{"PNG de exatamente 1 MB", pngComTamanhoExato(t, catalogo.MaxImagemBytes), catalogo.TipoImagemPNG, ".png", 8, 8, false, []string{"xxxxxxxx"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, err := catalogoimagem.Novo().Normalizar(c.dados)
			require.NoError(t, err)
			assert.Equal(t, c.tipo, out.Tipo)
			assert.Equal(t, c.ext, out.Extensao)
			assert.Equal(t, c.ext, catalogo.ExtensaoDoTipo(out.Tipo))
			if c.mesmosBytes {
				assert.Equal(t, c.dados, out.Dados)
			}
			for _, s := range c.semConteudos {
				assert.NotContains(t, string(out.Dados), s)
			}
			// a saída é uma imagem válida do mesmo formato e dimensões
			var cfg image.Config
			switch c.tipo {
			case catalogo.TipoImagemPNG:
				cfg, err = png.DecodeConfig(bytes.NewReader(out.Dados))
			case catalogo.TipoImagemJPEG:
				cfg, err = jpeg.DecodeConfig(bytes.NewReader(out.Dados))
			default:
				cfg, err = webp.DecodeConfig(bytes.NewReader(out.Dados))
			}
			require.NoError(t, err)
			assert.Equal(t, c.w, cfg.Width)
			assert.Equal(t, c.h, cfg.Height)
		})
	}
}

// JPEG é reencodado (q=90): os bytes de saída nunca são os de entrada (q=80).
func TestIMG01_Normalizador_JPEGReencodado(t *testing.T) {
	in := imgJPEG(t, 32, 32)
	out, err := catalogoimagem.Novo().Normalizar(in)
	require.NoError(t, err)
	assert.NotEqual(t, in, out.Dados)
}

func imgPNGPaletado(t *testing.T) []byte {
	t.Helper()
	pal := image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White})
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, pal))
	return b.Bytes()
}

func TestIMG01_Normalizador_Recusa(t *testing.T) {
	pngOK := imgPNG(t, 20, 20)
	jpegOK := imgJPEG(t, 32, 32)
	webpOK := imgWebP(10, 10)
	casos := []struct {
		nome  string
		dados []byte
		err   error
	}{
		{"vazio", nil, catalogo.ErrImagemInvalida},
		{"slice vazia", []byte{}, catalogo.ErrImagemInvalida},
		{"1 MB + 1", append(pngComTamanhoExato(t, catalogo.MaxImagemBytes), 0), catalogo.ErrImagemMuitoGrande},
		{"2 MB de lixo", bytes.Repeat([]byte{0xFF}, 2<<20), catalogo.ErrImagemMuitoGrande},
		{"SVG", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><rect width="1" height="1"/></svg>`), catalogo.ErrImagemInvalida},
		{"SVG sem prólogo", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), catalogo.ErrImagemInvalida},
		{"HTML", []byte(`<!DOCTYPE html><html><body><script>alert(1)</script></body></html>`), catalogo.ErrImagemInvalida},
		{"texto", []byte("isto não é uma imagem"), catalogo.ErrImagemInvalida},
		{"GIF (decoder registrado no processo de teste, mas fora da lista)", imgGIF(t), catalogo.ErrImagemInvalida},
		{"PDF", []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF"), catalogo.ErrImagemInvalida},
		{"ZIP", []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00conteudo"), catalogo.ErrImagemInvalida},
		{"BMP", append([]byte("BM"), make([]byte, 64)...), catalogo.ErrImagemInvalida},
		{"PNG truncado (metade)", pngOK[:len(pngOK)/2], catalogo.ErrImagemInvalida},
		{"PNG sem IEND/IDAT (só cabeçalho válido)", cabecalhoPNG(10, 10), catalogo.ErrImagemInvalida},
		{"só a assinatura PNG", []byte("\x89PNG\r\n\x1a\n"), catalogo.ErrImagemInvalida},
		{"JPEG truncado (metade)", jpegOK[:len(jpegOK)/2], catalogo.ErrImagemInvalida},
		{"só o SOI JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0}, catalogo.ErrImagemInvalida},
		{"magic JPEG com corpo PNG", append([]byte{0xFF, 0xD8, 0xFF}, pngOK...), catalogo.ErrImagemInvalida},
		{"WebP truncado", webpOK[:len(webpOK)-4], catalogo.ErrImagemInvalida},
		{"WebP só cabeçalho RIFF", []byte("RIFF\x04\x00\x00\x00WEBPVP8 "), catalogo.ErrImagemInvalida},
		{"bomb PNG 50000x50000", cabecalhoPNG(50000, 50000), catalogo.ErrImagemInvalida},
		{"PNG 4097x10", imgPNGGray(t, 4097, 10), catalogo.ErrImagemInvalida},
		{"PNG 10x4097", imgPNGGray(t, 10, 4097), catalogo.ErrImagemInvalida},
		{"PNG 4096x3907 (> 16.000.000 px)", imgPNGGray(t, 4096, 3907), catalogo.ErrImagemInvalida},
		{"PNG com dimensão 0 no cabeçalho", cabecalhoPNG(0, 10), catalogo.ErrImagemInvalida},
		{"PNG com altura 0 no cabeçalho", cabecalhoPNG(10, 0), catalogo.ErrImagemInvalida},
		{"WebP 4097x10", imgWebP(4097, 10), catalogo.ErrImagemInvalida},
		{"WebP bomb 16384x16384", imgWebP(16384, 16384), catalogo.ErrImagemInvalida},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, err := catalogoimagem.Novo().Normalizar(c.dados)
			require.ErrorIs(t, err, c.err)
			assert.Empty(t, out.Dados)
			assert.Empty(t, out.Extensao)
		})
	}
}

// O bomb é recusado pelo cabeçalho, sem alocar a imagem: tem de ser rápido.
func TestIMG01_Normalizador_BombRapido(t *testing.T) {
	bomb := cabecalhoPNG(50000, 50000)
	inicio := time.Now()
	for range 100 {
		_, err := catalogoimagem.Novo().Normalizar(bomb)
		require.ErrorIs(t, err, catalogo.ErrImagemInvalida)
	}
	assert.Less(t, time.Since(inicio), 2*time.Second)
}

// O erro do decoder nunca sobe: só o sentinela (mensagem fixa).
func TestIMG01_Normalizador_ErroSoSentinela(t *testing.T) {
	for _, dados := range [][]byte{[]byte("x"), cabecalhoPNG(50000, 50000), imgGIF(t)} {
		_, err := catalogoimagem.Novo().Normalizar(dados)
		require.Error(t, err)
		assert.Equal(t, "arquivo de imagem inválido (use JPEG, PNG ou WebP)", err.Error())
	}
	_, err := catalogoimagem.Novo().Normalizar(make([]byte, catalogo.MaxImagemBytes+1))
	assert.Equal(t, "imagem excede 1 MB", err.Error())
}

package catalogoimagem_test

// IMG-01 (Lote 26, TestBrain): geradores de imagens de teste EM MEMÓRIA
// (nada é gravado em disco). WebP é montado à mão (VP8L lossless com códigos
// de prefixo de um símbolo só: cada pixel ocupa 0 bit), porque a x/image não
// tem encoder de WebP.

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

func imgPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 7), uint8(y * 13), uint8(x ^ y), 255})
		}
	}
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, img))
	return b.Bytes()
}

func imgPNGGray(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))))
	return b.Bytes()
}

func imgJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x * 5), uint8(y * 3), 128, 255})
		}
	}
	var b bytes.Buffer
	require.NoError(t, jpeg.Encode(&b, img, &jpeg.Options{Quality: 80}))
	return b.Bytes()
}

// chunkPNG monta um chunk PNG (tamanho, tipo, dados, CRC).
func chunkPNG(tipo string, dados []byte) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, uint32(len(dados)))
	b.WriteString(tipo)
	b.Write(dados)
	crc := crc32.NewIEEE()
	crc.Write([]byte(tipo))
	crc.Write(dados)
	_ = binary.Write(&b, binary.BigEndian, crc.Sum32())
	return b.Bytes()
}

// cabecalhoPNG é a assinatura + IHDR declarando w x h (RGBA 8 bits), sem
// IDAT: o DecodeConfig lê as dimensões e o decode completo nunca deve rodar.
func cabecalhoPNG(w, h uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6
	out := []byte("\x89PNG\r\n\x1a\n")
	out = append(out, chunkPNG("IHDR", ihdr)...)
	return append(out, chunkPNG("IEND", nil)...)
}

// inserirAntesDoIEND põe um chunk antes do IEND de um PNG válido.
func inserirAntesDoIEND(pngBytes, chunk []byte) []byte {
	pos := len(pngBytes) - 12 // IEND: 4 tamanho + 4 tipo + 4 crc
	out := append([]byte{}, pngBytes[:pos]...)
	out = append(out, chunk...)
	return append(out, pngBytes[pos:]...)
}

// pngComTamanhoExato devolve um PNG válido de exatamente n bytes (preenche
// com um chunk ancilar "teSt" antes do IEND).
func pngComTamanhoExato(t *testing.T, n int) []byte {
	t.Helper()
	base := imgPNG(t, 8, 8)
	falta := n - len(base) - 12
	require.GreaterOrEqual(t, falta, 0)
	out := inserirAntesDoIEND(base, chunkPNG("teSt", bytes.Repeat([]byte{'x'}, falta)))
	require.Len(t, out, n)
	return out
}

// jpegComEXIF insere um segmento APP1 "Exif" com o texto logo depois do SOI.
func jpegComEXIF(t *testing.T, texto string) []byte {
	t.Helper()
	base := imgJPEG(t, 16, 16)
	payload := append([]byte("Exif\x00\x00"), []byte(texto)...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	seg = append(seg, payload...)
	out := append([]byte{}, base[:2]...)
	out = append(out, seg...)
	return append(out, base[2:]...)
}

// bitsLSB é o escritor de bits do VP8L (LSB primeiro).
type bitsLSB struct {
	buf  []byte
	acc  uint64
	nAcc uint
}

func (b *bitsLSB) put(v uint64, n uint) {
	b.acc |= v << b.nAcc
	b.nAcc += n
	for b.nAcc >= 8 {
		b.buf = append(b.buf, byte(b.acc))
		b.acc >>= 8
		b.nAcc -= 8
	}
}

func (b *bitsLSB) bytes() []byte {
	if b.nAcc > 0 {
		b.buf = append(b.buf, byte(b.acc))
		b.acc, b.nAcc = 0, 0
	}
	return b.buf
}

// imgWebP monta um WebP lossless (VP8L) w x h de cor única. Cada código de
// prefixo é "simples" com um símbolo de 8 bits; o pixel não ocupa bits.
func imgWebP(w, h int) []byte {
	var bw bitsLSB
	bw.put(0x2f, 8)
	bw.put(uint64(w-1), 14)
	bw.put(uint64(h-1), 14)
	bw.put(0, 1)                                              // alpha_is_used
	bw.put(0, 3)                                              // versão
	bw.put(0, 1)                                              // sem transform
	bw.put(0, 1)                                              // sem color cache
	bw.put(0, 1)                                              // sem meta prefix codes
	for _, sym := range []uint64{0x40, 0x80, 0xC0, 0xFF, 0} { // verde, vermelho, azul, alfa, distância
		bw.put(1, 1) // código simples
		bw.put(0, 1) // 1 símbolo
		bw.put(1, 1) // símbolo de 8 bits
		bw.put(sym, 8)
	}
	dados := bw.bytes()
	return riffWebP("VP8L", dados)
}

func riffWebP(chunk string, dados []byte) []byte {
	var c bytes.Buffer
	c.WriteString(chunk)
	_ = binary.Write(&c, binary.LittleEndian, uint32(len(dados)))
	c.Write(dados)
	if len(dados)%2 == 1 {
		c.WriteByte(0)
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(4+c.Len()))
	out.WriteString("WEBP")
	out.Write(c.Bytes())
	return out.Bytes()
}

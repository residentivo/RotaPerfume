package catalogoimagem_test

// SEC-34 (Lote 31), teto de saída: uma entrada WebP abaixo de
// catalogo.MaxImagemBytes (1 MiB) cuja conversão (PNG ou JPEG q90) passa de
// catalogo.MaxImagemSaidaBytes (4 MiB) é recusada com ErrImagemMuitoGrande.
//
// A entrada é um VP8L com P linhas de ruído e o resto da imagem em cópias
// LZ77 de distância P*W pixels (uma cópia por linha). O VP8L aceita
// distâncias de até ~1 M pixels; o deflate do PNG tem janela de 32 KiB e o
// JPEG não tem referência entre blocos, então a saída é ruído "cheio".

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"math/bits"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/webp"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoimagem"
)

// sec34Prefixo codifica n ≥ 1 como (prefixo, bits extras, valor extra) no
// esquema de prefixo LZ77 do VP8L (o inverso do lz77Param do decoder).
func sec34Prefixo(n int) (prefixo, nExtra int, extra uint64) {
	v := n - 1
	if v < 4 {
		return v, 0, 0
	}
	h := bits.Len(uint(v)) - 1
	segundo := (v >> (h - 1)) & 1
	prefixo = 2*h + segundo
	nExtra = h - 1
	return prefixo, nExtra, uint64(v - (2+segundo)<<(h-1))
}

// sec34Codigo grava um código Huffman canônico: o decoder lê primeiro o bit
// mais significativo.
func sec34Codigo(bw *bitsLSB, codigo uint64, tamanho int) {
	for i := tamanho - 1; i >= 0; i-- {
		bw.put((codigo>>i)&1, 1)
	}
}

// sec34CodigoNormal grava uma árvore "normal" a partir dos tamanhos de
// código (só 0, 8 e 9). O código dos tamanhos é: 8 → "0", 0 → "10", 9 → "11".
func sec34CodigoNormal(bw *bitsLSB, tamanhos []int) {
	bw.put(0, 1) // código normal
	// Ordem do kCodeLengthCodeOrder: 17,18,0,1,2,3,4,5,16,6,7,8,9 (13 itens).
	ordem := []int{17, 18, 0, 1, 2, 3, 4, 5, 16, 6, 7, 8, 9}
	tamanhoDoSimbolo := map[int]uint64{0: 2, 8: 1, 9: 2}
	bw.put(uint64(len(ordem)-4), 4)
	for _, s := range ordem {
		bw.put(tamanhoDoSimbolo[s], 3)
	}
	bw.put(0, 1) // sem max_symbol: lê todos os tamanhos do alfabeto
	for _, t := range tamanhos {
		switch t {
		case 8:
			sec34Codigo(bw, 0b0, 1)
		case 0:
			sec34Codigo(bw, 0b10, 2)
		case 9:
			sec34Codigo(bw, 0b11, 2)
		default:
			panic("tamanho fora do fixture")
		}
	}
}

// sec34WebPRepetido monta o VP8L w x h descrito no cabeçalho do arquivo.
// opaco=false sorteia o alfa (saída PNG); opaco=true usa alfa 0xFF (JPEG).
func sec34WebPRepetido(w, h, linhasRuido int, opaco bool) []byte {
	rng := rand.New(rand.NewPCG(34, 2026))
	var bw bitsLSB
	bw.put(0x2f, 8)
	bw.put(uint64(w-1), 14)
	bw.put(uint64(h-1), 14)
	bw.put(1, 1) // alpha_is_used
	bw.put(0, 3) // versão
	bw.put(0, 1) // sem transform
	bw.put(0, 1) // sem color cache
	bw.put(0, 1) // sem meta prefix codes

	prefixoLen, extraLenBits, extraLen := sec34Prefixo(w)
	simboloLen := 256 + prefixoLen
	prefixoDist, extraDistBits, extraDist := sec34Prefixo(linhasRuido*w + 120)

	// Verde: literais 0..254 com 8 bits, 255 e o símbolo de comprimento com
	// 9 bits (Kraft: 255/256 + 2/512 = 1). Canônico: 0..254 → 0..254,
	// 255 → 510, comprimento → 511.
	verde := make([]int, 256+24)
	for i := range 255 {
		verde[i] = 8
	}
	verde[255], verde[simboloLen] = 9, 9
	sec34CodigoNormal(&bw, verde)
	plano := make([]int, 256) // vermelho, azul, alfa: 8 bits, código = valor
	for i := range plano {
		plano[i] = 8
	}
	for range 3 {
		sec34CodigoNormal(&bw, plano)
	}
	// Distância: código simples com um símbolo (não ocupa bits por pixel).
	bw.put(1, 1)
	bw.put(0, 1)
	bw.put(1, 1)
	bw.put(uint64(prefixoDist), 8)

	verdeLiteral := func(g uint64) {
		if g == 255 {
			sec34Codigo(&bw, 510, 9)
			return
		}
		sec34Codigo(&bw, g, 8)
	}
	for range linhasRuido * w {
		verdeLiteral(rng.Uint64N(256))
		sec34Codigo(&bw, rng.Uint64N(256), 8) // vermelho
		sec34Codigo(&bw, rng.Uint64N(256), 8) // azul
		alfa := uint64(0xFF)
		if !opaco {
			alfa = rng.Uint64N(256)
		}
		sec34Codigo(&bw, alfa, 8)
	}
	for range h - linhasRuido {
		sec34Codigo(&bw, 511, 9) // comprimento = w (uma linha)
		bw.put(extraLen, uint(extraLenBits))
		bw.put(extraDist, uint(extraDistBits))
	}
	return riffWebP("VP8L", bw.bytes())
}

func TestSEC34_SaidaAcimaDoTeto_ErrImagemMuitoGrande(t *testing.T) {
	if testing.Short() {
		t.Skip("decode/encode de 4 M pixels")
	}
	casos := []struct {
		nome   string
		opaco  bool
		w, h   int
		encode func(*bytes.Buffer, image.Image) error
	}{
		{"transparente → PNG", false, 2048, 2048, func(b *bytes.Buffer, img image.Image) error { return png.Encode(b, img) }},
		// O JPEG (4:2:0) comprime mais o ruído: precisa de mais área.
		{"opaco → JPEG q90", true, 4096, 2048, func(b *bytes.Buffer, img image.Image) error {
			return jpeg.Encode(b, img, &jpeg.Options{Quality: 90})
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			const ruido = 8
			w, h := c.w, c.h
			entrada := sec34WebPRepetido(w, h, ruido, c.opaco)
			require.LessOrEqual(t, len(entrada), catalogo.MaxImagemBytes, "pré-condição: a entrada passa no teto de entrada")

			// Pré-condição: o fixture é um WebP válido e a conversão
			// realmente estoura o teto de saída.
			img, err := webp.Decode(bytes.NewReader(entrada))
			require.NoError(t, err)
			require.Equal(t, image.Rect(0, 0, w, h), img.Bounds())
			var conv bytes.Buffer
			require.NoError(t, c.encode(&conv, img))
			require.Greater(t, conv.Len(), catalogo.MaxImagemSaidaBytes, "pré-condição: a saída passa de 4 MiB")

			out, err := catalogoimagem.Novo().Normalizar(entrada)
			require.Error(t, err)
			assert.True(t, errors.Is(err, catalogo.ErrImagemMuitoGrande), "erro: %v", err)
			assert.Empty(t, out.Dados, "nada é devolvido para gravar")
			assert.Empty(t, out.Tipo)
		})
	}
}

// Controle: o mesmo fixture com poucas linhas fica abaixo do teto e é
// aceito (o teto de saída não recusa conversões legítimas).
func TestSEC34_SaidaAbaixoDoTeto_Aceita(t *testing.T) {
	for nome, opaco := range map[string]bool{"transparente → PNG": false, "opaco → JPEG": true} {
		t.Run(nome, func(t *testing.T) {
			entrada := sec34WebPRepetido(256, 64, 8, opaco)
			out, err := catalogoimagem.Novo().Normalizar(entrada)
			require.NoError(t, err)
			assert.LessOrEqual(t, len(out.Dados), catalogo.MaxImagemSaidaBytes)
			want := catalogo.TipoImagemPNG
			if opaco {
				want = catalogo.TipoImagemJPEG
			}
			assert.Equal(t, want, out.Tipo)
		})
	}
}

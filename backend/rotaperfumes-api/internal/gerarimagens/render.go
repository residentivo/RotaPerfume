package gerarimagens

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Geometria da imagem gerada: quadrado de LadoPx com o nome na diagonal
// (subindo da esquerda para a direita, -45°).
const (
	LadoPx         = 600
	anguloGraus    = -45.0
	larguraLinhaPx = 560
	alturaBlocoMax = 260
	maxLinhas      = 3
	tamanhoMax     = 76.0
	tamanhoMin     = 22.0
	passoTamanho   = 2.0
	tamanhoRodape  = 22.0
	margemMascara  = 12
	sombraPx       = 3
	reticencias    = "…"
)

var (
	fonteOnce sync.Once
	fonte     *opentype.Font
	fonteErr  error
)

func carregarFonte() (*opentype.Font, error) {
	fonteOnce.Do(func() { fonte, fonteErr = opentype.Parse(gobold.TTF) })
	return fonte, fonteErr
}

func novaFace(tamanho float64) (font.Face, error) {
	f, err := carregarFonte()
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: tamanho, DPI: 72, Hinting: font.HintingFull})
}

// RenderizarPNG desenha o PNG do produto: fundo em degradê com a cor
// derivada do SKU, o nome (descrição) em branco na diagonal, quebrado em até
// 3 linhas e truncado com "…" se não couber, e o SKU no rodapé.
func RenderizarPNG(descricao, sku string) ([]byte, error) {
	texto := strings.Join(strings.Fields(descricao), " ")
	if texto == "" {
		texto = sku
	}
	face, linhas, err := ajustarTexto(texto)
	if err != nil {
		return nil, err
	}
	defer face.Close()

	img := image.NewRGBA(image.Rect(0, 0, LadoPx, LadoPx))
	pintarFundo(img, sku)
	mascara := desenharMascara(face, linhas)
	compor(img, mascara, sombraPx, color.RGBA{0, 0, 0, 90})
	compor(img, mascara, 0, color.RGBA{255, 255, 255, 255})
	if err := desenharRodape(img, sku); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("gerarimagens: encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// ajustarTexto escolhe o maior tamanho de fonte em que o texto cabe em até
// maxLinhas linhas de larguraLinhaPx; no mínimo, trunca.
func ajustarTexto(texto string) (font.Face, []string, error) {
	for tam := tamanhoMax; tam >= tamanhoMin; tam -= passoTamanho {
		face, err := novaFace(tam)
		if err != nil {
			return nil, nil, fmt.Errorf("gerarimagens: fonte: %w", err)
		}
		linhas := quebrarLinhas(face, texto)
		altura := len(linhas) * face.Metrics().Height.Ceil()
		if len(linhas) <= maxLinhas && altura <= alturaBlocoMax && cabem(face, linhas) {
			return face, linhas, nil
		}
		_ = face.Close()
	}
	face, err := novaFace(tamanhoMin)
	if err != nil {
		return nil, nil, fmt.Errorf("gerarimagens: fonte: %w", err)
	}
	return face, truncar(face, quebrarLinhas(face, texto)), nil
}

func largura(face font.Face, s string) int { return font.MeasureString(face, s).Ceil() }

func cabem(face font.Face, linhas []string) bool {
	for _, l := range linhas {
		if largura(face, l) > larguraLinhaPx {
			return false
		}
	}
	return true
}

// quebrarLinhas faz a quebra gulosa por palavras.
func quebrarLinhas(face font.Face, texto string) []string {
	var linhas []string
	atual := ""
	for _, palavra := range strings.Fields(texto) {
		candidata := strings.TrimSpace(atual + " " + palavra)
		if atual != "" && largura(face, candidata) > larguraLinhaPx {
			linhas = append(linhas, atual)
			atual = palavra
			continue
		}
		atual = candidata
	}
	if atual != "" {
		linhas = append(linhas, atual)
	}
	return linhas
}

// truncar limita a maxLinhas e corta cada linha (com "…") até caber.
func truncar(face font.Face, linhas []string) []string {
	cortou := len(linhas) > maxLinhas
	if cortou {
		linhas = linhas[:maxLinhas]
	}
	for i, l := range linhas {
		ultima := i == len(linhas)-1
		if largura(face, l) <= larguraLinhaPx && !(ultima && cortou) {
			continue
		}
		r := []rune(l)
		for len(r) > 0 && largura(face, string(r)+reticencias) > larguraLinhaPx {
			r = r[:len(r)-1]
		}
		linhas[i] = strings.TrimSpace(string(r)) + reticencias
	}
	return linhas
}

// desenharMascara escreve as linhas centralizadas numa máscara alfa.
func desenharMascara(face font.Face, linhas []string) *image.Alpha {
	m := face.Metrics()
	alturaLinha := m.Height.Ceil()
	w := larguraLinhaPx + 2*margemMascara
	h := len(linhas)*alturaLinha + 2*margemMascara
	mascara := image.NewAlpha(image.Rect(0, 0, w, h))
	d := &font.Drawer{Dst: mascara, Src: image.Opaque, Face: face}
	for i, l := range linhas {
		x := (w - largura(face, l)) / 2
		y := margemMascara + i*alturaLinha + m.Ascent.Ceil()
		d.Dot = fixed.P(x, y)
		d.DrawString(l)
	}
	return mascara
}

// compor gira a máscara em anguloGraus em torno do centro da imagem
// (amostragem bilinear pelo mapeamento inverso) e a aplica com a cor c,
// deslocada de desloc pixels (sombra).
func compor(img *image.RGBA, mascara *image.Alpha, desloc int, c color.RGBA) {
	rad := anguloGraus * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	mb := mascara.Bounds()
	mcx, mcy := float64(mb.Dx())/2, float64(mb.Dy())/2
	centro := float64(LadoPx) / 2
	for y := range LadoPx {
		for x := range LadoPx {
			dx := float64(x-desloc) - centro + 0.5
			dy := float64(y-desloc) - centro + 0.5
			sx := cos*dx + sin*dy + mcx - 0.5
			sy := -sin*dx + cos*dy + mcy - 0.5
			a := amostrar(mascara, sx, sy)
			if a == 0 {
				continue
			}
			misturar(img, x, y, c, a*float64(c.A)/255)
		}
	}
}

// amostrar faz a interpolação bilinear do alfa (0 fora da máscara).
func amostrar(m *image.Alpha, x, y float64) float64 {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	v := func(px, py int) float64 {
		if !(image.Point{px, py}.In(m.Rect)) {
			return 0
		}
		return float64(m.AlphaAt(px, py).A) / 255
	}
	topo := v(x0, y0)*(1-fx) + v(x0+1, y0)*fx
	base := v(x0, y0+1)*(1-fx) + v(x0+1, y0+1)*fx
	return topo*(1-fy) + base*fy
}

func misturar(img *image.RGBA, x, y int, c color.RGBA, a float64) {
	o := img.RGBAAt(x, y)
	mix := func(fundo, frente uint8) uint8 { return uint8(float64(fundo)*(1-a) + float64(frente)*a + 0.5) }
	img.SetRGBA(x, y, color.RGBA{mix(o.R, c.R), mix(o.G, c.G), mix(o.B, c.B), 255})
}

// pintarFundo faz um degradê diagonal na matiz derivada do SKU (escura o
// bastante para o texto branco).
func pintarFundo(img *image.RGBA, sku string) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(sku))
	matiz := float64(h.Sum32() % 360)
	claro, escuro := hsl(matiz, 0.55, 0.42), hsl(math.Mod(matiz+35, 360), 0.60, 0.24)
	for y := range LadoPx {
		for x := range LadoPx {
			t := float64(x+y) / float64(2*(LadoPx-1))
			img.SetRGBA(x, y, color.RGBA{
				uint8(float64(claro.R)*(1-t) + float64(escuro.R)*t),
				uint8(float64(claro.G)*(1-t) + float64(escuro.G)*t),
				uint8(float64(claro.B)*(1-t) + float64(escuro.B)*t),
				255,
			})
		}
	}
}

// desenharRodape escreve o SKU centralizado no rodapé.
func desenharRodape(img *image.RGBA, sku string) error {
	if strings.TrimSpace(sku) == "" {
		return nil
	}
	face, err := novaFace(tamanhoRodape)
	if err != nil {
		return fmt.Errorf("gerarimagens: fonte: %w", err)
	}
	defer face.Close()
	texto := strings.TrimSpace(sku)
	d := &font.Drawer{Dst: img, Src: image.NewUniform(color.NRGBA{255, 255, 255, 210}), Face: face}
	d.Dot = fixed.P((LadoPx-largura(face, texto))/2, LadoPx-24)
	d.DrawString(texto)
	return nil
}

// hsl converte HSL (h em graus, s e l em [0,1]) para RGB.
func hsl(h, s, l float64) color.RGBA {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	conv := func(v float64) uint8 { return uint8(math.Round((v + m) * 255)) }
	return color.RGBA{conv(r), conv(g), conv(b), 255}
}

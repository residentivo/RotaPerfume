package catalogo

import "regexp"

// Limites da imagem do produto (IMG-01). O tamanho vale para o arquivo
// enviado; as dimensões são conferidas pelo cabeçalho ANTES do decode
// completo (proteção contra decompression bomb).
const (
	MaxImagemBytes     = 1 << 20
	MaxLadoImagemPx    = 4096
	MaxPixelsImagem    = 16_000_000
	TipoImagemJPEG     = "image/jpeg"
	TipoImagemPNG      = "image/png"
	TipoImagemWebP     = "image/webp"
	extensaoImagemJPEG = ".jpg"
	extensaoImagemPNG  = ".png"
	extensaoImagemWebP = ".webp"
)

// padraoNomeImagem é o único formato de nome aceito em disco e no banco:
// UUID v4 minúsculo + a extensão do tipo DETECTADO. É o mesmo regex do
// CHECK chk_produtos_imagem (sql/26_alter_produtos_imagem.sql).
var padraoNomeImagem = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\.(jpg|png|webp)$`)

// NomeImagemValido informa se nome segue o padrão <uuid-v4><ext>.
func NomeImagemValido(nome string) bool { return padraoNomeImagem.MatchString(nome) }

// ExtensaoDoTipo devolve a extensão gravada para o tipo detectado ("" se o
// tipo não for aceito).
func ExtensaoDoTipo(tipo string) string {
	switch tipo {
	case TipoImagemJPEG:
		return extensaoImagemJPEG
	case TipoImagemPNG:
		return extensaoImagemPNG
	case TipoImagemWebP:
		return extensaoImagemWebP
	default:
		return ""
	}
}

// TipoDoNome devolve o Content-Type pelo mapa fixo de extensões ("" se o
// nome não for válido). Nunca usa mime.TypeByExtension.
func TipoDoNome(nome string) string {
	if !NomeImagemValido(nome) {
		return ""
	}
	for _, tipo := range []string{TipoImagemJPEG, TipoImagemPNG, TipoImagemWebP} {
		ext := ExtensaoDoTipo(tipo)
		if len(nome) > len(ext) && nome[len(nome)-len(ext):] == ext {
			return tipo
		}
	}
	return ""
}

// DimensoesImagemValidas aplica 1 ≤ W,H ≤ MaxLadoImagemPx e
// W*H ≤ MaxPixelsImagem.
func DimensoesImagemValidas(largura, altura int) bool {
	if largura < 1 || altura < 1 || largura > MaxLadoImagemPx || altura > MaxLadoImagemPx {
		return false
	}
	return largura*altura <= MaxPixelsImagem
}

// ImagemNormalizada é a imagem já validada e (JPEG/PNG) reencodada, pronta
// para gravar: o nome em disco usa só Extensao, nunca o do cliente.
type ImagemNormalizada struct {
	Tipo     string
	Extensao string
	Dados    []byte
}

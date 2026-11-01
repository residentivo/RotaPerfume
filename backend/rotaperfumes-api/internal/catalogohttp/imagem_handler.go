package catalogohttp

import (
	"errors"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
)

// IMG-01: upload, remoção e leitura pública da imagem do produto.

const (
	// MaxCorpoUploadBytes é o teto do corpo multipart: a imagem (1 MB) mais
	// a folga de boundary e cabeçalhos das partes.
	MaxCorpoUploadBytes = catalogo.MaxImagemBytes + 64<<10
	// campoImagem é o único campo aceito no multipart.
	campoImagem = "imagem"
	// headerAntiCSRF/valorAntiCSRF (D2): um formulário HTML cross-site não
	// consegue enviar este header sem preflight CORS.
	headerAntiCSRF = "X-Requested-With"
	valorAntiCSRF  = "XMLHttpRequest"
	// PrefixoURLImagem é o caminho público das imagens (rota pública).
	PrefixoURLImagem = "/api/public/produtos/imagens/"
)

const (
	msgImagemMuitoGrande = "imagem excede 1 MB"
	msgUploadProibido    = "requisição de upload recusada"
)

// EnviarImagem POST /api/produtos/{id}/imagem (multipart, campo "imagem",
// até 1 MB) → 200 com o produto. O nome e o Content-Type enviados pelo
// cliente são ignorados.
func (h *ProdutoHandler) EnviarImagem(w http.ResponseWriter, r *http.Request) {
	const tag = "[produtos] EnviarImagem"
	principal, ok := principalOuErro(w, r, tag)
	if !ok {
		return
	}
	id, err := httpx.IDDoPath(r, "id")
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	if !h.uploadPermitido(r) {
		log.Printf("[produtos] upload de imagem recusado (anti-CSRF): id=%d por usuario role=%s", id, principal.Role())
		httpx.JSON(w, http.StatusForbidden, nil, msgUploadProibido)
		return
	}
	dados, err := lerImagemMultipart(w, r)
	if err != nil {
		responderErroImagem(w, tag, err)
		return
	}
	p, err := h.casos.DefinirImagemProduto(r.Context(), id, dados)
	if err != nil {
		responderErroImagem(w, tag, err)
		return
	}
	log.Printf("[produtos] imagem definida: id=%d arquivo=%s bytes_recebidos=%d por usuario role=%s", id, p.Imagem, len(dados), principal.Role())
	httpx.JSON(w, http.StatusOK, produtoParaResposta(*p), "")
}

// RemoverImagem DELETE /api/produtos/{id}/imagem → 200 com o produto
// (idempotente; 404 se o produto não existir).
func (h *ProdutoHandler) RemoverImagem(w http.ResponseWriter, r *http.Request) {
	const tag = "[produtos] RemoverImagem"
	principal, ok := principalOuErro(w, r, tag)
	if !ok {
		return
	}
	id, err := httpx.IDDoPath(r, "id")
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	p, err := h.casos.RemoverImagemProduto(r.Context(), id)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	log.Printf("[produtos] imagem zerada: id=%d por usuario role=%s", id, principal.Role())
	httpx.JSON(w, http.StatusOK, produtoParaResposta(*p), "")
}

// uploadPermitido exige X-Requested-With: XMLHttpRequest e, se houver
// Origin, que ela seja a do próprio host ou uma das origens CORS.
func (h *ProdutoHandler) uploadPermitido(r *http.Request) bool {
	if r.Header.Get(headerAntiCSRF) != valorAntiCSRF {
		return false
	}
	origem := r.Header.Get("Origin")
	if origem == "" || origem == "https://"+r.Host || origem == "http://"+r.Host {
		return true
	}
	for _, o := range h.origensUpload {
		if origem == o {
			return true
		}
	}
	return false
}

// lerImagemMultipart lê a única parte "imagem" do multipart/form-data sem
// tocar o disco (sem ParseMultipartForm). Content-Length acima do teto →
// ErrImagemMuitoGrande sem ler o corpo.
func lerImagemMultipart(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.ContentLength > MaxCorpoUploadBytes {
		return nil, catalogo.ErrImagemMuitoGrande
	}
	tipo, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || tipo != "multipart/form-data" || params["boundary"] == "" {
		return nil, catalogo.ErrImagemInvalida
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxCorpoUploadBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, catalogo.ErrImagemInvalida
	}
	dados, err := lerParteImagem(mr)
	if err != nil {
		return nil, err
	}
	if _, err := mr.NextPart(); !errors.Is(err, io.EOF) {
		return nil, erroDeLeitura(err, catalogo.ErrImagemInvalida) // segunda parte
	}
	return dados, nil
}

// lerParteImagem lê a primeira parte, que tem de ser o campo "imagem".
func lerParteImagem(mr *multipart.Reader) ([]byte, error) {
	part, err := mr.NextPart()
	if err != nil {
		return nil, erroDeLeitura(err, catalogo.ErrImagemInvalida)
	}
	defer part.Close()
	if part.FormName() != campoImagem {
		return nil, catalogo.ErrImagemInvalida
	}
	dados, err := io.ReadAll(io.LimitReader(part, catalogo.MaxImagemBytes+1))
	if err != nil {
		return nil, erroDeLeitura(err, catalogo.ErrImagemInvalida)
	}
	switch {
	case len(dados) > catalogo.MaxImagemBytes:
		return nil, catalogo.ErrImagemMuitoGrande
	case len(dados) == 0:
		return nil, catalogo.ErrImagemInvalida
	}
	return dados, nil
}

// erroDeLeitura traduz o estouro do MaxBytesReader em 413; o resto vira
// padrao.
func erroDeLeitura(err, padrao error) error {
	var excedeu *http.MaxBytesError
	if errors.As(err, &excedeu) {
		return catalogo.ErrImagemMuitoGrande
	}
	return padrao
}

// responderErroImagem: excesso de tamanho é 413 com a mensagem fixa; o
// resto segue o mapper (400 inválida, 404 produto, 500).
func responderErroImagem(w http.ResponseWriter, tag string, err error) {
	if errors.Is(err, catalogo.ErrImagemMuitoGrande) {
		httpx.JSON(w, http.StatusRequestEntityTooLarge, nil, msgImagemMuitoGrande)
		return
	}
	httpx.ResponderErro(w, tag, err)
}

// ImagemPublicaHandler serve GET /api/public/produtos/imagens/{arquivo}
// (rota pública, sem auth nem cookies). Não usa http.FileServer: o nome é
// conferido pelo regex, o Content-Type vem do mapa fixo e o arquivo sai por
// http.ServeContent (HEAD, Range e If-Modified-Since).
type ImagemPublicaHandler struct{ casos *catalogoapp.Casos }

// NovoImagemPublicaHandler exige os casos de uso (nil: panic na montagem).
func NovoImagemPublicaHandler(c *catalogoapp.Casos) *ImagemPublicaHandler {
	if c == nil {
		panic("catalogohttp: NovoImagemPublicaHandler exige os casos de uso")
	}
	return &ImagemPublicaHandler{casos: c}
}

// Servir GET /api/public/produtos/imagens/{arquivo}
func (h *ImagemPublicaHandler) Servir(w http.ResponseWriter, r *http.Request) {
	nome := r.PathValue("arquivo")
	tipo := catalogo.TipoDoNome(nome)
	if tipo == "" {
		httpx.ResponderErro(w, "[produtos] ServirImagem", catalogo.ErrImagemNaoEncontrada)
		return
	}
	f, modificado, err := h.casos.AbrirImagemProduto(nome)
	if err != nil {
		httpx.ResponderErro(w, "[produtos] ServirImagem", catalogo.ErrImagemNaoEncontrada)
		return
	}
	defer f.Close()
	hd := w.Header()
	hd.Set("Content-Type", tipo)
	hd.Set("Cache-Control", "public, max-age=31536000, immutable")
	hd.Set("Cross-Origin-Resource-Policy", "same-site")
	hd.Set("Content-Disposition", "inline")
	http.ServeContent(w, r, "", modificado, f)
}

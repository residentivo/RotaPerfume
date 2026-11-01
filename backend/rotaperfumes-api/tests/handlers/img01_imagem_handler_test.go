package handlers_test

// IMG-01 (Lote 26, TestBrain): contrato HTTP da imagem do produto (spec §1,
// §2, §3, §5 e §9), com o router real, sqlmock e o filesystem real num
// t.TempDir():
//   - POST: policy → id → anti-CSRF (X-Requested-With/Origin) → corpo;
//     413 (Content-Length sem ler, parte > 1 MB, chunked), 400 com mensagem
//     fixa para tudo que não é JPEG/PNG/WebP de verdade ou multipart
//     malformado; nome em disco <uuid-v4><ext do tipo detectado>; filename
//     do cliente nunca aparece em disco/banco/log/resposta;
//   - troca apaga a antiga; produto inexistente/falha no UPDATE/commit não
//     deixam arquivo;
//   - DELETE: zera e apaga, idempotente, 404;
//   - GET público: headers de segurança, HEAD, Range, 304; 404 para
//     traversal, maiúsculas, extensões fora do mapa, temporários, UUID v1,
//     symlink; 405 para outros métodos.

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/rotaperfumes-api/internal/catalogohttp"
	"github.com/rotaperfumes/shared/catalogo"
)

var reImg01NomeV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\.(jpg|png|webp)$`)

const (
	img01Msg400 = `{"success":false,"error":"arquivo de imagem inválido (use JPEG, PNG ou WebP)"}`
	img01Msg413 = `{"success":false,"error":"imagem excede 1 MB"}`
	img01Msg403 = `{"success":false,"error":"requisição de upload recusada"}`
)

func img01ImagemURL(t *testing.T, corpo []byte) any {
	t.Helper()
	var env struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(corpo, &env))
	v, ok := env.Data["imagem_url"]
	require.True(t, ok, "imagem_url tem de estar no JSON: %s", corpo)
	return v
}

// ---------------------------------------------------------------------------
// POST /api/produtos/{id}/imagem — sucesso
// ---------------------------------------------------------------------------

func TestIMG01_Upload_Aceita_GravaPeloTipoDetectado(t *testing.T) {
	poliglota := append(img01PNG(t, 8, 8), []byte("<html><script>alert('x')</script></html>")...)
	casos := []struct {
		nome, filename, tipo string
		dados                []byte
		ext, tipoDetectado   string
	}{
		{"PNG", "foto.png", "image/png", img01PNG(t, 10, 10), ".png", "image/png"},
		{"PNG enviado como foto.jpg (image/jpeg)", "foto.jpg", "image/jpeg", img01PNG(t, 10, 10), ".png", "image/png"},
		{"JPEG", "foto.jpeg", "image/jpeg", img01JPEG(t, 16, 16), ".jpg", "image/jpeg"},
		{"JPEG como .png e Content-Type text/plain", "x.png", "text/plain", img01JPEG(t, 16, 16), ".jpg", "image/jpeg"},
		{"WebP", "a.webp", "image/webp", img01WebP(), ".webp", "image/webp"},
		{"sem filename nem Content-Type", "", "", img01PNG(t, 3, 3), ".png", "image/png"},
		{"poliglota PNG + HTML depois do IEND", "x.png", "image/png", poliglota, ".png", "image/png"},
		{"exatamente 1 MB", "grande.png", "image/png", img01PNGExato(t, catalogo.MaxImagemBytes), ".png", "image/png"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := img01Setup(t)
			var gravado driver.Value
			img01EsperarTroca(e.mock, "", img01Fixo, &gravado)

			w := e.serve(img01UploadImagem(t, c.dados, c.filename, c.tipo))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, img01PrefixoURL+img01Fixo, img01ImagemURL(t, w.Body.Bytes()))
			require.NoError(t, e.mock.ExpectationsWereMet())

			nome, ok := gravado.(string)
			require.True(t, ok, "UPDATE recebeu %T", gravado)
			assert.Regexp(t, reImg01NomeV4, nome)
			assert.True(t, strings.HasSuffix(nome, c.ext), "extensão do tipo detectado: %s", nome)
			assert.Equal(t, []string{nome}, e.arquivos(t), "só o arquivo final, sem .tmp-")

			salvo, err := os.ReadFile(filepath.Join(e.dir, nome))
			require.NoError(t, err)
			assert.Equal(t, c.tipoDetectado, http.DetectContentType(salvo))
			assert.NotContains(t, string(salvo), "<script")
			assert.NotContains(t, string(salvo), "<html")
			assert.Contains(t, e.logs.String(), "[produtos] imagem definida: id=1")
		})
	}
}

// filename hostil nunca chega ao disco, ao banco, ao log nem à resposta.
func TestIMG01_Upload_FilenameHostilNaoVaza(t *testing.T) {
	casos := []struct{ nome, disposicao, marcador string }{
		{"traversal", `form-data; name="imagem"; filename="../../etc/passwd"`, "etc/passwd"},
		{"traversal absoluto", `form-data; name="imagem"; filename="/etc/passwd.png"`, "passwd"},
		{"windows", `form-data; name="imagem"; filename="..\..\boot.ini"`, "boot.ini"},
		{"CRLF escapado", `form-data; name="imagem"; filename="a\r\nb.png"`, `a\r\nb`},
		{"CRLF por RFC 2231", `form-data; name="imagem"; filename*=UTF-8''a%0D%0Ab.png`, "b.png"},
		{"script", `form-data; name="imagem"; filename="<script>alert(1)</script>.png"`, "<script>"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := img01Setup(t)
			var gravado driver.Value
			img01EsperarTroca(e.mock, "", img01Fixo, &gravado)
			body, ct := img01Multipart(t, img01Parte{campo: "imagem", tipo: "image/png", dados: img01PNG(t, 4, 4), disposicaoCrua: c.disposicao})
			w := e.serve(img01Upload(t, "/api/produtos/1/imagem", body, ct))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			nome, _ := gravado.(string)
			assert.Regexp(t, reImg01NomeV4, nome)
			for _, arq := range e.arquivos(t) {
				assert.Regexp(t, reImg01NomeV4, arq)
				assert.NotContains(t, arq, c.marcador)
			}
			assert.NotContains(t, w.Body.String(), c.marcador)
			assert.NotContains(t, e.logs.String(), c.marcador)
			assert.NotContains(t, nome, c.marcador)
		})
	}
}

// CRLF CRU no cabeçalho da parte (injeção de header): o multipart fica
// malformado ou a linha vira header extra; em nenhum caso o nome vaza.
func TestIMG01_Upload_FilenameComCRLFCru(t *testing.T) {
	e := img01Setup(t)
	var gravado driver.Value
	img01EsperarTroca(e.mock, "", img01Fixo, &gravado)
	boundary := "limiteIMG01"
	corpo := "--" + boundary + "\r\n" +
		"Content-Disposition: form-data; name=\"imagem\"; filename=\"a\r\nb.png\"\r\n" +
		"Content-Type: image/png\r\n\r\n" + string(img01PNG(t, 4, 4)) + "\r\n--" + boundary + "--\r\n"
	w := e.serve(img01Upload(t, "/api/produtos/1/imagem", strings.NewReader(corpo), "multipart/form-data; boundary="+boundary))
	assert.Contains(t, []int{http.StatusOK, http.StatusBadRequest}, w.Code, w.Body.String())
	if w.Code == http.StatusBadRequest {
		assert.JSONEq(t, img01Msg400, w.Body.String())
	}
	for _, arq := range e.arquivos(t) {
		assert.Regexp(t, reImg01NomeV4, arq)
	}
	assert.NotContains(t, w.Body.String(), "b.png")
	assert.NotContains(t, e.logs.String(), "b.png")
}

// Troca: a imagem antiga sai do disco depois do commit.
func TestIMG01_Upload_TrocaApagaAntiga(t *testing.T) {
	e := img01Setup(t)
	require.NoError(t, os.WriteFile(filepath.Join(e.dir, img01Antiga), []byte("antiga"), 0o640))
	var gravado driver.Value
	img01EsperarTroca(e.mock, img01Antiga, img01Fixo, &gravado)

	w := e.serve(img01UploadImagem(t, img01PNG(t, 5, 5), "nova.png", "image/png"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []string{gravado.(string)}, e.arquivos(t))
	assert.NoFileExists(t, filepath.Join(e.dir, img01Antiga))
}

// Origem do próprio host e da allowlist CORS passam do anti-CSRF.
func TestIMG01_Upload_OrigemPermitida(t *testing.T) {
	for _, origem := range []string{"http://example.com", "https://example.com", img01OrigemCORS} {
		t.Run(origem, func(t *testing.T) {
			e := img01Setup(t)
			var gravado driver.Value
			img01EsperarTroca(e.mock, "", img01Fixo, &gravado)
			req := img01UploadImagem(t, img01PNG(t, 2, 2), "a.png", "image/png")
			req.Header.Set("Origin", origem) // httptest.NewRequest: Host = example.com
			w := e.serve(req)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		})
	}
}

// ---------------------------------------------------------------------------
// POST — recusas antes do corpo (auth, policy, id, anti-CSRF)
// ---------------------------------------------------------------------------

func TestIMG01_Upload_RecusasAntesDoCorpo(t *testing.T) {
	casos := []struct {
		nome   string
		path   string
		prep   func(t *testing.T, r *http.Request)
		status int
		corpo  string
	}{
		{"sem token → 401", "/api/produtos/1/imagem", func(t *testing.T, r *http.Request) { r.Header.Del("Authorization") }, http.StatusUnauthorized, ""},
		{"token inválido → 401", "/api/produtos/1/imagem", func(t *testing.T, r *http.Request) { r.Header.Set("Authorization", "Bearer x.y.z") }, http.StatusUnauthorized, ""},
		{"vendedor → 403", "/api/produtos/1/imagem", func(t *testing.T, r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 2, "normal"))
		}, http.StatusForbidden, ""},
		{"vendedor com id inválido → 403 (policy antes do id)", "/api/produtos/abc/imagem", func(t *testing.T, r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 2, "normal"))
			r.Header.Del("X-Requested-With")
		}, http.StatusForbidden, ""},
		{"id inválido → 400", "/api/produtos/abc/imagem", nil, http.StatusBadRequest, `{"success":false,"error":"id inválido"}`},
		{"id zero → 400", "/api/produtos/0/imagem", nil, http.StatusBadRequest, `{"success":false,"error":"id inválido"}`},
		{"id inválido sem X-Requested-With → 400 (id antes do anti-CSRF)", "/api/produtos/-1/imagem", func(t *testing.T, r *http.Request) { r.Header.Del("X-Requested-With") }, http.StatusBadRequest, `{"success":false,"error":"id inválido"}`},
		{"sem X-Requested-With → 403", "/api/produtos/1/imagem", func(t *testing.T, r *http.Request) { r.Header.Del("X-Requested-With") }, http.StatusForbidden, img01Msg403},
		{"X-Requested-With errado → 403", "/api/produtos/1/imagem", func(t *testing.T, r *http.Request) { r.Header.Set("X-Requested-With", "fetch") }, http.StatusForbidden, img01Msg403},
		{"X-Requested-With minúsculo → 403", "/api/produtos/1/imagem", func(t *testing.T, r *http.Request) { r.Header.Set("X-Requested-With", "xmlhttprequest") }, http.StatusForbidden, img01Msg403},
		{"Origin de outro site → 403", "/api/produtos/1/imagem", func(t *testing.T, r *http.Request) { r.Header.Set("Origin", "https://evil.test") }, http.StatusForbidden, img01Msg403},
		{"Origin null → 403", "/api/produtos/1/imagem", func(t *testing.T, r *http.Request) { r.Header.Set("Origin", "null") }, http.StatusForbidden, img01Msg403},
		{"Origin com porta diferente → 403", "/api/produtos/1/imagem", func(t *testing.T, r *http.Request) { r.Header.Set("Origin", "http://example.com:8081") }, http.StatusForbidden, img01Msg403},
		{"Origin prefixo da allowlist → 403", "/api/produtos/1/imagem", func(t *testing.T, r *http.Request) { r.Header.Set("Origin", img01OrigemCORS+".evil.test") }, http.StatusForbidden, img01Msg403},
		{"Content-Length 5 MB sem X-Requested-With → 403 (anti-CSRF antes do corpo)", "/api/produtos/1/imagem", func(t *testing.T, r *http.Request) {
			r.Header.Del("X-Requested-With")
			r.ContentLength = 5 << 20
		}, http.StatusForbidden, img01Msg403},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := img01Setup(t)
			body, ct := img01Multipart(t, img01Parte{campo: "imagem", filename: "a.png", tipo: "image/png", dados: img01PNG(t, 2, 2)})
			conta := &contaLeitura{r: body}
			req := img01Upload(t, c.path, conta, ct)
			req.ContentLength = int64(body.Len())
			if c.prep != nil {
				c.prep(t, req)
			}
			w := e.serve(req)
			require.Equal(t, c.status, w.Code, w.Body.String())
			if c.corpo != "" {
				assert.JSONEq(t, c.corpo, w.Body.String())
			}
			assert.Zero(t, conta.lido, "o corpo não pode ser lido")
			assert.Empty(t, e.arquivos(t))
			assert.NoError(t, e.mock.ExpectationsWereMet(), "nenhum SQL")
		})
	}
}

// ---------------------------------------------------------------------------
// POST — tamanho
// ---------------------------------------------------------------------------

func TestIMG01_Upload_TamanhoDoCorpo(t *testing.T) {
	grande := img01PNGExato(t, catalogo.MaxImagemBytes+1)
	casos := []struct {
		nome    string
		montar  func(t *testing.T) *http.Request
		semLer  bool
		aceita4 bool // 400 rápido: o multipart é recusado antes de estourar o teto
	}{
		{"parte com 1 MB + 1", func(t *testing.T) *http.Request {
			return img01UploadImagem(t, grande, "a.png", "image/png")
		}, false, false},
		{"Content-Length 5 MB → 413 sem ler", func(t *testing.T) *http.Request {
			body, ct := img01Multipart(t, img01Parte{campo: "imagem", dados: img01PNG(t, 2, 2)})
			r := img01Upload(t, "/api/produtos/1/imagem", body, ct)
			r.ContentLength = 5 << 20
			return r
		}, true, false},
		{"Content-Length 1 byte acima do teto do corpo", func(t *testing.T) *http.Request {
			body, ct := img01Multipart(t, img01Parte{campo: "imagem", dados: img01PNG(t, 2, 2)})
			r := img01Upload(t, "/api/produtos/1/imagem", body, ct)
			r.ContentLength = catalogohttp.MaxCorpoUploadBytes + 1
			return r
		}, true, false},
		{"chunked com parte de 3 MB", func(t *testing.T) *http.Request {
			body, ct := img01Multipart(t, img01Parte{campo: "imagem", dados: bytes.Repeat([]byte{0x89}, 3<<20)})
			r := img01Upload(t, "/api/produtos/1/imagem", semTamanho{body}, ct)
			r.ContentLength = -1
			return r
		}, false, false},
		{"chunked com cabeçalho de parte de 2 MB (estoura o MaxBytesReader)", func(t *testing.T) *http.Request {
			boundary := "limiteIMG01"
			corpo := "--" + boundary + "\r\n" +
				"Content-Disposition: form-data; name=\"imagem\"\r\n" +
				"X-Lixo: " + strings.Repeat("a", 2<<20) + "\r\n\r\n" +
				string(img01PNG(t, 2, 2)) + "\r\n--" + boundary + "--\r\n"
			r := img01Upload(t, "/api/produtos/1/imagem", semTamanho{strings.NewReader(corpo)}, "multipart/form-data; boundary="+boundary)
			r.ContentLength = -1
			return r
		}, false, false},
		{"chunked com preâmbulo de 2 MB sem quebra de linha → 400 rápido", func(t *testing.T) *http.Request {
			body, ct := img01Multipart(t, img01Parte{campo: "imagem", dados: img01PNG(t, 2, 2)})
			corpo := append(bytes.Repeat([]byte("lixo "), (2<<20)/5), body.Bytes()...)
			r := img01Upload(t, "/api/produtos/1/imagem", semTamanho{bytes.NewReader(corpo)}, ct)
			r.ContentLength = -1
			return r
		}, false, true},
		{"chunked: imagem válida + 2ª parte de 2 MB → 400 (segunda parte)", func(t *testing.T) *http.Request {
			body, ct := img01Multipart(t,
				img01Parte{campo: "imagem", dados: img01PNG(t, 2, 2)},
				img01Parte{campo: "outro", dados: bytes.Repeat([]byte{'z'}, 2<<20)})
			r := img01Upload(t, "/api/produtos/1/imagem", semTamanho{body}, ct)
			r.ContentLength = -1
			return r
		}, false, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := img01Setup(t)
			req := c.montar(t)
			conta := &contaLeitura{r: req.Body}
			req.Body = struct {
				*contaLeitura
				closer
			}{conta, closer{}}
			w := e.serve(req)
			if c.aceita4 {
				require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
				assert.JSONEq(t, img01Msg400, w.Body.String())
				assert.LessOrEqual(t, conta.lido, 64<<10, "recusa sem ler o resto do corpo")
			} else {
				require.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
				assert.JSONEq(t, img01Msg413, w.Body.String())
			}
			if c.semLer {
				assert.Zero(t, conta.lido, "Content-Length acima do teto: 413 sem ler o corpo")
			} else {
				assert.LessOrEqual(t, conta.lido, catalogohttp.MaxCorpoUploadBytes+64<<10, "o corpo é cortado no teto")
			}
			assert.Empty(t, e.arquivos(t))
			assert.NoError(t, e.mock.ExpectationsWereMet())
		})
	}
}

type closer struct{}

func (closer) Close() error { return nil }

// ---------------------------------------------------------------------------
// POST — conteúdo e multipart inválidos → 400 com a mensagem fixa
// ---------------------------------------------------------------------------

func TestIMG01_Upload_Conteudo400(t *testing.T) {
	pngOK := img01PNG(t, 20, 20)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><rect width="1" height="1"/></svg>`)
	casos := []struct {
		nome, filename, tipo string
		dados                []byte
	}{
		{"SVG como .png / image/png", "x.png", "image/png", svg},
		{"SVG como .svg", "x.svg", "image/svg+xml", svg},
		{"HTML como .jpg", "foto.jpg", "image/jpeg", []byte("<!DOCTYPE html><html><script>alert(1)</script></html>")},
		{"texto como .jpg", "foto.jpg", "image/jpeg", []byte("texto puro")},
		{"GIF como .jpg", "foto.jpg", "image/jpeg", img01GIF(t)},
		{"GIF como .gif", "a.gif", "image/gif", img01GIF(t)},
		{"PDF como .jpg", "foto.jpg", "image/jpeg", []byte("%PDF-1.7\n%%EOF")},
		{"ZIP como .jpg", "foto.jpg", "image/jpeg", []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00abc")},
		{"PNG truncado", "a.png", "image/png", pngOK[:len(pngOK)/2]},
		{"bomb 50000x50000", "bomb.png", "image/png", img01CabecalhoPNG(50000, 50000)},
		{"largura 0", "a.png", "image/png", img01CabecalhoPNG(0, 10)},
		{"4097x10", "a.png", "image/png", img01PNGGray(t, 4097, 10)},
		{"parte vazia", "a.png", "image/png", nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := img01Setup(t)
			inicio := time.Now()
			w := e.serve(img01UploadImagem(t, c.dados, c.filename, c.tipo))
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.JSONEq(t, img01Msg400, w.Body.String())
			assert.Less(t, time.Since(inicio), 2*time.Second)
			assert.Empty(t, e.arquivos(t))
			assert.NoError(t, e.mock.ExpectationsWereMet(), "nenhum SQL")
		})
	}
}

func TestIMG01_Upload_Multipart400(t *testing.T) {
	pngOK := img01PNG(t, 2, 2)
	casos := []struct {
		nome   string
		montar func(t *testing.T) (*bytes.Buffer, string)
	}{
		{"JSON", func(t *testing.T) (*bytes.Buffer, string) {
			return bytes.NewBufferString(`{"imagem":"iVBORw0KGgo="}`), "application/json"
		}},
		{"sem Content-Type", func(t *testing.T) (*bytes.Buffer, string) { return bytes.NewBuffer(pngOK), "" }},
		{"image/png cru (não multipart)", func(t *testing.T) (*bytes.Buffer, string) { return bytes.NewBuffer(pngOK), "image/png" }},
		{"x-www-form-urlencoded", func(t *testing.T) (*bytes.Buffer, string) {
			return bytes.NewBufferString("imagem=abc"), "application/x-www-form-urlencoded"
		}},
		{"multipart sem boundary", func(t *testing.T) (*bytes.Buffer, string) {
			b, _ := img01Multipart(t, img01Parte{campo: "imagem", dados: pngOK})
			return b, "multipart/form-data"
		}},
		{"multipart/mixed", func(t *testing.T) (*bytes.Buffer, string) {
			b, ct := img01Multipart(t, img01Parte{campo: "imagem", dados: pngOK})
			return b, strings.Replace(ct, "form-data", "mixed", 1)
		}},
		{"Content-Type malformado", func(t *testing.T) (*bytes.Buffer, string) {
			return bytes.NewBuffer(pngOK), "multipart/form-data; boundary="
		}},
		{"boundary que não aparece no corpo", func(t *testing.T) (*bytes.Buffer, string) {
			b, _ := img01Multipart(t, img01Parte{campo: "imagem", dados: pngOK})
			return b, "multipart/form-data; boundary=outro"
		}},
		{"multipart sem nenhuma parte", func(t *testing.T) (*bytes.Buffer, string) { return img01Multipart(t) }},
		{"corpo vazio", func(t *testing.T) (*bytes.Buffer, string) {
			_, ct := img01Multipart(t)
			return &bytes.Buffer{}, ct
		}},
		{"campo errado", func(t *testing.T) (*bytes.Buffer, string) {
			return img01Multipart(t, img01Parte{campo: "arquivo", filename: "a.png", dados: pngOK})
		}},
		{"campo IMAGEM maiúsculo", func(t *testing.T) (*bytes.Buffer, string) {
			return img01Multipart(t, img01Parte{campo: "IMAGEM", filename: "a.png", dados: pngOK})
		}},
		{"duas partes imagem", func(t *testing.T) (*bytes.Buffer, string) {
			return img01Multipart(t, img01Parte{campo: "imagem", dados: pngOK}, img01Parte{campo: "imagem", dados: pngOK})
		}},
		{"imagem + outro campo", func(t *testing.T) (*bytes.Buffer, string) {
			return img01Multipart(t, img01Parte{campo: "imagem", dados: pngOK}, img01Parte{campo: "descricao", dados: []byte("x")})
		}},
		{"outro campo + imagem", func(t *testing.T) (*bytes.Buffer, string) {
			return img01Multipart(t, img01Parte{campo: "descricao", dados: []byte("x")}, img01Parte{campo: "imagem", dados: pngOK})
		}},
		{"multipart truncado (sem boundary final)", func(t *testing.T) (*bytes.Buffer, string) {
			b, ct := img01Multipart(t, img01Parte{campo: "imagem", dados: pngOK})
			return bytes.NewBuffer(b.Bytes()[:b.Len()-10]), ct
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := img01Setup(t)
			body, ct := c.montar(t)
			w := e.serve(img01Upload(t, "/api/produtos/1/imagem", body, ct))
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.JSONEq(t, img01Msg400, w.Body.String())
			assert.Empty(t, e.arquivos(t))
			assert.NoError(t, e.mock.ExpectationsWereMet(), "nenhum SQL")
		})
	}
}

// ---------------------------------------------------------------------------
// POST — falhas depois de gravar: nenhum arquivo pode sobrar
// ---------------------------------------------------------------------------

func TestIMG01_Upload_FalhaNaTroca_NaoSobraArquivo(t *testing.T) {
	casos := []struct {
		nome   string
		mock   func(m sqlmock.Sqlmock)
		status int
		corpo  string
	}{
		{"produto inexistente → 404", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(reImg01Lock).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"imagem"}))
			m.ExpectRollback()
		}, http.StatusNotFound, `{"success":false,"error":"produto não encontrado"}`},
		{"falha no SELECT FOR UPDATE → 500", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(reImg01Lock).WillReturnError(errors.New("lock wait timeout"))
			m.ExpectRollback()
		}, http.StatusInternalServerError, ""},
		{"falha no UPDATE → 500", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(reImg01Lock).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(img01Antiga))
			m.ExpectExec(reImg01Update).WillReturnError(errors.New("disk full"))
			m.ExpectRollback()
		}, http.StatusInternalServerError, ""},
		{"falha no commit → 500", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(reImg01Lock).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(nil))
			m.ExpectExec(reImg01Update).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit().WillReturnError(errors.New("deadlock"))
		}, http.StatusInternalServerError, ""},
		{"falha no begin → 500", func(m sqlmock.Sqlmock) {
			m.ExpectBegin().WillReturnError(errors.New("pool"))
		}, http.StatusInternalServerError, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := img01Setup(t)
			require.NoError(t, os.WriteFile(filepath.Join(e.dir, img01Antiga), []byte("antiga"), 0o640))
			c.mock(e.mock)
			w := e.serve(img01UploadImagem(t, img01PNG(t, 3, 3), "a.png", "image/png"))
			require.Equal(t, c.status, w.Code, w.Body.String())
			if c.corpo != "" {
				assert.JSONEq(t, c.corpo, w.Body.String())
			} else {
				assert.JSONEq(t, `{"success":false,"error":"erro interno"}`, w.Body.String())
			}
			assert.Equal(t, []string{img01Antiga}, e.arquivos(t), "o novo é apagado; a antiga fica")
			assert.NoError(t, e.mock.ExpectationsWereMet())
		})
	}
}

// Releitura falha depois do commit: 500, mas o arquivo vigente fica (é o do banco).
func TestIMG01_Upload_ReleituraFalha(t *testing.T) {
	e := img01Setup(t)
	var gravado driver.Value
	e.mock.ExpectBegin()
	e.mock.ExpectQuery(reImg01Lock).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(nil))
	e.mock.ExpectExec(reImg01Update).WithArgs(capturaArg{&gravado}, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	e.mock.ExpectCommit()
	e.mock.ExpectQuery(reImg01Reler).WillReturnError(errors.New("conexão caiu"))
	w := e.serve(img01UploadImagem(t, img01PNG(t, 3, 3), "a.png", "image/png"))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, []string{gravado.(string)}, e.arquivos(t))
}

// Montado sem armazenamento (Config sem PRODUTO_IMAGENS_DIR): upload/remoção
// 500 e leitura 404, sem SQL.
func TestIMG01_ImagensDesligadas(t *testing.T) {
	e := img01SetupDir(t, "")
	w := e.serve(img01UploadImagem(t, img01PNG(t, 2, 2), "a.png", "image/png"))
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())

	req := httptest.NewRequest(http.MethodDelete, "/api/produtos/1/imagem", nil)
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
	assert.Equal(t, http.StatusInternalServerError, e.serve(req).Code)

	w = e.serve(httptest.NewRequest(http.MethodGet, img01PrefixoURL+img01Antiga, nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NoError(t, e.mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// DELETE /api/produtos/{id}/imagem
// ---------------------------------------------------------------------------

func img01Delete(t *testing.T, path string, uid int64, role string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	if uid > 0 {
		req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), uid, role))
	}
	return req
}

func TestIMG01_Remover(t *testing.T) {
	casos := []struct {
		nome    string
		path    string
		uid     int64
		role    string
		mock    func(m sqlmock.Sqlmock)
		status  int
		sobra   []string
		urlNula bool
	}{
		{"com imagem: zera (NULL) e apaga", "/api/produtos/1/imagem", 1, "admin", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(reImg01Lock).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(img01Antiga))
			m.ExpectExec(reImg01Update).WithArgs(nil, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
			m.ExpectQuery(reImg01Reler).WillReturnRows(img01LinhaProduto(""))
		}, http.StatusOK, []string{}, true},
		{"sem imagem: idempotente, sem UPDATE", "/api/produtos/1/imagem", 1, "admin", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(reImg01Lock).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(nil))
			m.ExpectCommit()
			m.ExpectQuery(reImg01Reler).WillReturnRows(img01LinhaProduto(""))
		}, http.StatusOK, []string{img01Antiga}, true},
		{"nome adulterado no banco: zera sem tocar o disco", "/api/produtos/1/imagem", 1, "admin", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(reImg01Lock).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow("../" + img01Antiga))
			m.ExpectExec(reImg01Update).WithArgs(nil, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
			m.ExpectQuery(reImg01Reler).WillReturnRows(img01LinhaProduto(""))
		}, http.StatusOK, []string{img01Antiga}, true},
		{"produto inexistente → 404", "/api/produtos/1/imagem", 1, "admin", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(reImg01Lock).WillReturnRows(sqlmock.NewRows([]string{"imagem"}))
			m.ExpectRollback()
		}, http.StatusNotFound, []string{img01Antiga}, false},
		{"falha no UPDATE → 500 e o arquivo fica", "/api/produtos/1/imagem", 1, "admin", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectQuery(reImg01Lock).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(img01Antiga))
			m.ExpectExec(reImg01Update).WillReturnError(errors.New("x"))
			m.ExpectRollback()
		}, http.StatusInternalServerError, []string{img01Antiga}, false},
		{"sem token → 401", "/api/produtos/1/imagem", 0, "", func(sqlmock.Sqlmock) {}, http.StatusUnauthorized, []string{img01Antiga}, false},
		{"vendedor → 403", "/api/produtos/1/imagem", 2, "normal", func(sqlmock.Sqlmock) {}, http.StatusForbidden, []string{img01Antiga}, false},
		{"id inválido → 400", "/api/produtos/x/imagem", 1, "admin", func(sqlmock.Sqlmock) {}, http.StatusBadRequest, []string{img01Antiga}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := img01Setup(t)
			require.NoError(t, os.WriteFile(filepath.Join(e.dir, img01Antiga), []byte("antiga"), 0o640))
			c.mock(e.mock)
			w := e.serve(img01Delete(t, c.path, c.uid, c.role))
			require.Equal(t, c.status, w.Code, w.Body.String())
			if c.urlNula {
				assert.Nil(t, img01ImagemURL(t, w.Body.Bytes()))
			}
			assert.ElementsMatch(t, c.sobra, e.arquivos(t))
			assert.NoError(t, e.mock.ExpectationsWereMet())
		})
	}
}

// Segunda remoção seguida: idempotente (200 nas duas).
func TestIMG01_Remover_DuasVezes(t *testing.T) {
	e := img01Setup(t)
	require.NoError(t, os.WriteFile(filepath.Join(e.dir, img01Antiga), []byte("antiga"), 0o640))
	e.mock.ExpectBegin()
	e.mock.ExpectQuery(reImg01Lock).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(img01Antiga))
	e.mock.ExpectExec(reImg01Update).WithArgs(nil, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	e.mock.ExpectCommit()
	e.mock.ExpectQuery(reImg01Reler).WillReturnRows(img01LinhaProduto(""))
	e.mock.ExpectBegin()
	e.mock.ExpectQuery(reImg01Lock).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(nil))
	e.mock.ExpectCommit()
	e.mock.ExpectQuery(reImg01Reler).WillReturnRows(img01LinhaProduto(""))
	for range 2 {
		w := e.serve(img01Delete(t, "/api/produtos/1/imagem", 1, "admin"))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	assert.Empty(t, e.arquivos(t))
	assert.NoError(t, e.mock.ExpectationsWereMet())
}

// imagem_url no GET/List: só para nome válido; adulterado vira null.
func TestIMG01_DTO_ImagemURL(t *testing.T) {
	casos := []struct {
		nome, imagem string
		want         any
	}{
		{"válida", img01Antiga, img01PrefixoURL + img01Antiga},
		{"sem imagem", "", nil},
		{"adulterada no banco", "../../etc/passwd", nil},
		{"maiúscula", strings.ToUpper(img01Antiga), nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := img01Setup(t)
			e.mock.ExpectQuery(reImg01Reler).WillReturnRows(img01LinhaProduto(c.imagem))
			req := httptest.NewRequest(http.MethodGet, "/api/produtos/1", nil)
			req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
			w := e.serve(req)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, c.want, img01ImagemURL(t, w.Body.Bytes()))
			assert.NotContains(t, w.Body.String(), e.dir, "o diretório nunca é exposto")
		})
	}
}

// ---------------------------------------------------------------------------
// GET /api/public/produtos/imagens/{arquivo}
// ---------------------------------------------------------------------------

func TestIMG01_Publica_Serve(t *testing.T) {
	casos := []struct{ nome, tipo string }{
		{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa.png", "image/png"},
		{"bbbbbbbb-bbbb-4bbb-9bbb-bbbbbbbbbbbb.jpg", "image/jpeg"},
		{"cccccccc-cccc-4ccc-accc-cccccccccccc.webp", "image/webp"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := img01Setup(t)
			conteudo := []byte("<html>conteúdo arbitrário do arquivo " + c.nome)
			require.NoError(t, os.WriteFile(filepath.Join(e.dir, c.nome), conteudo, 0o640))
			for _, comToken := range []bool{false, true} {
				req := httptest.NewRequest(http.MethodGet, img01PrefixoURL+c.nome, nil)
				if comToken {
					req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
				}
				w := e.serve(req)
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				h := w.Header()
				assert.Equal(t, c.tipo, h.Get("Content-Type"), "mapa fixo, sem sniff do conteúdo")
				assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
				assert.Contains(t, h.Get("Content-Security-Policy"), "default-src 'none'")
				assert.Equal(t, "public, max-age=31536000, immutable", h.Get("Cache-Control"))
				assert.Equal(t, "same-site", h.Get("Cross-Origin-Resource-Policy"))
				assert.Equal(t, "inline", h.Get("Content-Disposition"))
				assert.Empty(t, h.Values("Set-Cookie"))
				assert.NotEmpty(t, h.Get("Last-Modified"))
				assert.Equal(t, conteudo, w.Body.Bytes())
			}
			assert.NoError(t, e.mock.ExpectationsWereMet(), "rota pública não toca o banco")
		})
	}
}

func TestIMG01_Publica_HeadRangeCondicional(t *testing.T) {
	e := img01Setup(t)
	conteudo := []byte("0123456789abcdef")
	require.NoError(t, os.WriteFile(filepath.Join(e.dir, img01Antiga), conteudo, 0o640))
	url := img01PrefixoURL + img01Antiga

	w := e.serve(httptest.NewRequest(http.MethodHead, url, nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "16", w.Header().Get("Content-Length"))
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"))

	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Range", "bytes=2-5")
	w = e.serve(req)
	require.Equal(t, http.StatusPartialContent, w.Code)
	assert.Equal(t, "2345", w.Body.String())
	assert.Equal(t, "bytes 2-5/16", w.Header().Get("Content-Range"))

	req = httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Range", "bytes=100-200")
	assert.Equal(t, http.StatusRequestedRangeNotSatisfiable, e.serve(req).Code)

	req = httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("If-Modified-Since", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
	assert.Equal(t, http.StatusNotModified, e.serve(req).Code)
}

func TestIMG01_Publica_404(t *testing.T) {
	e := img01Setup(t)
	pai := filepath.Dir(e.dir)
	require.NoError(t, os.WriteFile(filepath.Join(pai, "segredo.png"), []byte("segredo"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pai, img01Antiga), []byte("segredo"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(e.dir, ".tmp-"+strings.TrimSuffix(img01Antiga, ".png")), []byte("tmp"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(e.dir, ".tmp-"+img01Antiga), []byte("tmp"), 0o600))
	existente := "dddddddd-dddd-4ddd-8ddd-dddddddddddd.png"
	require.NoError(t, os.WriteFile(filepath.Join(e.dir, existente), []byte("img"), 0o640))
	require.NoError(t, os.WriteFile(filepath.Join(e.dir, strings.ToUpper(strings.TrimSuffix(existente, ".png"))+".png"), []byte("img"), 0o640))
	linkFora := "11111111-1111-4111-8111-111111111111.png"
	linkDentro := "22222222-2222-4222-9222-222222222222.png"
	require.NoError(t, os.Symlink("../segredo.png", filepath.Join(e.dir, linkFora)))
	require.NoError(t, os.Symlink(existente, filepath.Join(e.dir, linkDentro)))
	diretorio := "33333333-3333-4333-a333-333333333333.jpg"
	require.NoError(t, os.Mkdir(filepath.Join(e.dir, diretorio), 0o750))

	paths := []string{
		img01PrefixoURL + img01Antiga, // não existe no dir (só no pai)
		img01PrefixoURL + "%2e%2e%2f" + img01Antiga,
		img01PrefixoURL + "%2e%2e%2fsegredo.png",
		img01PrefixoURL + "..%2f" + img01Antiga,
		img01PrefixoURL + "..%2fsegredo.png",
		img01PrefixoURL + "%2F" + existente,
		img01PrefixoURL + "%2Fetc%2Fpasswd",
		img01PrefixoURL + "..%5c" + img01Antiga,
		img01PrefixoURL + strings.ToUpper(strings.TrimSuffix(existente, ".png")) + ".png",
		img01PrefixoURL + strings.TrimSuffix(existente, ".png") + ".PNG",
		img01PrefixoURL + strings.TrimSuffix(existente, ".png") + ".svg",
		img01PrefixoURL + strings.TrimSuffix(existente, ".png") + ".html",
		img01PrefixoURL + strings.TrimSuffix(existente, ".png") + ".jpeg",
		img01PrefixoURL + strings.TrimSuffix(existente, ".png") + ".gif",
		img01PrefixoURL + ".tmp-" + strings.TrimSuffix(img01Antiga, ".png"),
		img01PrefixoURL + ".tmp-" + img01Antiga,
		img01PrefixoURL + "dddddddd-dddd-1ddd-8ddd-dddddddddddd.png", // UUID v1
		img01PrefixoURL + existente + "%00.png",
		img01PrefixoURL + existente + "%0a",
		img01PrefixoURL + linkFora,
		img01PrefixoURL + linkDentro,
		img01PrefixoURL + diretorio,
		img01PrefixoURL + "segredo.png",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			w := e.serve(httptest.NewRequest(http.MethodGet, p, nil))
			require.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
			assert.NotContains(t, w.Body.String(), "segredo")
			assert.NotContains(t, w.Body.String(), "img")
		})
	}
}

// Path com ".." ou "." literal é limpo pelo ServeMux (redirect 307 para o
// caminho limpo; com "..", fora da rota de imagens) e diretório vazio não
// casa com a rota: em nenhum caso o arquivo de fora sai.
func TestIMG01_Publica_PathLimpoOuVazio(t *testing.T) {
	e := img01Setup(t)
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(e.dir), "segredo.png"), []byte("CONTEUDO-SECRETO"), 0o600))
	for _, p := range []string{
		img01PrefixoURL,
		img01PrefixoURL + "../segredo.png",
		img01PrefixoURL + "../../../../etc/passwd",
		img01PrefixoURL + "./" + img01Antiga,
		strings.TrimSuffix(img01PrefixoURL, "/"),
	} {
		t.Run(p, func(t *testing.T) {
			w := e.serve(httptest.NewRequest(http.MethodGet, p, nil))
			assert.Contains(t, []int{http.StatusNotFound, http.StatusMovedPermanently, http.StatusTemporaryRedirect}, w.Code, "body: %s", w.Body.String())
			assert.NotContains(t, w.Body.String(), "CONTEUDO-SECRETO")
			if loc := w.Header().Get("Location"); loc != "" {
				w2 := e.serve(httptest.NewRequest(http.MethodGet, loc, nil))
				assert.NotEqual(t, http.StatusOK, w2.Code, "o destino do redirect não pode servir arquivo: %s", loc)
				assert.NotContains(t, w2.Body.String(), "CONTEUDO-SECRETO")
			}
		})
	}
}

func TestIMG01_Publica_405(t *testing.T) {
	e := img01Setup(t)
	require.NoError(t, os.WriteFile(filepath.Join(e.dir, img01Antiga), []byte("x"), 0o640))
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(m, func(t *testing.T) {
			req := httptest.NewRequest(m, img01PrefixoURL+img01Antiga, strings.NewReader("x"))
			req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
			w := e.serve(req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code, w.Body.String())
			assert.FileExists(t, filepath.Join(e.dir, img01Antiga))
		})
	}
}

func TestIMG01_NovoImagemPublicaHandler_ExigeCasos(t *testing.T) {
	assert.Panics(t, func() { catalogohttp.NovoImagemPublicaHandler(nil) })
}

// O bootstrap liga o mesmo diretório para o upload e para a leitura: o que
// é enviado sai pela URL devolvida.
func TestIMG01_UploadDepoisLeituraPublica(t *testing.T) {
	e := img01Setup(t)
	var gravado driver.Value
	img01EsperarTroca(e.mock, "", img01Fixo, &gravado)
	png := img01PNG(t, 6, 6)
	w := e.serve(img01UploadImagem(t, png, "a.png", "image/png"))
	require.Equal(t, http.StatusOK, w.Code)
	w = e.serve(httptest.NewRequest(http.MethodGet, img01PrefixoURL+gravado.(string), nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
	assert.Equal(t, "image/png", http.DetectContentType(w.Body.Bytes()))
}

// Handler chamado fora do router (sem Principal): falha fechado com 500,
// sem ler o corpo nem tocar o banco/disco.
func TestIMG01_HandlerSemPrincipal_500(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	dir := t.TempDir()
	arm, err := bootstrap.AbrirArmazenamentoImagens(dir)
	require.NoError(t, err)
	defer arm.Close()
	cat := bootstrap.NovoCatalogoHTTPCom(bootstrap.NovoCatalogo(db, arm), nil)
	casos := map[string]http.HandlerFunc{"EnviarImagem": cat.Produto.EnviarImagem, "RemoverImagem": cat.Produto.RemoverImagem}
	for nome, h := range casos {
		t.Run(nome, func(t *testing.T) {
			body, ct := img01Multipart(t, img01Parte{campo: "imagem", dados: img01PNG(t, 2, 2)})
			req := httptest.NewRequest(http.MethodPost, "/api/produtos/1/imagem", body)
			req.Header.Set("Content-Type", ct)
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			req.SetPathValue("id", "1")
			w := httptest.NewRecorder()
			h(w, req)
			assert.Equal(t, http.StatusInternalServerError, w.Code)
			ents, _ := os.ReadDir(dir)
			assert.Empty(t, ents)
		})
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

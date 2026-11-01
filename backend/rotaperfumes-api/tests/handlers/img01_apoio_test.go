package handlers_test

// IMG-01 (Lote 26, TestBrain): apoio dos testes HTTP da imagem do produto.
// Router real (bootstrap.HandlerHTTP) sobre sqlmock e o adapter de
// filesystem real num t.TempDir() (nada toca backend/rotaperfumes-api/data/).
// As imagens são geradas em memória.

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/vlog"
)

const (
	img01OrigemCORS = "https://app.rotaperfumes.test"
	img01Fixo       = "0d4e5f6a-7b8c-4d9e-8f0a-1b2c3d4e5f60.png" // nome devolvido pela releitura
	img01Antiga     = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa.png"
	reImg01Lock     = `SELECT imagem FROM produtos WHERE id = \? FOR UPDATE`
	reImg01Update   = `UPDATE produtos SET imagem = \? WHERE id = \?`
	reImg01Reler    = `FROM produtos WHERE id = \?`
	img01PrefixoURL = "/api/public/produtos/imagens/"
)

type img01Env struct {
	h    http.Handler
	mock sqlmock.Sqlmock
	dir  string
	logs *syncBuffer
}

// syncBuffer: log.SetOutput pode ser chamado de goroutines do server.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// img01Setup monta o router com as imagens ligadas (dir em t.TempDir()) e
// captura o log (com vlog ligado) para conferir o que é logado.
func img01Setup(t *testing.T) *img01Env {
	t.Helper()
	return img01SetupDir(t, filepath.Join(t.TempDir(), "produtos-imagens"))
}

func img01SetupDir(t *testing.T, dir string) *img01Env {
	t.Helper()
	cfg := testCfg()
	cfg.ProdutoImagensDir = dir
	cfg.CORSAllowedOrigins = []string{img01OrigemCORS}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	logs := &syncBuffer{}
	anterior := log.Writer()
	log.SetOutput(logs)
	vlogAntes := vlog.Enabled()
	vlog.SetEnabled(true)
	t.Cleanup(func() { log.SetOutput(anterior); vlog.SetEnabled(vlogAntes) })
	hs := bootstrap.NovosHandlersCom(db, cfg, novaIdentidadeTeste(db, cfg).casos)
	return &img01Env{h: bootstrap.HandlerHTTP(cfg, fakeUserStatusChecker{}, hs), mock: mock, dir: dir, logs: logs}
}

func (e *img01Env) arquivos(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(e.dir)
	require.NoError(t, err)
	out := make([]string, 0, len(ents))
	for _, en := range ents {
		out = append(out, en.Name())
	}
	return out
}

func (e *img01Env) serve(req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	return w
}

// img01Parte é uma parte do multipart; cabecalho cru substitui o
// Content-Disposition gerado (para filenames hostis).
type img01Parte struct {
	campo, filename, tipo string
	dados                 []byte
	disposicaoCrua        string
}

func img01Multipart(t *testing.T, partes ...img01Parte) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for _, p := range partes {
		h := textproto.MIMEHeader{}
		switch {
		case p.disposicaoCrua != "":
			h.Set("Content-Disposition", p.disposicaoCrua)
		case p.filename != "":
			h.Set("Content-Disposition", `form-data; name="`+p.campo+`"; filename="`+p.filename+`"`)
		default:
			h.Set("Content-Disposition", `form-data; name="`+p.campo+`"`)
		}
		if p.tipo != "" {
			h.Set("Content-Type", p.tipo)
		}
		w, err := mw.CreatePart(h)
		require.NoError(t, err)
		_, err = w.Write(p.dados)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())
	return &b, mw.FormDataContentType()
}

// img01Upload monta o POST com token de admin e X-Requested-With.
func img01Upload(t *testing.T, path string, body io.Reader, ct string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	return req
}

func img01UploadImagem(t *testing.T, dados []byte, filename, tipo string) *http.Request {
	t.Helper()
	body, ct := img01Multipart(t, img01Parte{campo: "imagem", filename: filename, tipo: tipo, dados: dados})
	return img01Upload(t, "/api/produtos/1/imagem", body, ct)
}

// capturaArg guarda o valor que o UPDATE recebeu (o nome gerado).
type capturaArg struct{ v *driver.Value }

func (c capturaArg) Match(v driver.Value) bool { *c.v = v; return true }

func img01LinhaProduto(imagem string) *sqlmock.Rows {
	return sqlmock.NewRows(produtoColunasHeaderForHandler()).AddRow(int64(1), "SKU-001", "Perfume Teste", "Perfumaria",
		"Marca X", "Cítrico", 99.9, 45.0, "UN", nil, true, imagem, arq02Criado, arq02Atualizado)
}

// img01EsperarTroca: tx da troca com sucesso (imagem anterior = antiga, ""
// = NULL) e a releitura devolvendo releitura.
func img01EsperarTroca(m sqlmock.Sqlmock, antiga, releitura string, gravado *driver.Value) {
	var ant any
	if antiga != "" {
		ant = antiga
	}
	m.ExpectBegin()
	m.ExpectQuery(reImg01Lock).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"imagem"}).AddRow(ant))
	m.ExpectExec(reImg01Update).WithArgs(capturaArg{gravado}, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	m.ExpectQuery(reImg01Reler).WithArgs(int64(1)).WillReturnRows(img01LinhaProduto(releitura))
}

// ---- imagens em memória ----

func img01PNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 9), uint8(y * 5), 77, 255})
		}
	}
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, img))
	return b.Bytes()
}

func img01PNGGray(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))))
	return b.Bytes()
}

func img01JPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)), nil))
	return b.Bytes()
}

func img01GIF(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, gif.Encode(&b, image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White}), nil))
	return b.Bytes()
}

func img01Chunk(tipo string, dados []byte) []byte {
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

// img01CabecalhoPNG: assinatura + IHDR (w x h) + IEND, sem pixels (bomb).
func img01CabecalhoPNG(w, h uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6
	out := append([]byte("\x89PNG\r\n\x1a\n"), img01Chunk("IHDR", ihdr)...)
	return append(out, img01Chunk("IEND", nil)...)
}

// img01PNGExato: PNG válido com exatamente n bytes (chunk ancilar de enchimento).
func img01PNGExato(t *testing.T, n int) []byte {
	t.Helper()
	base := img01PNG(t, 8, 8)
	pos := len(base) - 12
	falta := n - len(base) - 12
	require.GreaterOrEqual(t, falta, 0)
	out := append([]byte{}, base[:pos]...)
	out = append(out, img01Chunk("teSt", bytes.Repeat([]byte{'x'}, falta))...)
	out = append(out, base[pos:]...)
	require.Len(t, out, n)
	return out
}

// img01WebP: WebP lossless (VP8L) 2x2 de cor única montado à mão.
func img01WebP() []byte {
	var buf []byte
	var acc uint64
	var n uint
	put := func(v uint64, bits uint) {
		acc |= v << n
		n += bits
		for n >= 8 {
			buf = append(buf, byte(acc))
			acc >>= 8
			n -= 8
		}
	}
	put(0x2f, 8)
	put(1, 14)
	put(1, 14)
	put(0, 1+3+1+1+1)
	for _, s := range []uint64{0x10, 0x20, 0x30, 0xFF, 0} {
		put(1, 1)
		put(0, 1)
		put(1, 1)
		put(s, 8)
	}
	if n > 0 {
		buf = append(buf, byte(acc))
	}
	var c bytes.Buffer
	c.WriteString("VP8L")
	_ = binary.Write(&c, binary.LittleEndian, uint32(len(buf)))
	c.Write(buf)
	if len(buf)%2 == 1 {
		c.WriteByte(0)
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(4+c.Len()))
	out.WriteString("WEBP")
	out.Write(c.Bytes())
	return out.Bytes()
}

// contaLeitura conta quantos bytes do corpo foram lidos.
type contaLeitura struct {
	r    io.Reader
	lido int
}

func (c *contaLeitura) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.lido += n
	return n, err
}

// semTamanho esconde o tamanho do corpo (httptest.NewRequest deduz
// Content-Length de *bytes.Buffer): simula o chunked.
type semTamanho struct{ io.Reader }

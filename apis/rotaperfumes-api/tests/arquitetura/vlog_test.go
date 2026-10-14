package arquitetura_test

// O7 (§6.3): chamadas a vlog.Printf e vlog.Trace sem conteúdo sensível.
//  1. o formato é literal e não contém %v, %+v nem %#v;
//  2. nenhum argumento é (ou contém) identificador/campo que case com
//     senha|password|token|secret|cnpj|email|hash|captcha|chave|key, a menos
//     que esteja dentro de vlog.MaskEmail(...), .Mascarado() ou len(...);
//     ids inteiros de token (tokenID/token_id) ficam numa allowlist por nome;
//  3. nenhum argumento é r.URL, r.Header, r.Body, cmd, req ou filtro inteiros.

import (
	"go/ast"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	reSensivel = regexp.MustCompile(`(?i)senha|password|token|secret|cnpj|email|hash|captcha|chave|key`)
	// Ids numéricos de linha (nunca o token em si). Sem go/types, a allowlist
	// é por nome: o tipo inteiro é garantido na revisão.
	idsPermitidos = map[string]bool{"tokenID": true, "TokenID": true, "tokenId": true, "token_id": true, "refreshTokenID": true}
	// Argumentos que nunca podem ir inteiros ao log.
	argsProibidos = map[string]bool{"cmd": true, "req": true, "filtro": true, "r.URL": true, "r.Header": true, "r.Body": true}
)

func violacoesVlog(arqs []arquivo, aplica func(pkg string) bool) []string {
	var out []string
	for _, a := range arqs {
		if a.pkg == pkgVlog || !aplica(a.pkg) {
			continue
		}
		chamadasA(a, pkgVlog, func(nome string, call *ast.CallExpr) {
			if (nome != "Printf" && nome != "Trace") || len(call.Args) < 3 {
				return
			}
			onde := a.pos(call.Pos()) + ": vlog." + nome + " "
			if motivo := formatoProibido(call.Args[2]); motivo != "" {
				out = append(out, onde+motivo)
			}
			alias := nomesDoImport(a.f, pkgVlog)
			for _, arg := range call.Args[3:] {
				if motivo := argumentoProibido(arg, alias); motivo != "" {
					out = append(out, onde+motivo)
				}
			}
		})
	}
	return out
}

func formatoProibido(e ast.Expr) string {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "com formato não literal"
	}
	f, _ := strconv.Unquote(lit.Value)
	f = strings.ReplaceAll(f, "%%", "")
	for _, verbo := range []string{"%v", "%+v", "%#v"} {
		if strings.Contains(f, verbo) {
			return "com " + verbo + " no formato"
		}
	}
	return ""
}

func argumentoProibido(arg ast.Expr, aliasVlog string) string {
	if nome := textoDe(arg); argsProibidos[nome] {
		return "com argumento proibido " + nome
	}
	var motivo string
	ast.Inspect(arg, func(n ast.Node) bool {
		if motivo != "" {
			return false
		}
		switch x := n.(type) {
		case *ast.CallExpr:
			if chamadaMascarada(x, aliasVlog) {
				return false // conteúdo protegido pela máscara / só o tamanho
			}
		case *ast.SelectorExpr:
			if sensivel(x.Sel.Name) {
				motivo = "com campo sensível " + textoDe(x)
			}
			_, xEhIdent := x.X.(*ast.Ident)
			return !xEhIdent // `email.Dominio`: só o campo final conta
		case *ast.Ident:
			if sensivel(x.Name) {
				motivo = "com identificador sensível " + x.Name
			}
		}
		return true
	})
	return motivo
}

func sensivel(nome string) bool { return reSensivel.MatchString(nome) && !idsPermitidos[nome] }

func chamadaMascarada(call *ast.CallExpr, aliasVlog string) bool {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name == "len"
	case *ast.SelectorExpr:
		if fn.Sel.Name == "Mascarado" {
			return true
		}
		id, ok := fn.X.(*ast.Ident)
		return ok && id.Name == aliasVlog && fn.Sel.Name == "MaskEmail"
	}
	return false
}

func textoDe(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return textoDe(x.X) + "." + x.Sel.Name
	}
	return ""
}

// excecaoVlog é um achado conhecido do LEGADO, revisado e benigno. A lista
// só encolhe: cada entrada some quando o arquivo é migrado/reescrito, e uma
// entrada que não casa mais nada faz o teste falhar (lista sempre atual).
type excecaoVlog struct {
	pkgArquivo string // "<import path> <arquivo>.go"
	trecho     string // parte do motivo
	porque     string
}

var excecoesVlogLegado = []excecaoVlog{
	{modShared + "/config config.go", "identificador sensível chave", "nome da variável de ambiente, nunca o valor"},
	{modShared + "/config config.go", "identificador sensível key", "nome da variável de ambiente, nunca o valor"},
	{modShared + "/services email_service.go", "%v no formato", "erro do pool de CAs do sistema (sem dado de usuário)"},
	{modShared + "/tools/resetpassword resetpassword.go", "%v no formato", "booleano admin_encontrado"},
}

func (e excecaoVlog) cobre(v string) bool {
	return strings.HasPrefix(v, e.pkgArquivo+":") && strings.Contains(v, e.trecho)
}

// A regra vale para os DOIS módulos inteiros (R-g), exceto o próprio pacote
// vlog e as exceções revisadas do legado acima. Os pacotes novos não têm
// exceção nenhuma. O TST05 continua sendo a prova em runtime.
func TestARQ_VlogSemConteudoSensivel(t *testing.T) {
	arqs := todosOsArquivos(t)
	usadas := make([]bool, len(excecoesVlogLegado))
	var restantes []string
	for _, v := range violacoesVlog(arqs, func(string) bool { return true }) {
		coberta := false
		for i, e := range excecoesVlogLegado {
			if e.cobre(v) && !pacoteNovo(strings.Fields(v)[0]) {
				usadas[i], coberta = true, true
			}
		}
		if !coberta {
			restantes = append(restantes, v)
		}
	}
	assert.Empty(t, restantes)
	for i, e := range excecoesVlogLegado {
		assert.True(t, usadas[i], "exceção obsoleta (remova da lista): %s %q", e.pkgArquivo, e.trecho)
	}
	assert.Empty(t, violacoesVlog(arqs, pacoteNovo), "pacotes novos não têm exceção")

	vistos := 0
	for _, a := range arqs {
		if pacoteNovo(a.pkg) {
			chamadasA(a, pkgVlog, func(string, *ast.CallExpr) { vistos++ })
		}
	}
	assert.Positive(t, vistos, "a regra deveria enxergar as chamadas de vlog do httpx")
}

func TestARQ_VlogSemConteudoSensivel_DetectaViolacoes(t *testing.T) {
	a := fonteSintetica(t, modShared+"/identidade/identidadeapp", "login.go", `package identidadeapp

import v "github.com/rotaperfumes/shared/vlog"

func f(email, senha, busca string, tokenID int64, cmd Cmd, u Usuario, r *Req, formato string) {
	v.Printf("a.go", "F", "cmd=%v", 1)
	v.Printf("a.go", "F", "cmd=%+v", 1)
	v.Printf("a.go", "F", formato)
	v.Printf("a.go", "F", "e=%s", email)
	v.Printf("a.go", "F", "s=%s", u.PasswordHash)
	v.Printf("a.go", "F", "x=%s", strings.ToLower(senha))
	v.Printf("a.go", "F", "x=%d", cmd)
	v.Printf("a.go", "F", "x=%s", r.URL)
	defer v.Trace("a.go", "F", "x=%s", u.Email)(nil)

	v.Printf("a.go", "F", "e=%s", v.MaskEmail(email))
	v.Printf("a.go", "F", "e=%s", u.Email.Mascarado())
	v.Printf("a.go", "F", "len=%d token_id=%d", len(senha), tokenID)
	v.Printf("a.go", "F", "busca=%t pct=100%%v", busca != "")
	v.Printf("a.go", "F", "id=%d", u.ID)
}
`)
	v := violacoesVlog([]arquivo{a}, func(string) bool { return true })
	assert.Len(t, v, 9, strings.Join(v, "\n"))
}

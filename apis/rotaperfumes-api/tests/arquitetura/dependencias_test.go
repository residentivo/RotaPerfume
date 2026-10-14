package arquitetura_test

// Regras de dependência da camada HTTP da API (§4.1/§4.2): internal/httpx e
// internal/<ctx>http sem banco nem adapters; só internal/bootstrap liga os
// adapters *mysql; nada no shared importa a API.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ehAdapterMySQL: shared/<ctx>/<ctx>mysql.
func ehAdapterMySQL(imp string) bool {
	return strings.HasPrefix(imp, modShared+"/") && strings.HasSuffix(imp, "mysql")
}

// ehHTTPDeContexto: internal/<ctx>http (o kit transversal httpx não conta).
func ehHTTPDeContexto(ip string) bool {
	return strings.HasPrefix(ip, modAPI+"/internal/") && strings.HasSuffix(ip, "http") && ip != pkgHttpx
}

func violacoesDeDependencia(pacotes []pacote) []string {
	var out []string
	falha := func(p pacote, imp, motivo string) {
		out = append(out, fmt.Sprintf("%s importa %s (%s)", p.importPath, imp, motivo))
	}
	for _, p := range pacotes {
		for _, imp := range p.imports {
			switch {
			case p.importPath == pkgHttpx && (dentroDe(imp, "database/sql") || ehAdapterMySQL(imp)):
				falha(p, imp, "httpx não fala com o banco")
			case p.importPath == pkgHttpx && (dentroDe(imp, modAPI+"/handlers") || dentroDe(imp, modAPI+"/services") ||
				dentroDe(imp, modShared+"/repositories") || dentroDe(imp, modAPI+"/internal/bootstrap")):
				falha(p, imp, "httpx não depende do legado")
			case p.importPath == pkgHttpx && (dentroDe(imp, modAPI+"/middleware") || dentroDe(imp, modShared+"/config")):
				falha(p, imp, "ARQ-03: httpx não depende do middleware legado nem da config")
			case ehHTTPDeContexto(p.importPath) && (dentroDe(imp, "database/sql") || ehAdapterMySQL(imp) ||
				dentroDe(imp, modShared+"/repositories") || dentroDe(imp, modShared+"/config")):
				falha(p, imp, "<ctx>http só fala com <ctx>app")
			case ehAdapterMySQL(imp) && !dentroDe(p.importPath, modAPI+"/internal/bootstrap") && strings.HasPrefix(p.importPath, modAPI):
				falha(p, imp, "só o bootstrap liga adapters *mysql")
			case strings.HasPrefix(p.importPath, modShared) && dentroDe(imp, modAPI):
				falha(p, imp, "shared não importa a API")
			}
		}
	}
	return out
}

func TestARQ_Dependencias_API(t *testing.T) {
	assert.Empty(t, violacoesDeDependencia(pacotesDosDoisModulos(t)))
}

func TestARQ_Dependencias_API_DetectaViolacoes(t *testing.T) {
	ruins := []pacote{
		{importPath: pkgHttpx, imports: []string{"database/sql"}},
		{importPath: pkgHttpx, imports: []string{modShared + "/catalogo/catalogomysql"}},
		{importPath: pkgHttpx, imports: []string{modAPI + "/handlers"}},
		{importPath: pkgHttpx, imports: []string{modAPI + "/middleware"}},
		{importPath: pkgHttpx, imports: []string{modShared + "/config"}},
		{importPath: modAPI + "/internal/catalogohttp", imports: []string{modShared + "/repositories"}},
		{importPath: modAPI + "/internal/catalogohttp", imports: []string{modShared + "/config"}},
		{importPath: modAPI + "/handlers", imports: []string{modShared + "/catalogo/catalogomysql"}},
		{importPath: modShared + "/kernel", imports: []string{pkgHttpx}},
	}
	assert.Len(t, violacoesDeDependencia(ruins), len(ruins))

	bons := []pacote{
		{importPath: pkgHttpx, imports: []string{"net/http", pkgKernel, pkgVlog, modShared + "/identidade"}},
		{importPath: modAPI + "/internal/bootstrap", imports: []string{"database/sql", modShared + "/catalogo/catalogomysql"}},
		{importPath: modAPI + "/internal/catalogohttp", imports: []string{pkgHttpx, modShared + "/catalogo/catalogoapp"}},
	}
	assert.Empty(t, violacoesDeDependencia(bons))
}

// Sanidade: a varredura enxerga os dois módulos.
func TestARQ_Varredura_DoisModulos(t *testing.T) {
	achou := map[string]bool{}
	for _, p := range pacotesDosDoisModulos(t) {
		achou[p.importPath] = len(p.arquivos) > 0
	}
	for _, esperado := range []string{pkgHttpx, modAPI + "/internal/bootstrap", modAPI + "/middleware", pkgKernel, pkgVlog, modShared + "/repositories"} {
		assert.True(t, achou[esperado], "varredura não encontrou %s", esperado)
	}
}

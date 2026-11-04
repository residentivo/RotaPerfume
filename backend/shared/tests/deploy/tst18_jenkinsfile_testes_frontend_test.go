package deploy_test

// TST-18 (2026-11-04): estágio "Testes Frontend" do Jenkinsfile.
//
// O pipeline roda npm ci + lint + vitest do frontend/ num container
// NODE_IMAGE fixado por digest (SEC-31), entre "Testes Go" e "Build", com o
// código enviado por stdin (tar), como o "Testes Go". Os arquivos do
// frontend/ lidos aqui (.nvmrc, Dockerfile, package*.json, tests/) vão no tar
// do "Testes Go" (senão este teste falha no Jenkins, de propósito).

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// estagioJenkins devolve o corpo do stage(nome) até o próximo stage( (ou o
// fim do bloco stages).
func estagioJenkins(t *testing.T, jf, nome string) string {
	t.Helper()
	marca := "stage('" + nome + "')"
	i := strings.Index(jf, marca)
	require.GreaterOrEqual(t, i, 0, "estágio %q no Jenkinsfile", nome)
	resto := jf[i+len(marca):]
	if j := strings.Index(resto, "stage('"); j >= 0 {
		resto = resto[:j]
	}
	return resto
}

func TestTST18_Jenkinsfile_OrdemDosEstagios(t *testing.T) {
	jf := lerRaiz(t, "Jenkinsfile")
	casos := []struct{ antes, depois string }{
		{"Checkout", "Testes Go"},
		{"Testes Go", "Testes Frontend"},
		{"Testes Frontend", "Build"},
		{"Testes Frontend", "Deploy"},
	}
	for _, c := range casos {
		t.Run(c.antes+" antes de "+c.depois, func(t *testing.T) {
			iA := strings.Index(jf, "stage('"+c.antes+"')")
			iD := strings.Index(jf, "stage('"+c.depois+"')")
			require.GreaterOrEqual(t, iA, 0)
			require.GreaterOrEqual(t, iD, 0)
			assert.Less(t, iA, iD)
		})
	}
	assert.Equal(t, 1, strings.Count(jf, "stage('Testes Frontend')"), "um único estágio")
}

var reNodeImage = regexp.MustCompile(`^node:(\d+\.\d+\.\d+)-alpine@sha256:[0-9a-f]{64}$`)

// SEC-31: NODE_IMAGE fixada por digest, com a versão do frontend/.nvmrc e a
// mesma linha major do NODE_VERSION do frontend/Dockerfile.
func TestTST18_NodeImageFixadaPorDigest(t *testing.T) {
	jf := lerRaiz(t, "Jenkinsfile")
	m := regexp.MustCompile(`(?m)^\s*NODE_IMAGE\s*=\s*'([^']+)'`).FindStringSubmatch(jf)
	require.NotNil(t, m, "NODE_IMAGE no environment do Jenkinsfile")
	img := reNodeImage.FindStringSubmatch(m[1])
	require.NotNil(t, img, "node:<x.y.z>-alpine@sha256:<64 hex>: %q", m[1])

	nvmrc := strings.TrimPrefix(strings.TrimSpace(lerRaiz(t, "frontend", ".nvmrc")), "v")
	assert.Equal(t, nvmrc, img[1], "versão do NODE_IMAGE = frontend/.nvmrc")

	df := lerRaiz(t, "frontend", "Dockerfile")
	mv := regexp.MustCompile(`(?m)^ARG NODE_VERSION=(\d+)`).FindStringSubmatch(df)
	require.NotNil(t, mv, "ARG NODE_VERSION no frontend/Dockerfile")
	assert.Equal(t, mv[1], strings.SplitN(img[1], ".", 2)[0], "mesma linha major do Dockerfile")

	// O "make deps-audit-imagens" acha as imagens pelo padrão tag@sha256 nos
	// arquivos de IMAGENS_FIXADAS_EM (o Jenkinsfile é um deles).
	refs := regexp.MustCompile(`[a-z0-9._/-]+:[A-Za-z0-9._-]+@sha256:[0-9a-f]{64}`).FindAllString(jf, -1)
	assert.Contains(t, refs, m[1], "visível para o deps-audit-imagens")
	assert.Contains(t, jf, "node:22.23-alpine", "comentário SEC-31 explica como atualizar o node")
}

func TestTST18_EstagioTestesFrontend_Comandos(t *testing.T) {
	corpo := estagioJenkins(t, lerRaiz(t, "Jenkinsfile"), "Testes Frontend")
	casos := []struct {
		nome, trecho string
	}{
		{"shebang (sem sh -xe)", "sh '''#!/bin/bash"},
		{"pipefail", "set -euo pipefail"},
		{"container descartável por stdin", `docker run --rm -i`},
		{"imagem fixada", `"$NODE_IMAGE"`},
		{"extrai o tar", "tar -xf - -C /src"},
		{"entra no frontend", "cd /src/frontend"},
		{"npm ci pelo lockfile", "npm ci"},
		{"lint", "npm run lint"},
		{"testes", "npm test"},
		{"sem node_modules", "--exclude=frontend/node_modules"},
		{"sem .next", "--exclude=frontend/.next"},
		{"sem coverage", "--exclude=frontend/coverage"},
		{"sem tsbuildinfo", "--exclude=frontend/tsconfig.tsbuildinfo"},
		{"sem .env", "--exclude=frontend/.env"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			assert.Contains(t, corpo, c.trecho)
		})
	}

	ordem := []string{"npm ci", "npm run lint", "npm test"}
	for i := 1; i < len(ordem); i++ {
		assert.Less(t, strings.Index(corpo, ordem[i-1]), strings.Index(corpo, ordem[i]), "%s antes de %s", ordem[i-1], ordem[i])
	}

	proibidos := []struct{ nome, re string }{
		{"bind mount -v", `docker run[^\n]*\s-v\s`},
		{"--mount", `--mount`},
		{"npm install (sem lockfile)", `npm (install|i)\b`},
		{"credenciais", `withCredentials`},
		{"set -x", `set -[a-z]*x`},
		{"rede do banco", `--network`},
	}
	for _, p := range proibidos {
		t.Run("sem "+p.nome, func(t *testing.T) {
			assert.NotRegexp(t, p.re, corpo)
		})
	}
}

// Os scripts do package.json que o estágio chama.
func TestTST18_PackageJsonScripts(t *testing.T) {
	pj := lerRaiz(t, "frontend", "package.json")
	for _, s := range []string{`"lint": "eslint ."`, `"test": "vitest run"`} {
		t.Run(s, func(t *testing.T) { assert.Contains(t, pj, s) })
	}
	_, err := os.Stat(caminhoRaiz(t, "frontend", "package-lock.json"))
	require.NoError(t, err, "npm ci exige o lockfile")
}

var (
	// import ... from "../../../backend/..."
	reImportForaFrontend = regexp.MustCompile(`["'](?:\.\./)+(backend/[A-Za-z0-9_./-]+)["']`)
	// resolve(process.cwd(), "..", "backend", "shared", ...)
	reResolveForaFrontend = regexp.MustCompile(`resolve\(\s*process\.cwd\(\)\s*,\s*"\.\."\s*,((?:\s*"[^"]+"\s*,?)+)\)`)
	reStringJS            = regexp.MustCompile(`"([^"]+)"`)
)

// arquivosForaDoFrontend varre frontend/tests e lista os caminhos (relativos à
// raiz) que os testes leem fora de frontend/.
func arquivosForaDoFrontend(t *testing.T) []string {
	t.Helper()
	vistos := map[string]bool{}
	err := filepath.WalkDir(caminhoRaiz(t, "frontend", "tests"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		src := string(b)
		for _, m := range reImportForaFrontend.FindAllStringSubmatch(src, -1) {
			vistos[m[1]] = true
		}
		for _, m := range reResolveForaFrontend.FindAllStringSubmatch(src, -1) {
			var partes []string
			for _, s := range reStringJS.FindAllStringSubmatch(m[1], -1) {
				partes = append(partes, s[1])
			}
			vistos[strings.Join(partes, "/")] = true
		}
		return nil
	})
	require.NoError(t, err)
	out := make([]string, 0, len(vistos))
	for k := range vistos {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Todo arquivo de fora do frontend/ lido pelos testes do Vitest vai no tar
// do estágio, no mesmo caminho (senão o vitest falha no Jenkins).
func TestTST18_ArquivosForaDoFrontendNoTar(t *testing.T) {
	corpo := estagioJenkins(t, lerRaiz(t, "Jenkinsfile"), "Testes Frontend")
	arquivos := arquivosForaDoFrontend(t)
	require.NotEmpty(t, arquivos, "a varredura acha os testes cruzados (CNPJ, contrato de erros)")
	assert.Contains(t, arquivos, "backend/shared/vendas/erros.go")
	assert.Contains(t, arquivos, "backend/shared/cnpj/testdata/casos_cruzados.json")
	for _, a := range arquivos {
		t.Run(a, func(t *testing.T) {
			_, err := os.Stat(caminhoRaiz(t, filepath.FromSlash(a)))
			require.NoError(t, err, "arquivo existe no repositório")
			assert.Regexp(t, `\s`+regexp.QuoteMeta(a)+`(\s|$)`, corpo, "%s no tar do estágio", a)
		})
	}
}

// Os arquivos do frontend/ lidos por este teste vão no tar do "Testes Go".
func TestTST18_TesteGoEnviaArquivosDoFrontend(t *testing.T) {
	corpo := estagioJenkins(t, lerRaiz(t, "Jenkinsfile"), "Testes Go")
	for _, a := range []string{"frontend/.nvmrc", "frontend/Dockerfile", "frontend/package.json", "frontend/package-lock.json", "frontend/tests"} {
		t.Run(a, func(t *testing.T) {
			assert.Regexp(t, `\s`+regexp.QuoteMeta(a)+`(\s|$)`, corpo)
		})
	}
}

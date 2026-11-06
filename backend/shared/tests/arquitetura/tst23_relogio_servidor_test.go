package arquitetura_test

// TST-23 (Lote 37) — DECISÃO REGISTRADA sobre fuso/relógio no SQL:
//
//  1. O fuso fica no Go: o pacote tz fixa time.Local em -03:00 e o DSN usa
//     loc=Local (config.DSN; tests/config/config_test.go garante).
//  2. O DSN fixa o time_zone da SESSÃO em '-03:00' (config.DSN, derivado de
//     tz; DB-08), para o DEFAULT/ON UPDATE CURRENT_TIMESTAMP das colunas
//     TIMESTAMP gravar o instante certo. Isso não libera o relógio do
//     servidor em DML: o item 3 continua valendo (CLI e seeds rodam em
//     sessão UTC).
//  3. Nenhum SQL de produção ou de teste usa o relógio do servidor
//     (NOW(), CURRENT_TIMESTAMP, CURDATE() & cia.): datas e horas são
//     calculadas no Go e passadas como parâmetro (?), por exemplo
//     time.Now().Add(-5*time.Minute).Truncate(time.Second).
//
// Motivo: com o servidor em UTC e o Go em -03, NOW()/CURDATE() divergem
// do relógio da aplicação em 3 h (e de dia entre 21h e 24h -03). O aceite é
// a integração verde no banco descartável com --default-time-zone='+00:00'
// e '-03:00'.
//
// Catraca: varre por AST os literais string de TODOS os .go dos dois
// módulos de backend/ (produção e _test). Comentários não são literais e
// ficam fora; `SELECT @@session.time_zone` não casa com o padrão. Cada
// exceção fica na allowlist por arquivo:linha com o motivo; entrada que não
// casa mais com nada falha (allowlist sem sobra). Este arquivo fica fora da
// varredura porque contém o próprio padrão e as amostras do teste dele.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var reRelogioServidor = regexp.MustCompile(`(?i)\b(NOW|CURRENT_TIMESTAMP|CURRENT_DATE|CURRENT_TIME|CURDATE|CURTIME|SYSDATE|UTC_TIMESTAMP|UTC_DATE|UTC_TIME|UNIX_TIMESTAMP|LOCALTIME|LOCALTIMESTAMP)\b`)

const arquivoCatracaTST23 = "shared/tests/arquitetura/tst23_relogio_servidor_test.go"

const motivoTextoNaoSQL = "texto descritivo (nome de subteste/mensagem), não é SQL"

// allowlistRelogioServidor: caminho relativo a backend/ + ":" + linha.
var allowlistRelogioServidor = map[string]string{
	// Falsos positivos do padrão case-insensitive ("now" em inglês).
	"rotaperfumes-api/tests/services/refresh_clock_test.go:25":              motivoTextoNaoSQL,
	"rotaperfumes-api/tests/services/sec12_chore02_integration_test.go:182": motivoTextoNaoSQL,
	"rotaperfumes-api/tests/services/sec12_chore02_integration_test.go:183": motivoTextoNaoSQL,
	"rotaperfumes-api/tests/services/sec12_chore02_integration_test.go:184": motivoTextoNaoSQL,
	"rotaperfumes-api/tests/services/sec12_chore02_integration_test.go:185": motivoTextoNaoSQL,
	"shared/tests/repositories/sec10_invalidar_sessoes_truncate_test.go:44": motivoTextoNaoSQL,
}

type achadoRelogio struct {
	local, funcao, trecho string
}

// varrerRelogioServidor percorre backend/ (raiz relativa a este pacote) e
// devolve os literais string que casam com reRelogioServidor, e quantos
// arquivos .go foram analisados.
func varrerRelogioServidor(t *testing.T) ([]achadoRelogio, int) {
	t.Helper()
	backend, err := filepath.Abs("../../..")
	require.NoError(t, err)
	var achados []achadoRelogio
	arquivos := 0
	for _, modulo := range []string{"shared", "rotaperfumes-api"} {
		err := filepath.WalkDir(filepath.Join(backend, modulo), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") {
				return nil
			}
			rel, err := filepath.Rel(backend, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if rel == arquivoCatracaTST23 {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			arquivos++
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				valor, err := strconv.Unquote(lit.Value)
				if err != nil {
					valor = lit.Value
				}
				if m := reRelogioServidor.FindString(valor); m != "" {
					achados = append(achados, achadoRelogio{
						local:  rel + ":" + strconv.Itoa(fset.Position(lit.Pos()).Line),
						funcao: m,
						trecho: strings.Join(strings.Fields(valor), " "),
					})
				}
				return true
			})
			return nil
		})
		require.NoError(t, err, modulo)
	}
	return achados, arquivos
}

func TestTST23_NenhumSQLUsaORelogioDoServidor(t *testing.T) {
	achados, arquivos := varrerRelogioServidor(t)
	assert.Greater(t, arquivos, 300, "a varredura alcançou os dois módulos")

	vistos := map[string]bool{}
	var violacoes []string
	for _, a := range achados {
		vistos[a.local] = true
		if _, ok := allowlistRelogioServidor[a.local]; !ok {
			violacoes = append(violacoes, a.local+" ["+a.funcao+"]: "+a.trecho)
		}
	}
	sort.Strings(violacoes)
	assert.Empty(t, violacoes, "SQL com relógio do servidor: calcular no Go e passar como parâmetro (decisão TST-23 no topo deste arquivo)")

	var sobras []string
	for local, motivo := range allowlistRelogioServidor {
		assert.NotEmpty(t, strings.TrimSpace(motivo), "allowlist sem motivo: %s", local)
		if !vistos[local] {
			sobras = append(sobras, local)
		}
	}
	sort.Strings(sobras)
	assert.Empty(t, sobras, "entradas da allowlist que não casam mais com nada: remover")
}

// O teste da sec12 (repositories) que motivou o card não volta a usar o
// relógio do servidor: nenhuma entrada da allowlist pode cobri-lo.
func TestTST23_Sec12RefreshNaoEstaNaAllowlist(t *testing.T) {
	for local := range allowlistRelogioServidor {
		assert.False(t, strings.HasPrefix(local, "shared/tests/repositories/sec12_chore02_refresh_integration_test.go:"), local)
	}
}

func TestTST23_PadraoDoRelogioDoServidor(t *testing.T) {
	casos := []struct {
		sql   string
		casar bool
	}{
		{"UPDATE t SET revoked_at = NOW() - INTERVAL 5 MINUTE", true},
		{"update t set x = now()", true},
		{"DEFAULT CURRENT_TIMESTAMP", true},
		{"WHERE d = CURRENT_DATE", true},
		{"WHERE d = CURDATE()", true},
		{"SELECT CURTIME()", true},
		{"SELECT SYSDATE()", true},
		{"SELECT UTC_TIMESTAMP()", true},
		{"SELECT utc_date()", true},
		{"SELECT UNIX_TIMESTAMP()", true},
		{"SELECT LOCALTIME", true},
		{"SELECT LOCALTIMESTAMP()", true},
		{`DATE_SUB\(CURDATE\(\), INTERVAL 6 DAY\)`, true},
		{"SELECT @@session.time_zone", false},
		{"UPDATE t SET revoked_at = ? WHERE id = ?", false},
		{"SELECT nowhere, known, snow FROM t", false},
		{"SELECT current_timestamps FROM t", false},
		{"SELECT data_cadastro FROM clientes", false},
	}
	for _, c := range casos {
		t.Run(c.sql, func(t *testing.T) {
			assert.Equal(t, c.casar, reRelogioServidor.MatchString(c.sql))
		})
	}
}

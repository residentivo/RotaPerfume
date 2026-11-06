package arquitetura_test

// DB-08 (Lote 38) — catraca do relógio do servidor no SQL versionado.
//
// Com o time_zone da SESSÃO em '-03:00' (config.DSN, tz.OffsetSQL), o único
// uso aceito do relógio do servidor no SQL versionado é o DEFAULT/ON UPDATE
// CURRENT_TIMESTAMP das colunas de auditoria `created_at`/`updated_at`
// TIMESTAMP. Qualquer outro casamento com reRelogioServidor (definido na
// catraca TST-23) exige entrada na allowlist por arquivo:linha com motivo; a
// entrada que não casa mais com nada falha (allowlist sem sobra).
//
// TST-24 (Lote 39): sql/*.sql passam por um lexer do ARQUIVO inteiro, igual ao
// do MariaDB, que remove só os comentários e mantém os "\n" (os números de
// linha continuam os do arquivo). O conteúdo das strings fica e é varrido
// (fail-closed: SQL de PREPARE continua conferido). String ou bloco não
// fechado no fim do arquivo é erro.
//
// TST-25 (Lote 39): deploy/*.sh, scripts/*.sh, Makefile e Jenkinsfile não são
// SQL: só as linhas cujo primeiro não-espaço é "#" ficam fora, sem a remoção de
// comentários SQL, com allowlist própria.
//
// As amostras deste teste são montadas por concatenação para não casarem com a
// catraca TST-23, que varre os literais dos .go.

import (
	"fmt"
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

// Amostras sem o literal inteiro (ver cabeçalho). db08Relogio também compõe os
// padrões abaixo.
const (
	db08Relogio = "CURRENT_" + "TIMESTAMP"
	db08Curdate = "CUR" + "DATE()"
	db08Agora   = "N" + "OW()"
)

// db08ColunaAuditoria casa a definição de coluna `created_at`/`updated_at`
// TIMESTAMP (crases opcionais: sql/07 não as usa).
var db08ColunaAuditoria = regexp.MustCompile("(?i)^\\s*`?(created_at|updated_at)`?\\s+TIMESTAMP\\b")

// db08DefaultAuditoria é o único trecho aceito nessas colunas:
// DEFAULT CURRENT_TIMESTAMP [ON UPDATE CURRENT_TIMESTAMP].
var db08DefaultAuditoria = regexp.MustCompile(`(?i)\bDEFAULT\s+` + db08Relogio + `(\(\))?(\s+ON\s+UPDATE\s+` + db08Relogio + `(\(\))?)?`)

// db08AllowlistSQL: caminho relativo à raiz do repositório + ":" + linha.
var db08AllowlistSQL = map[string]string{
	"sql/25_seed_movimentacoes_mes_atual.sql:65": "seed de dev: o mês corrente vem do relógio do servidor, protegido pelo SET time_zone = '-03:00' da linha 61 (sessão do cliente mariadb em -03)",
}

// db08AllowlistEmbutido: mesma forma, para os arquivos que não são SQL. Hoje
// não há exceção.
var db08AllowlistEmbutido = map[string]string{}

// db08FimDoDelimitado devolve o índice logo depois do fechamento do trecho
// aberto em fonte[inicio] (', " ou crase). Em ' e ", "\x" é escape. A aspa
// dobrada fecha e reabre o trecho, o que dá o mesmo resultado de continuar
// dentro dele.
func db08FimDoDelimitado(fonte string, inicio int) (int, error) {
	abre := fonte[inicio]
	for i := inicio + 1; i < len(fonte); i++ {
		switch {
		case fonte[i] == '\\' && abre != '`':
			i++
		case fonte[i] == abre:
			return i + 1, nil
		}
	}
	return 0, fmt.Errorf("%c aberta na linha %d não fechada no fim do arquivo", abre, db08NumeroDaLinha(fonte, inicio))
}

func db08NumeroDaLinha(fonte string, indice int) int {
	return strings.Count(fonte[:indice], "\n") + 1
}

// db08ComentarioDeLinha: "#", ou "--" seguido de espaço, controle ou fim do
// arquivo. No MariaDB, "--CURDATE()" é "- -CURDATE()", ou seja, código.
func db08ComentarioDeLinha(fonte string, i int) bool {
	if fonte[i] == '#' {
		return true
	}
	if !strings.HasPrefix(fonte[i:], "--") {
		return false
	}
	return i+2 >= len(fonte) || fonte[i+2] <= ' ' || fonte[i+2] == 0x7f
}

// db08TamanhoCondicional devolve o tamanho de "/*!" ou "/*M!" mais os dígitos
// da versão em fonte[i:], ou 0 se não for bloco condicional. Sem separar os
// dígitos, "/*!50000CURDATE()*/" passaria pelo \b do padrão.
func db08TamanhoCondicional(fonte string, i int) int {
	resto := fonte[i:]
	tamanho := 0
	switch {
	case strings.HasPrefix(resto, "/*!"):
		tamanho = len("/*!")
	case strings.HasPrefix(resto, "/*M!"):
		tamanho = len("/*M!")
	default:
		return 0
	}
	for tamanho < len(resto) && resto[tamanho] >= '0' && resto[tamanho] <= '9' {
		tamanho++
	}
	return tamanho
}

// db08RemoverComentariosSQL é o lexer do TST-24: tira os comentários do arquivo
// (um espaço no lugar, porque o comentário também separa tokens) e mantém os
// "\n". "/*!" e "/*M!" ficam como código: o servidor executa o conteúdo deles.
func db08RemoverComentariosSQL(fonte string) (string, error) {
	var saida strings.Builder
	condicionaisAbertos, ultimoCondicional := 0, 0
	for i := 0; i < len(fonte); {
		tamanhoCondicional := db08TamanhoCondicional(fonte, i)
		switch {
		case fonte[i] == '\'' || fonte[i] == '"' || fonte[i] == '`':
			fim, err := db08FimDoDelimitado(fonte, i)
			if err != nil {
				return "", err
			}
			saida.WriteString(fonte[i:fim])
			i = fim
		case db08ComentarioDeLinha(fonte, i):
			fimDaLinha := strings.IndexByte(fonte[i:], '\n')
			if fimDaLinha < 0 {
				return saida.String(), db08CondicionalFechado(fonte, condicionaisAbertos, ultimoCondicional)
			}
			saida.WriteByte(' ')
			i += fimDaLinha
		case tamanhoCondicional > 0:
			condicionaisAbertos++
			ultimoCondicional = i
			saida.WriteString(fonte[i:i+tamanhoCondicional] + " ")
			i += tamanhoCondicional
		case strings.HasPrefix(fonte[i:], "/*"):
			fim := strings.Index(fonte[i+2:], "*/")
			if fim < 0 {
				return "", fmt.Errorf("bloco /* aberto na linha %d não fechado no fim do arquivo", db08NumeroDaLinha(fonte, i))
			}
			fim += i + 2 + len("*/")
			saida.WriteString(" " + strings.Repeat("\n", strings.Count(fonte[i:fim], "\n")))
			i = fim
		case strings.HasPrefix(fonte[i:], "*/") && condicionaisAbertos > 0:
			condicionaisAbertos--
			saida.WriteString("*/")
			i += len("*/")
		default:
			saida.WriteByte(fonte[i])
			i++
		}
	}
	return saida.String(), db08CondicionalFechado(fonte, condicionaisAbertos, ultimoCondicional)
}

func db08CondicionalFechado(fonte string, abertos, ultimo int) error {
	if abertos == 0 {
		return nil
	}
	return fmt.Errorf("bloco condicional aberto na linha %d não fechado no fim do arquivo", db08NumeroDaLinha(fonte, ultimo))
}

// db08LinhaViolaRelogio: a linha, já sem comentários quando o arquivo é SQL, usa
// o relógio do servidor fora do DEFAULT/ON UPDATE das colunas de auditoria.
func db08LinhaViolaRelogio(linha string) bool {
	if db08ColunaAuditoria.MatchString(linha) {
		linha = db08DefaultAuditoria.ReplaceAllString(linha, "")
	}
	return reRelogioServidor.MatchString(linha)
}

// db08Analisador devolve número da linha -> trecho das violações de um arquivo.
type db08Analisador func(fonte string) (map[int]string, error)

// db08ViolacoesSQL analisa um arquivo SQL (TST-24).
func db08ViolacoesSQL(fonte string) (map[int]string, error) {
	codigo, err := db08RemoverComentariosSQL(fonte)
	if err != nil {
		return nil, err
	}
	violacoes := map[int]string{}
	for i, linha := range strings.Split(codigo, "\n") {
		if db08LinhaViolaRelogio(linha) {
			violacoes[i+1] = strings.TrimSpace(linha)
		}
	}
	return violacoes, nil
}

// db08ViolacoesEmbutido analisa um arquivo que não é SQL (TST-25). Fora as
// linhas iniciadas por "#", tudo conta: um "-- " de shell ou um "#" no meio da
// linha não escondem nada.
func db08ViolacoesEmbutido(fonte string) (map[int]string, error) {
	violacoes := map[int]string{}
	for i, linha := range strings.Split(fonte, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(linha), "#") && db08LinhaViolaRelogio(linha) {
			violacoes[i+1] = strings.TrimSpace(linha)
		}
	}
	return violacoes, nil
}

// db08Varrer aplica o analisador à lista de arquivos (relativos à raiz) e
// devolve "arquivo:linha" -> trecho. Arquivo ilegível ou mal formado é erro.
func db08Varrer(raiz string, arquivos []string, analisar db08Analisador) (map[string]string, error) {
	achados := map[string]string{}
	for _, rel := range arquivos {
		conteudo, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		violacoes, err := analisar(string(conteudo))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		for linha, trecho := range violacoes {
			achados[rel+":"+strconv.Itoa(linha)] = trecho
		}
	}
	return achados, nil
}

func db08Raiz(t *testing.T) string {
	t.Helper()
	raiz, err := filepath.Abs("../../../..")
	require.NoError(t, err)
	return raiz
}

// db08Listar expande o glob (relativo à raiz) em caminhos relativos à raiz.
func db08Listar(t *testing.T, raiz, padrao string) []string {
	t.Helper()
	caminhos, err := filepath.Glob(filepath.Join(raiz, filepath.FromSlash(padrao)))
	require.NoError(t, err)
	rels := make([]string, 0, len(caminhos))
	for _, caminho := range caminhos {
		rel, err := filepath.Rel(raiz, caminho)
		require.NoError(t, err)
		rels = append(rels, filepath.ToSlash(rel))
	}
	return rels
}

// db08Confrontar separa os achados fora da allowlist (violações) e as entradas
// da allowlist que não casam com nada (sobras) ou estão sem motivo.
func db08Confrontar(achados, allowlist map[string]string) (violacoes, sobras, semMotivo []string) {
	for local, trecho := range achados {
		if _, ok := allowlist[local]; !ok {
			violacoes = append(violacoes, local+": "+trecho)
		}
	}
	for local, motivo := range allowlist {
		if strings.TrimSpace(motivo) == "" {
			semMotivo = append(semMotivo, local)
		}
		if _, ok := achados[local]; !ok {
			sobras = append(sobras, local)
		}
	}
	sort.Strings(violacoes)
	sort.Strings(sobras)
	sort.Strings(semMotivo)
	return violacoes, sobras, semMotivo
}

func db08ConferirCatraca(t *testing.T, achados, allowlist map[string]string) {
	t.Helper()
	violacoes, sobras, semMotivo := db08Confrontar(achados, allowlist)
	assert.Empty(t, violacoes, "relógio do servidor fora do DEFAULT de created_at/updated_at TIMESTAMP (DB-08): calcular no Go ou justificar na allowlist")
	assert.Empty(t, sobras, "entradas da allowlist que não casam mais com nada: remover")
	assert.Empty(t, semMotivo, "entradas da allowlist sem motivo")
}

func TestDB08_SQLSoUsaRelogioNoDefaultDeAuditoria(t *testing.T) {
	raiz := db08Raiz(t)
	arquivos := db08Listar(t, raiz, "sql/*.sql")
	assert.Greater(t, len(arquivos), 20, "a varredura alcançou sql/*.sql")

	achados, err := db08Varrer(raiz, arquivos, db08ViolacoesSQL)
	require.NoError(t, err)
	db08ConferirCatraca(t, achados, db08AllowlistSQL)
}

func TestTST25_SQLEmbutidoSoUsaRelogioNoDefaultDeAuditoria(t *testing.T) {
	raiz := db08Raiz(t)
	deploy := db08Listar(t, raiz, "deploy/*.sh")
	scripts := db08Listar(t, raiz, "scripts/*.sh")
	assert.GreaterOrEqual(t, len(deploy), 3, "a varredura alcançou deploy/*.sh")
	assert.NotEmpty(t, scripts, "a varredura alcançou scripts/*.sh")

	// Makefile e Jenkinsfile entram pelo nome: se sumirem, db08Varrer dá erro.
	arquivos := append(append(deploy, scripts...), "Makefile", "Jenkinsfile")
	achados, err := db08Varrer(raiz, arquivos, db08ViolacoesEmbutido)
	require.NoError(t, err)
	db08ConferirCatraca(t, achados, db08AllowlistEmbutido)
}

func TestDB08_PadraoDaCatracaSQL(t *testing.T) {
	casos := []struct {
		nome   string
		linha  string
		violar bool
	}{
		{"created_at com DEFAULT", "    `created_at` TIMESTAMP NOT NULL DEFAULT " + db08Relogio + " COMMENT 'x',", false},
		{"updated_at com ON UPDATE", "    `updated_at` TIMESTAMP NOT NULL DEFAULT " + db08Relogio + " ON UPDATE " + db08Relogio + ",", false},
		{"created_at sem crases (sql/07)", "    created_at TIMESTAMP DEFAULT " + db08Relogio + ",", false},
		{"DEFAULT com parênteses", "`created_at` TIMESTAMP DEFAULT " + db08Relogio + "()", false},
		{"linha de comentário", "-- usa " + db08Curdate + " no seed", false},
		{"comentário no fim da linha", "SET @x = 1; -- " + db08Curdate, false},
		{"coluna sem relógio", "`ultimo_login_at` TIMESTAMP NULL DEFAULT NULL,", false},
		{"outra coluna com DEFAULT", "`expires_at` DATETIME NOT NULL DEFAULT " + db08Relogio + ",", true},
		{"TIMESTAMP de outra coluna", "`criado` TIMESTAMP DEFAULT " + db08Relogio + ",", true},
		{"created_at DATETIME", "`created_at` DATETIME DEFAULT " + db08Relogio + ",", true},
		{"auditoria com relógio extra", "`created_at` TIMESTAMP DEFAULT " + db08Relogio + " CHECK (created_at <= " + db08Relogio + ")", true},
		{"DML com relógio", "UPDATE t SET d = " + db08Curdate + " WHERE id = 1;", true},
		{"seed com relógio antes do comentário", "SET @ref = LAST_DAY(" + db08Curdate + ");  -- referência", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			violacoes, err := db08ViolacoesSQL(c.linha)
			require.NoError(t, err)
			assert.Equal(t, c.violar, len(violacoes) > 0, c.linha)
		})
	}
}

// TST-24: o lexer não pode falhar aberto com "-- " dentro de string nem aceitar
// como comentário o que o MariaDB executa.
func TestTST24_LexerDeComentariosSQL(t *testing.T) {
	casos := []struct {
		nome   string
		fonte  string
		violar bool
	}{
		{"-- dentro de aspas simples", "SELECT '-- ', " + db08Relogio, true},
		{"-- dentro de aspas duplas", `SELECT "-- ", ` + db08Agora, true},
		{"-- dentro de crases", "SELECT `-- `, " + db08Agora, true},
		{"aspa dobrada continua na string", "SELECT 'it''s -- ', " + db08Curdate, true},
		{"aspa dupla dobrada continua na string", `SELECT "a""b -- ", ` + db08Curdate, true},
		{"aspa escapada continua na string", `SELECT 'a\'-- ', ` + db08Curdate, true},
		{"barra escapada fecha a string", `SELECT 'a\\' -- ` + db08Curdate, false},
		{"crase não tem escape com barra", "SELECT `a\\` -- " + db08Curdate, false},
		{"relógio dentro de string é varrido", "PREPARE s FROM 'SELECT " + db08Agora + "';", true},
		{"--relógio é código", "SELECT 1 --" + db08Curdate, true},
		{"-- com tab é comentário", "SELECT 1 --\t" + db08Curdate, false},
		{"-- com CRLF é comentário", "SELECT 1 --\r\nSELECT 2;", false},
		{"-- no fim do arquivo", "SELECT 1 --", false},
		{"-- no fim da linha não esconde a seguinte", "SELECT 1 --\n" + db08Curdate, true},
		{"# é comentário", "# " + db08Agora, false},
		{"# colado no código é comentário", "SELECT 1#" + db08Agora, false},
		{"# dentro de string não é comentário", "SELECT '# " + db08Agora + "'", true},
		{"bloco é comentário", "/* " + db08Agora + " */", false},
		{"bloco não engana com aspa", "/* it's */ SELECT 1;", false},
		{"/*! é código", "/*! " + db08Agora + " */", true},
		{"/*M! é código", "/*M! " + db08Agora + " */", true},
		{"/*! com versão colada é código", "/*!50000" + db08Agora + " */", true},
		{"/*M! com versão colada é código", "/*M!100100" + db08Agora + " */", true},
		{"/*m! minúsculo é comentário", "/*m! " + db08Agora + " */", false},
		{"bloco e -- dentro de /*! são comentários", "/*! /* " + db08Agora + " */ SELECT 1 -- " + db08Agora + "\n*/", false},
		{"string multilinha com relógio", "SELECT 'a\n" + db08Agora + "\nb';", true},
		{"bloco multilinha com relógio", "/* a\n" + db08Agora + "\nb */ SELECT 1;", false},
		{"bloco no meio do DEFAULT de auditoria", "created_at TIMESTAMP /* x */ DEFAULT /* y */ " + db08Relogio + ",", false},
		{"hora corrente SQL padrão entra no padrão", "SELECT CURRENT_" + "TIME", true},
		{"hora UTC entra no padrão", "SELECT UTC_" + "TIME()", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			violacoes, err := db08ViolacoesSQL(c.fonte)
			require.NoError(t, err)
			assert.Equal(t, c.violar, len(violacoes) > 0, c.fonte)
		})
	}
}

func TestTST24_NaoFechadoNoFimDoArquivoEErro(t *testing.T) {
	casos := []struct{ nome, fonte, linha string }{
		{"aspa simples", "SELECT 1;\nSELECT 'abc", "linha 2"},
		{"aspa dupla", `SELECT "abc`, "linha 1"},
		{"crase", "SELECT `abc", "linha 1"},
		{"barra no último byte", `SELECT 'abc\`, "linha 1"},
		{"aspa dobrada no fim", "SELECT 'abc''", "linha 1"},
		{"bloco", "SELECT 1;\n\n/* abc", "linha 3"},
		{"bloco condicional", "/*! SELECT 1;", "linha 1"},
		{"bloco condicional com comentário no fim", "/*! SELECT 1; -- x", "linha 1"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := db08ViolacoesSQL(c.fonte)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.linha)
		})
	}
}

func TestTST24_NumeroDaLinhaDepoisDeComentarioEString(t *testing.T) {
	casos := []struct {
		nome  string
		fonte string
		linha int
	}{
		{"bloco multilinha", "/* a\nb\nc */\nSELECT 1;\nSET @x = " + db08Curdate + ";", 5},
		{"bloco multilinha na mesma linha do relógio", "/* a\nb */ SET @x = " + db08Curdate + ";", 2},
		{"comentários de linha", "-- a\n# b\nSET @x = " + db08Curdate + ";", 3},
		{"relógio dentro de string multilinha", "SELECT 'a\nb\n" + db08Curdate + "';", 3},
		{"depois de string multilinha", "SELECT 'a\nb';\nSET @x = " + db08Curdate + ";", 3},
		{"CRLF", "/* a\r\nb */\r\nSET @x = " + db08Curdate + ";\r\n", 3},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			violacoes, err := db08ViolacoesSQL(c.fonte)
			require.NoError(t, err)
			require.Len(t, violacoes, 1)
			assert.Contains(t, violacoes, c.linha)
		})
	}
}

func TestTST25_AnalisadorDeEmbutido(t *testing.T) {
	heredoc := "mariadb <<'SQL'\n"
	casos := []struct {
		nome   string
		fonte  string
		linhas []int
	}{
		{"heredoc com DEFAULT em outra coluna", heredoc + "  expira TIMESTAMP DEFAULT " + db08Relogio + ",\nSQL\n", []int{2}},
		{"heredoc com DEFAULT em created_at", heredoc + "  created_at TIMESTAMP DEFAULT " + db08Relogio + ",\nSQL\n", nil},
		{"linha de comentário", "# " + db08Agora + "\n", nil},
		{"linha de comentário indentada", "\t  # " + db08Agora + "\n", nil},
		{"# no meio da linha não é comentário", "echo oi # " + db08Agora + "\n", []int{1}},
		{"-- não é comentário fora do SQL", "-- " + db08Agora + "\n", []int{1}},
		{"bloco /* não é comentário fora do SQL", "/* " + db08Agora + " */\n", []int{1}},
		{"aspa solta não dá erro fora do SQL", "echo it's\n", nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			violacoes, err := db08ViolacoesEmbutido(c.fonte)
			require.NoError(t, err)
			var linhas []int
			for linha := range violacoes {
				linhas = append(linhas, linha)
			}
			sort.Ints(linhas)
			assert.Equal(t, c.linhas, linhas)
		})
	}
}

func TestTST25_VarreduraDeArquivoSintetico(t *testing.T) {
	raiz := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(raiz, "deploy"), 0o700))
	escrever := func(rel, conteudo string) {
		require.NoError(t, os.WriteFile(filepath.Join(raiz, filepath.FromSlash(rel)), []byte(conteudo), 0o600))
	}
	escrever("deploy/ruim.sh", "#!/bin/bash\nmariadb <<'SQL'\nCREATE TABLE t (\n  expira TIMESTAMP DEFAULT "+db08Relogio+"\n);\nSQL\n")
	escrever("deploy/bom.sh", "#!/bin/bash\n# "+db08Agora+"\nmariadb <<'SQL'\nCREATE TABLE t (\n  created_at TIMESTAMP DEFAULT "+db08Relogio+"\n);\nSQL\n")
	escrever("ruim.sql", "SELECT 1;\nSELECT 'abc")

	arquivos := db08Listar(t, raiz, "deploy/*.sh")
	require.Equal(t, []string{"deploy/bom.sh", "deploy/ruim.sh"}, arquivos)

	achados, err := db08Varrer(raiz, arquivos, db08ViolacoesEmbutido)
	require.NoError(t, err)
	require.Len(t, achados, 1)
	assert.Contains(t, achados, "deploy/ruim.sh:4")

	_, err = db08Varrer(raiz, []string{"ruim.sql"}, db08ViolacoesSQL)
	require.Error(t, err, "string não fechada faz a varredura falhar")
	assert.Contains(t, err.Error(), "ruim.sql")

	_, err = db08Varrer(raiz, []string{"Makefile"}, db08ViolacoesEmbutido)
	assert.ErrorIs(t, err, os.ErrNotExist, "arquivo da lista ausente faz a varredura falhar")
}

func TestTST25_AllowlistSemSobra(t *testing.T) {
	achados := map[string]string{"a.sh:1": "x", "a.sh:2": "y"}
	allowlist := map[string]string{"a.sh:1": "motivo", "a.sh:9": "motivo velho", "a.sh:2": " "}

	violacoes, sobras, semMotivo := db08Confrontar(achados, allowlist)

	assert.Empty(t, violacoes)
	assert.Equal(t, []string{"a.sh:9"}, sobras)
	assert.Equal(t, []string{"a.sh:2"}, semMotivo)

	violacoes, _, _ = db08Confrontar(achados, map[string]string{"a.sh:1": "motivo"})
	assert.Equal(t, []string{"a.sh:2: y"}, violacoes)
}

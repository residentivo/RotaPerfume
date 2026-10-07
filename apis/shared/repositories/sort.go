// Package repositories contém funções puras de acesso a dados (stateless).
// sort.go concentra o helper compartilhado de ordenação dinâmica (order_by /
// order_dir), reutilizado por todos os repositórios com listagem paginada.
package repositories

import (
	"strings"

	"github.com/rotaperfumes/shared/vlog"
)

// resolveOrderDir normaliza orderDir para "ASC" ou "DESC" (case-insensitive).
// Retorna defaultDir (também normalizado) se orderDir for vazio ou inválido.
func resolveOrderDir(orderDir, defaultDir string) string {
	vlog.Printf("sort.go", "resolveOrderDir", "definindo dir com resultado de chamada a strings.ToUpper")
	dir := strings.ToUpper(strings.TrimSpace(orderDir))
	vlog.Printf("sort.go", "resolveOrderDir", "verificando se dir == \"ASC\" || dir == \"DESC\"")
	if dir == "ASC" || dir == "DESC" {
		return dir
	}
	return strings.ToUpper(strings.TrimSpace(defaultDir))
}

// resolveOrderColumn resolve orderBy (campo aceito pela API) contra uma
// whitelist de colunas SQL reais. Nomes de coluna não podem ser
// parametrizados com placeholders `?` em SQL, então qualquer valor fora da
// whitelist (incluindo vazio) cai em defaultCol — isso evita SQL injection
// via order_by.
func resolveOrderColumn(whitelist map[string]string, orderBy, defaultCol string) string {
	vlog.Printf("sort.go", "resolveOrderColumn", "definindo col, ok = whitelist[strings.ToLower(strings.TrimSpace(orderBy))] e verificando se ok")
	if col, ok := whitelist[strings.ToLower(strings.TrimSpace(orderBy))]; ok {
		return col
	}
	return defaultCol
}

// buildOrderByClause monta a cláusula " ORDER BY <coluna> <dir>" completa a
// partir de orderBy/orderDir informados pelo cliente, validados contra a
// whitelist e o default de cada entidade.
func buildOrderByClause(whitelist map[string]string, orderBy, orderDir, defaultCol, defaultDir string) string {
	vlog.Printf("sort.go", "buildOrderByClause", "definindo col com resultado de chamada a resolveOrderColumn")
	col := resolveOrderColumn(whitelist, orderBy, defaultCol)
	vlog.Printf("sort.go", "buildOrderByClause", "definindo dir com resultado de chamada a resolveOrderDir")
	dir := resolveOrderDir(orderDir, defaultDir)
	return " ORDER BY " + col + " " + dir
}

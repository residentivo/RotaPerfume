// Package repositories contém funções puras de acesso a dados (stateless).
// sort.go concentra o helper compartilhado de ordenação dinâmica (order_by /
// order_dir), reutilizado por todos os repositórios com listagem paginada.
package repositories

import "strings"

// resolveOrderDir normaliza orderDir para "ASC" ou "DESC" (case-insensitive).
// Retorna defaultDir (também normalizado) se orderDir for vazio ou inválido.
func resolveOrderDir(orderDir, defaultDir string) string {
	dir := strings.ToUpper(strings.TrimSpace(orderDir))
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
	if col, ok := whitelist[strings.ToLower(strings.TrimSpace(orderBy))]; ok {
		return col
	}
	return defaultCol
}

// buildOrderByClause monta a cláusula " ORDER BY <coluna> <dir>" completa a
// partir de orderBy/orderDir informados pelo cliente, validados contra a
// whitelist e o default de cada entidade.
func buildOrderByClause(whitelist map[string]string, orderBy, orderDir, defaultCol, defaultDir string) string {
	col := resolveOrderColumn(whitelist, orderBy, defaultCol)
	dir := resolveOrderDir(orderDir, defaultDir)
	return " ORDER BY " + col + " " + dir
}

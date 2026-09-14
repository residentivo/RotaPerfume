package repositories

import "testing"

// Testes internos (package repositories, não repositories_test) porque
// resolveOrderColumn/resolveOrderDir/buildOrderByClause não são exportados.

func TestResolveOrderDir(t *testing.T) {
	testes := []struct {
		nome       string
		orderDir   string
		defaultDir string
		want       string
	}{
		{"asc minúsculo", "asc", "ASC", "ASC"},
		{"desc minúsculo", "desc", "ASC", "DESC"},
		{"ASC maiúsculo", "ASC", "DESC", "ASC"},
		{"DESC maiúsculo", "DESC", "ASC", "DESC"},
		{"case misto (AsC)", "AsC", "DESC", "ASC"},
		{"com espaços em volta", "  desc  ", "ASC", "DESC"},
		{"vazio cai no default", "", "DESC", "DESC"},
		{"valor inválido cai no default", "sideways", "ASC", "ASC"},
		{"SQL injection cai no default", "asc; DROP TABLE x;--", "ASC", "ASC"},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			got := resolveOrderDir(tt.orderDir, tt.defaultDir)
			if got != tt.want {
				t.Errorf("resolveOrderDir(%q, %q) = %q, want %q", tt.orderDir, tt.defaultDir, got, tt.want)
			}
		})
	}
}

func TestResolveOrderColumn(t *testing.T) {
	whitelist := map[string]string{
		"id":           "id",
		"nome":         "nome_completo",
		"cliente_nome": "c.razao_social",
	}

	testes := []struct {
		nome       string
		orderBy    string
		defaultCol string
		want       string
	}{
		{"campo válido direto", "id", "id", "id"},
		{"campo válido mapeado para coluna diferente", "nome", "id", "nome_completo"},
		{"campo válido com alias de outra tabela", "cliente_nome", "id", "c.razao_social"},
		{"case-insensitive", "NOME", "id", "nome_completo"},
		{"com espaços em volta", "  nome  ", "id", "nome_completo"},
		{"fora da whitelist cai no default", "senha_hash", "id", "id"},
		{"vazio cai no default", "", "id", "id"},
		{"SQL injection cai no default", "1; DROP TABLE usuarios;--", "id", "id"},
		{"tentativa de subquery cai no default", "(SELECT 1)", "id", "id"},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			got := resolveOrderColumn(whitelist, tt.orderBy, tt.defaultCol)
			if got != tt.want {
				t.Errorf("resolveOrderColumn(%q) = %q, want %q", tt.orderBy, got, tt.want)
			}
		})
	}
}

func TestBuildOrderByClause(t *testing.T) {
	whitelist := map[string]string{
		"id":   "id",
		"nome": "nome",
	}

	testes := []struct {
		nome       string
		orderBy    string
		orderDir   string
		defaultCol string
		defaultDir string
		want       string
	}{
		{"válido asc", "nome", "asc", "id", "ASC", " ORDER BY nome ASC"},
		{"válido desc", "nome", "desc", "id", "ASC", " ORDER BY nome DESC"},
		{"tudo vazio usa defaults", "", "", "id", "DESC", " ORDER BY id DESC"},
		{"order_by inválido, order_dir válido", "coluna_inexistente", "desc", "id", "ASC", " ORDER BY id DESC"},
		{"order_by válido, order_dir inválido", "nome", "sideways", "id", "ASC", " ORDER BY nome ASC"},
		{"injection em order_by e order_dir", "1); DROP TABLE x;--", "asc; DROP TABLE x;--", "id", "ASC", " ORDER BY id ASC"},
	}

	for _, tt := range testes {
		t.Run(tt.nome, func(t *testing.T) {
			got := buildOrderByClause(whitelist, tt.orderBy, tt.orderDir, tt.defaultCol, tt.defaultDir)
			if got != tt.want {
				t.Errorf("buildOrderByClause(...) = %q, want %q", got, tt.want)
			}
		})
	}
}

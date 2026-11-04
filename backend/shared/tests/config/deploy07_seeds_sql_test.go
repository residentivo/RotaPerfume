package config_test

// DEPLOY-07 (Lote 33, TestBrain): seeds de sql/ aplicados pelo
// deploy/seed-producao.sh.
//   - 02_seed_admin.sql grava deve_trocar_senha = 1 (INSERT e ON DUPLICATE);
//   - 03_seed_vendedores.sql: INSERT IGNORE (nunca REPLACE INTO/DELETE/
//     TRUNCATE/DROP), 42 vendedores e 42 usuários, usuário inativo se e só se
//     o vendedor tem data_desligamento (ids 1, 3, 9, 30, 37, 38), e o UPDATE
//     final só desativa;
//   - os dois mantêm o hash PLACEHOLDER (nunca um hash real versionado).

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lerSQL(t *testing.T, nome string) string {
	t.Helper()
	return lerTexto(t, filepath.Join(raizProjeto(t), "sql", nome))
}

// semComentariosSQL remove as linhas "-- ...".
func semComentariosSQL(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "--") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func TestDEPLOY07_SeedAdmin_DeveTrocarSenha(t *testing.T) {
	sql := semComentariosSQL(lerSQL(t, "02_seed_admin.sql"))
	m := regexp.MustCompile("(?s)INSERT INTO `usuarios` \\((.*?)\\) VALUES \\((.*?)\\)\\s*ON DUPLICATE KEY UPDATE(.*?);").FindStringSubmatch(sql)
	require.NotNil(t, m, "INSERT ... ON DUPLICATE KEY UPDATE do admin")
	cols := strings.Split(m[1], ",")
	vals := strings.Split(m[2], ",")
	require.Len(t, vals, len(cols))
	pos := map[string]string{}
	for i, c := range cols {
		pos[strings.Trim(strings.TrimSpace(c), "`")] = strings.TrimSpace(vals[i])
	}
	assert.Equal(t, "1", pos["deve_trocar_senha"], "admin entra obrigado a trocar a senha")
	assert.Equal(t, "1", pos["ativo"])
	assert.Equal(t, "'admin'", pos["role"])
	assert.Equal(t, "'admin@rotaperfumes.com.br'", pos["email"])
	assert.Contains(t, pos["password_hash"], "PLACEHOLDER", "hash real nunca versionado")
	assert.Contains(t, m[3], "`deve_trocar_senha` = VALUES(`deve_trocar_senha`)", "reaplicar o seed volta a exigir a troca")
}

var (
	reVendedor = regexp.MustCompile(`(?m)^\((\d+), '[^']*', '[^']*', '[A-Z]{2}', '\d{4}-\d{2}-\d{2}', (NULL|'\d{4}-\d{2}-\d{2}'), [\d.]+\)[,;]$`)
	reUsuario  = regexp.MustCompile(`(?m)^\('[^']*', '([^']+)', @HASH_MUDAR_123, 'normal', (\d+), ([01]), ([01])\)[,;]$`)
)

func TestDEPLOY07_SeedVendedores_IdempotenteEDesligadosInativos(t *testing.T) {
	bruto := lerSQL(t, "03_seed_vendedores.sql")
	sql := semComentariosSQL(bruto)
	up := strings.ToUpper(sql)
	for _, proibido := range []string{"REPLACE INTO", "DELETE ", "TRUNCATE", "DROP ", "ALTER "} {
		assert.NotContains(t, up, proibido, "03 não pode destruir linhas: %s", proibido)
	}
	assert.Contains(t, sql, "INSERT IGNORE INTO `vendedores`")
	assert.Contains(t, sql, "INSERT IGNORE INTO `usuarios`")
	assert.Equal(t, 2, strings.Count(up, "INSERT "), "só os dois INSERT IGNORE")
	assert.Regexp(t, `SET @HASH_MUDAR_123 = '\$2a\$12\$XXXXPLACEHOLDER[A-Z0-9_]+XXXX';`, sql)

	desligado := map[int]bool{}
	for _, m := range reVendedor.FindAllStringSubmatch(sql, -1) {
		id, _ := strconv.Atoi(m[1])
		desligado[id] = m[2] != "NULL"
	}
	require.Len(t, desligado, 42, "42 vendedores")
	var ids []int
	for id, d := range desligado {
		if d {
			ids = append(ids, id)
		}
	}
	assert.ElementsMatch(t, []int{1, 3, 9, 30, 37, 38}, ids, "vendedores desligados")

	usuarios := reUsuario.FindAllStringSubmatch(sql, -1)
	require.Len(t, usuarios, 42, "um usuário por vendedor")
	emails := map[string]bool{}
	vistos := map[int]bool{}
	for _, u := range usuarios {
		email, ativo, troca := u[1], u[3], u[4]
		idV, _ := strconv.Atoi(u[2])
		t.Run(email, func(t *testing.T) {
			assert.False(t, emails[email], "e-mail repetido")
			assert.False(t, vistos[idV], "vendedor %d com dois usuários", idV)
			assert.Contains(t, desligado, idV, "vendedor %d existe no seed", idV)
			esperado := "1"
			if desligado[idV] {
				esperado = "0"
			}
			assert.Equal(t, esperado, ativo, "ativo = 0 se e só se o vendedor %d está desligado", idV)
			assert.Equal(t, "1", troca, "deve_trocar_senha = 1")
			assert.Regexp(t, `^[a-z]+\.[a-z]+(\.[a-z]{2})?@rotaperfumes\.com\.br$`, email)
		})
		emails[email], vistos[idV] = true, true
	}

	// UPDATE final: só desativa usuários normais de vendedores desligados.
	m := regexp.MustCompile("(?s)UPDATE `usuarios` u\\s+JOIN `vendedores` v ON v\\.id = u\\.id_vendedor\\s+SET (.*?)\\s+WHERE (.*?);").FindStringSubmatch(sql)
	require.NotNil(t, m, "UPDATE idempotente dos desligados")
	assert.Equal(t, "u.ativo = 0", strings.TrimSpace(m[1]), "só desativa")
	for _, s := range []string{"v.data_desligamento IS NOT NULL", "u.role = 'normal'", "u.ativo <> 0"} {
		assert.Contains(t, m[2], s)
	}
	assert.Equal(t, 1, strings.Count(up, "UPDATE "), "um único UPDATE")
	assert.NotRegexp(t, `(?i)SET\s+[^;]*ativo\s*=\s*1`, sql, "nunca ativa ninguém")
}

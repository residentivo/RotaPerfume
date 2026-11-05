package deploy_test

// DB-07 (Lote 36): o sql_mode do MariaDB do deploy/mariadb-docker.sh liga o
// ONLY_FULL_GROUP_BY (antes desligado, DB-04/DB-06). Estático (valor exato
// do SQL_MODE e a linha do cnf) e ensaio (o zz-rotaperfumes.cnf gerado).

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sqlModeDB07 = "ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION"

func TestDB07_SqlModeComOnlyFullGroupBy(t *testing.T) {
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	m := regexp.MustCompile(`(?m)^readonly SQL_MODE="([^"]+)"$`).FindStringSubmatch(md)
	require.NotNil(t, m, "readonly SQL_MODE no mariadb-docker.sh")
	assert.Equal(t, sqlModeDB07, m[1])
	assert.Contains(t, strings.Split(m[1], ","), "ONLY_FULL_GROUP_BY")
	assert.Contains(t, md, "\nsql_mode = $SQL_MODE\n", "o cnf usa o SQL_MODE")
	assert.NotContains(t, md, "sem ONLY_FULL_GROUP_BY", "comentário antigo (DB-04/DB-06) atualizado")
}

func TestDB07_Ensaio_CnfComOnlyFullGroupBy(t *testing.T) {
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca")
	require.NoError(t, r.err, r.out)
	cnf := e.arq("opt/rotaperfumes/mariadb-imagem/zz-rotaperfumes.cnf")
	require.NotEmpty(t, cnf)
	assert.Contains(t, cnf, "\nsql_mode = "+sqlModeDB07+"\n")
}

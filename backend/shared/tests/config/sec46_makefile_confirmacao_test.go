package config_test

// SEC-46 (Lote 34): confirmação explícita nos alvos destrutivos do Makefile.
//   - db-guard-sem-producao: db-down/db-reset/db-rebuild/db-seed nunca rodam
//     na produção, nem com CONFIRMO_PRODUCAO (sem válvula);
//   - db-guard-drop: DROP DATABASE rotaperfumes fora do loopback (CasaOS
//     :3307, aliases fora da lista) exige CONFIRMAR=<DB_NAME> NA LINHA DE
//     COMANDO; loopback e outros nomes de banco passam calados.
// Tudo numa cópia do Makefile em t.TempDir(), sem .env, com um cliente
// mariadb falso que só grava o argv: nada conecta em banco.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSEC46_Makefile_GuardasSaoPreRequisitosDiretos(t *testing.T) {
	mk := lerMakefile(t)
	for _, g := range []string{"db-guard-sem-producao", "db-guard-drop"} {
		require.Contains(t, mk.regras, g)
		assert.True(t, mk.phony[g], "%s no .PHONY", g)
	}
	for _, a := range []string{"db-down", "db-seed", "db-reset", "db-rebuild"} {
		assert.Contains(t, mk.regras[a].prereqs, "db-guard-sem-producao", "%s depende direto da guarda sem produção", a)
	}
	for _, a := range []string{"db-down", "db-reset", "db-rebuild"} {
		prereqs := mk.regras[a].prereqs
		assert.Contains(t, prereqs, "db-guard-drop", "%s depende direto da guarda de DROP", a)
		assert.Equal(t, "db-guard-producao", prereqs[0], "%s: a guarda do DEPLOY-07 continua a primeira", a)
	}
	for _, a := range []string{"help", "test", "test-shared", "test-api", "lint", "db-up", "db-create", "db-check-env", "db-export"} {
		assert.False(t, mk.dependeDe(a, "db-guard-sem-producao"), "%s não usa a guarda sem produção", a)
		assert.False(t, mk.dependeDe(a, "db-guard-drop"), "%s não usa a guarda de DROP", a)
	}
	assert.False(t, mk.dependeDe("db-seed", "db-guard-drop"), "db-seed não apaga o banco")

	// A detecção de produção é uma só (PRODUCAO_DETECTA), usada pelas duas guardas.
	assert.Contains(t, mk.linhaDaVariavel(t, "PRODUCAO_DETECTA"), "$(PRODUCAO_HOSTS_LAN)")
	for _, g := range []string{"db-guard-producao", "db-guard-sem-producao"} {
		assert.Contains(t, strings.Join(mk.regras[g].receita, "\n"), "$(PRODUCAO_DETECTA)", g)
	}
	receita := strings.Join(mk.regras["db-guard-sem-producao"].receita, "\n")
	assert.NotContains(t, receita, "CONFIRMO_PRODUCAO)", "sem válvula: a receita não lê CONFIRMO_PRODUCAO")
	assert.NotContains(t, receita, "CONFIRMAR")
}

func TestSEC46_Makefile_GuardaSemProducao(t *testing.T) {
	casos := []struct {
		nome   string
		args   []string
		recusa bool
	}{
		{"producao confirmada", []string{"DB_HOST=192.168.168.106", "DB_PORT=3306", "DB_NAME=rotaperfumes", "CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes"}, true},
		{"producao sem porta", []string{"DB_HOST=192.168.168.106", "DB_PORT="}, true},
		{"rede docker", []string{"DB_HOST=rotaperfumes-mariadb", "DB_PORT=3307"}, true},
		{"producao com CONFIRMAR", []string{"DB_HOST=192.168.168.106", "DB_PORT=3306", "CONFIRMAR=rotaperfumes"}, true},
		{"casaos 3307", []string{"DB_HOST=192.168.168.106", "DB_PORT=3307"}, false},
		{"loopback", []string{"DB_HOST=127.0.0.1", "DB_PORT=3306"}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-sem-producao", nil, c.args...)
			if c.recusa {
				require.Error(t, err, out)
				assert.Contains(t, out, "SEC-46:")
				assert.Contains(t, out, "mesmo com CONFIRMO_PRODUCAO")
				assert.Contains(t, out, "deploy/mariadb-docker.sh --recriar")
				assert.Contains(t, out, "deploy/seed-producao.sh")
			} else {
				assert.NoError(t, err, out)
				assert.Empty(t, strings.TrimSpace(out))
			}
		})
	}
}

func TestSEC46_Makefile_GuardaDrop(t *testing.T) {
	casaos := []string{"DB_HOST=192.168.168.106", "DB_PORT=3307"}
	casos := []struct {
		nome   string
		env    []string
		args   []string
		recusa bool
		aviso  bool
	}{
		{nome: "casaos sem CONFIRMAR", args: append([]string{"DB_NAME=rotaperfumes"}, casaos...), recusa: true},
		{nome: "casaos com CONFIRMAR", args: append([]string{"DB_NAME=rotaperfumes", "CONFIRMAR=rotaperfumes"}, casaos...), aviso: true},
		{nome: "CONFIRMAR só no ambiente", env: []string{"CONFIRMAR=rotaperfumes"}, args: append([]string{"DB_NAME=rotaperfumes"}, casaos...), recusa: true},
		{nome: "CONFIRMAR errado", args: append([]string{"DB_NAME=rotaperfumes", "CONFIRMAR=outro"}, casaos...), recusa: true},
		{nome: "ROTAPERFUMES maiúsculo exige", args: append([]string{"DB_NAME=ROTAPERFUMES", "CONFIRMAR=rotaperfumes"}, casaos...), recusa: true},
		{nome: "ROTAPERFUMES confirmado", args: append([]string{"DB_NAME=ROTAPERFUMES", "CONFIRMAR=ROTAPERFUMES"}, casaos...), aviso: true},
		{nome: "DB_NAME padrão", args: casaos, recusa: true},
		{nome: "alias fora da lista", args: []string{"DB_HOST=prod.exemplo.lan", "DB_PORT=3306"}, recusa: true},
		{nome: "banco descartável", args: append([]string{"DB_NAME=rotaperfumes_teste"}, casaos...)},
		{nome: "localhost", args: []string{"DB_HOST=localhost", "DB_PORT=3306", "DB_NAME=rotaperfumes"}},
		{nome: "127.0.0.1", args: []string{"DB_HOST=127.0.0.1", "DB_PORT=3306", "DB_NAME=rotaperfumes"}},
		{nome: "::1", args: []string{"DB_HOST=::1", "DB_NAME=rotaperfumes"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, err := makeAlvo(t, copiaDoMakefile(t), "db-guard-drop", c.env, c.args...)
			switch {
			case c.recusa:
				require.Error(t, err, out)
				assert.Contains(t, out, "SEC-46: DROP DATABASE")
				assert.Contains(t, out, "(fora do loopback): repita com CONFIRMAR=")
			case c.aviso:
				assert.NoError(t, err, out)
				assert.Contains(t, out, "AVISO (SEC-46): DROP DATABASE")
				assert.Contains(t, out, "confirmado por CONFIRMAR=")
			default:
				assert.NoError(t, err, out)
				assert.Empty(t, strings.TrimSpace(out))
			}
		})
	}
}

// sec46ClienteFalso cria um cliente "mariadb" que responde ao --version e
// grava o argv (uma linha por chamada) em argv.log; nunca conecta.
func sec46ClienteFalso(t *testing.T, dir string) (cliente, log string) {
	t.Helper()
	cliente = filepath.Join(dir, "mariadb-falso")
	log = filepath.Join(dir, "argv.log")
	script := "#!/bin/sh\n[ \"$1\" = --version ] && { echo 'mariadb from 11.4.5-MariaDB, client 15.2'; exit 0; }\necho \"$*\" >> '" + log + "'\n"
	require.NoError(t, os.WriteFile(cliente, []byte(script), 0o755))
	return cliente, log
}

func lerSeExiste(t *testing.T, caminho string) string {
	t.Helper()
	b, err := os.ReadFile(caminho)
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	return string(b)
}

func TestSEC46_Makefile_DbDownPontaAPonta(t *testing.T) {
	base := []string{"DB_HOST=192.168.168.106", "DB_PORT=3307", "DB_NAME=rotaperfumes",
		"DB_TLS_REQUIRED=false", "DB_TLS_CA=", "DB_USUARIO=u-sec46", "DB_SENHA=senha-sec46"}

	t.Run("sem CONFIRMAR não chama o DROP", func(t *testing.T) {
		dir := copiaDoMakefile(t)
		cliente, log := sec46ClienteFalso(t, dir)
		out, err := makeAlvo(t, dir, "db-down", nil, append(base, "DB_CLIENTE_BIN="+cliente)...)
		require.Error(t, err, out)
		assert.Contains(t, out, "SEC-46: DROP DATABASE rotaperfumes em 192.168.168.106:3307")
		assert.NotContains(t, lerSeExiste(t, log), "DROP", "o cliente não pode ter sido chamado")
		assert.NotContains(t, out, "senha-sec46")
	})
	t.Run("com CONFIRMAR chama o DROP", func(t *testing.T) {
		dir := copiaDoMakefile(t)
		cliente, log := sec46ClienteFalso(t, dir)
		out, err := makeAlvo(t, dir, "db-down", nil, append(base, "DB_CLIENTE_BIN="+cliente, "CONFIRMAR=rotaperfumes")...)
		require.NoError(t, err, out)
		assert.Contains(t, out, "AVISO (SEC-46)")
		argv := lerSeExiste(t, log)
		assert.Contains(t, argv, "DROP DATABASE IF EXISTS rotaperfumes;")
		assert.NotContains(t, argv, "senha-sec46", "senha só por MYSQL_PWD, nunca no argv")
	})
	t.Run("loopback sem CONFIRMAR", func(t *testing.T) {
		dir := copiaDoMakefile(t)
		cliente, log := sec46ClienteFalso(t, dir)
		args := []string{"DB_HOST=127.0.0.1", "DB_PORT=3306", "DB_NAME=rotaperfumes", "DB_TLS_CA=", "DB_USUARIO=u", "DB_SENHA=s", "DB_CLIENTE_BIN=" + cliente}
		out, err := makeAlvo(t, dir, "db-down", nil, args...)
		require.NoError(t, err, out)
		assert.NotContains(t, out, "SEC-46")
		assert.Contains(t, lerSeExiste(t, log), "DROP DATABASE IF EXISTS rotaperfumes;")
	})
	t.Run("producao confirmada continua recusada", func(t *testing.T) {
		dir := copiaDoMakefile(t)
		cliente, log := sec46ClienteFalso(t, dir)
		args := []string{"DB_HOST=192.168.168.106", "DB_PORT=3306", "DB_NAME=rotaperfumes", "DB_TLS_REQUIRED=false", "DB_TLS_CA=",
			"DB_USUARIO=u", "DB_SENHA=s", "DB_CLIENTE_BIN=" + cliente,
			"CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes", "CONFIRMAR=rotaperfumes"}
		out, err := makeAlvo(t, dir, "db-down", nil, args...)
		require.Error(t, err, out)
		assert.Contains(t, out, "mesmo com CONFIRMO_PRODUCAO")
		assert.Empty(t, lerSeExiste(t, log))
	})
}

// db-reset/db-rebuild/db-seed: só a recusa (o caminho feliz roda DDL, seed e
// go run, fora do alcance de um teste sem banco).
func TestSEC46_Makefile_ResetRebuildSeedRecusam(t *testing.T) {
	comum := []string{"DB_NAME=rotaperfumes", "DB_TLS_REQUIRED=false", "DB_TLS_CA=", "DB_USUARIO=u", "DB_SENHA=s"}
	casos := []struct {
		alvo, msg string
		args      []string
	}{
		{"db-reset", "(fora do loopback): repita com CONFIRMAR=rotaperfumes", []string{"DB_HOST=192.168.168.106", "DB_PORT=3307"}},
		{"db-rebuild", "(fora do loopback): repita com CONFIRMAR=rotaperfumes", []string{"DB_HOST=192.168.168.106", "DB_PORT=3307"}},
		{"db-reset", "mesmo com CONFIRMO_PRODUCAO", []string{"DB_HOST=192.168.168.106", "DB_PORT=3306", "CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes", "CONFIRMAR=rotaperfumes"}},
		{"db-rebuild", "mesmo com CONFIRMO_PRODUCAO", []string{"DB_HOST=rotaperfumes-mariadb", "DB_PORT=3306", "CONFIRMO_PRODUCAO=rotaperfumes-mariadb/rotaperfumes", "CONFIRMAR=rotaperfumes"}},
		{"db-seed", "mesmo com CONFIRMO_PRODUCAO", []string{"DB_HOST=192.168.168.106", "DB_PORT=3306", "CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes"}},
	}
	for _, c := range casos {
		t.Run(c.alvo+"/"+c.msg, func(t *testing.T) {
			dir := copiaDoMakefile(t)
			cliente, log := sec46ClienteFalso(t, dir)
			args := append(append(append([]string{}, comum...), c.args...), "DB_CLIENTE_BIN="+cliente)
			out, err := makeAlvo(t, dir, c.alvo, nil, args...)
			require.Error(t, err, out)
			assert.Contains(t, out, c.msg)
			assert.Empty(t, lerSeExiste(t, log), "nenhuma chamada ao cliente antes da recusa")
		})
	}
}

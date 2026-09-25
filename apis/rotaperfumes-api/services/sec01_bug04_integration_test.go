package services_test

// Integração (INTEGRATION=1, MySQL local) de BUG-04 (re-desligar preserva a
// data) e SEC-01 (Create de cliente por usuário normal grava cliente +
// carteira atomicamente). Usa apenas registros temporários ZZ-TEST-* e os
// apaga no t.Cleanup.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

func pularSemIntegracao(t *testing.T) {
	t.Helper()
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("teste de integração: defina INTEGRATION=1 (requer MySQL local)")
	}
}

// criarVendedorTemp cria um vendedor ZZ-TEST-<tag>-* e agenda o DELETE.
func criarVendedorTemp(t *testing.T, ctx context.Context, db *sql.DB, tag string) int64 {
	t.Helper()
	nome := fmt.Sprintf("ZZ-TEST-%s-%d", tag, time.Now().UnixNano())
	v, err := services.NewVendedorService(db, &config.Config{}).CreateVendedor(ctx, db, services.VendedorInput{
		Nome: nome, Regiao: "Teste", UF: "PR", DataAdmissao: "2024-01-01", MetaMensal: 1,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.Exec("DELETE FROM vendedores WHERE id = ? AND nome LIKE 'ZZ-TEST-%'", v.ID)
		assert.NoError(t, err)
	})
	return v.ID
}

func TestIntegracaoBUG04_RedesligarVendedorPreservaDataOriginal(t *testing.T) {
	pularSemIntegracao(t)
	db := abrirDBIntegracao(t)
	ctx := context.Background()
	id := criarVendedorTemp(t, ctx, db, "BUG04")

	_, err := db.Exec("UPDATE vendedores SET data_desligamento = '2020-05-05' WHERE id = ?", id)
	require.NoError(t, err)

	v, err := services.NewVendedorService(db, &config.Config{}).DeleteVendedor(ctx, db, id)
	require.NoError(t, err, "desligar de novo deve responder sucesso (200)")
	require.NotNil(t, v)

	var data string
	require.NoError(t, db.QueryRow(
		"SELECT DATE_FORMAT(data_desligamento, '%Y-%m-%d') FROM vendedores WHERE id = ?", id).Scan(&data))
	assert.Equal(t, "2020-05-05", data, "COALESCE deve preservar a data original")
}

func TestIntegracaoSEC01_CreateClienteNaCarteira(t *testing.T) {
	pularSemIntegracao(t)
	db := abrirDBIntegracao(t)
	ctx := context.Background()
	svc := services.NewClienteService(db, &config.Config{})
	sufixo := fmt.Sprintf("%d", time.Now().UnixNano())

	input := func(tag string) services.ClienteInput {
		return services.ClienteInput{
			CNPJ: "ZZ" + sufixo[len(sufixo)-12:], RazaoSocial: "ZZ-TEST-SEC01-" + tag + "-" + sufixo,
			Segmento: "Teste", Cidade: "Curitiba", UF: "PR",
		}
	}
	apagarCliente := func(id int64) {
		// carteiras tem ON DELETE CASCADE a partir de clientes.
		_, err := db.Exec("DELETE FROM clientes WHERE cliente_id_origem = ? AND razao_social LIKE 'ZZ-TEST-SEC01-%'", id)
		assert.NoError(t, err)
	}

	t.Run("sucesso grava cliente e carteira ativa de hoje", func(t *testing.T) {
		vendID := criarVendedorTemp(t, ctx, db, "SEC01")
		c, err := svc.CreateClienteNaCarteira(ctx, db, input("OK"), vendID)
		require.NoError(t, err)
		t.Cleanup(func() { apagarCliente(c.ClienteIDOrigem) })

		var vend int64
		var inicio string
		var fim sql.NullString
		require.NoError(t, db.QueryRow(
			"SELECT vendedor_id, DATE_FORMAT(data_inicio, '%Y-%m-%d'), data_fim FROM carteiras WHERE cliente_id = ?",
			c.ClienteIDOrigem).Scan(&vend, &inicio, &fim))
		assert.Equal(t, vendID, vend)
		assert.Equal(t, time.Now().Format("2006-01-02"), inicio)
		assert.False(t, fim.Valid, "data_fim deve ser NULL")
	})

	t.Run("falha na carteira faz rollback do cliente", func(t *testing.T) {
		in := input("ROLLBACK")
		_, err := svc.CreateClienteNaCarteira(ctx, db, in, -1) // FK de vendedor inválida
		require.Error(t, err)

		var n int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM clientes WHERE razao_social = ?", in.RazaoSocial).Scan(&n))
		assert.Zero(t, n, "cliente não pode ficar gravado sem carteira")
	})
}

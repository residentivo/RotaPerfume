package services_test

// Teste de INTEGRAÇÃO do BUG-01 contra o MySQL local (DSN loc=Local).
//
// Só roda com INTEGRATION=1 (ex.: `make test-integration`, ou
// `INTEGRATION=1 go test ./services/ -run TestIntegracaoBUG01 -v`).
// Credenciais via DB_HOST/DB_PORT/DB_NAME/DB_USUARIO/DB_SENHA (defaults do
// .env local: localhost:3306/rotaperfumes, golang/golang).
//
// Fluxo (por fuso de time.Local): cria um vendedor e um produto TEMPORÁRIOS
// (nome/sku com prefixo ZZ-TEST-BUG01-), confere no banco (DATE_FORMAT) que
// a data gravada é exatamente a informada, lê de volta pela aplicação,
// serializa em JSON como a API faz, re-salva a data lida (simulando a tela
// de edição que devolve os 10 primeiros caracteres) por 3 ciclos e confere
// que não há deslocamento acumulado. Os registros são apagados no
// t.Cleanup (DELETE por id) — nada além deles é tocado.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
	shareddb "github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/repositories"
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// abrirDBIntegracao abre o MySQL local já com time.Local definido (o driver
// resolve loc=Local no parse do DSN).
func abrirDBIntegracao(t *testing.T) *sql.DB {
	t.Helper()
	cfg := &config.Config{
		DBHost:    envOr("DB_HOST", "localhost"),
		DBPort:    envOr("DB_PORT", "3306"),
		DBName:    envOr("DB_NAME", "rotaperfumes"),
		DBUsuario: envOr("DB_USUARIO", "golang"),
		DBSenha:   envOr("DB_SENHA", "golang"),
	}
	db, err := shareddb.Open(cfg.DSN())
	require.NoError(t, err, "MySQL local indisponível")
	t.Cleanup(func() { db.Close() })
	return db
}

// dataNoBanco lê a coluna DATE como texto direto no MySQL (sem conversão de
// fuso do driver) — a "verdade" persistida.
func dataNoBanco(t *testing.T, db *sql.DB, tabela, coluna string, id int64) string {
	t.Helper()
	var s sql.NullString
	q := fmt.Sprintf("SELECT DATE_FORMAT(%s, '%%Y-%%m-%%d') FROM %s WHERE id = ?", coluna, tabela)
	require.NoError(t, db.QueryRow(q, id).Scan(&s))
	require.True(t, s.Valid)
	return s.String
}

// dataComoAPI serializa o time.Time como a API faz (JSON RFC3339) e extrai
// os 10 primeiros caracteres, como o frontend faz ao preencher o <input
// type="date"> da tela de edição.
func dataComoAPI(t *testing.T, v time.Time) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)[1:11]
}

func TestIntegracaoBUG01_GravaLeReSalvaSemDeslocamento(t *testing.T) {
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("teste de integração: defina INTEGRATION=1 (requer MySQL local)")
	}

	fusos := []*time.Location{
		time.Local, // fuso real da máquina
		time.FixedZone("UTC-3", -3*3600),
		time.FixedZone("UTC+9", 9*3600),
		time.FixedZone("UTC-12", -12*3600),
	}
	datas := []string{"2024-01-01", "2023-03-01", "2024-02-29"}

	for _, loc := range fusos {
		for _, data := range datas {
			t.Run(loc.String()+"/"+data, func(t *testing.T) {
				old := time.Local
				time.Local = loc
				t.Cleanup(func() { time.Local = old })

				db := abrirDBIntegracao(t)
				ctx := context.Background()
				cfg := &config.Config{}
				sufixo := fmt.Sprintf("%d", time.Now().UnixNano())

				t.Run("vendedor.data_admissao", func(t *testing.T) {
					svc := services.NewVendedorService(db, cfg)
					in := services.VendedorInput{Nome: "ZZ-TEST-BUG01-" + sufixo, Regiao: "Teste", UF: "PR", DataAdmissao: data}
					v, err := svc.CreateVendedor(ctx, db, in)
					require.NoError(t, err)
					require.NotZero(t, v.ID)
					t.Cleanup(func() {
						_, err := db.Exec("DELETE FROM vendedores WHERE id = ? AND nome LIKE 'ZZ-TEST-BUG01-%'", v.ID)
						assert.NoError(t, err)
					})

					assert.Equal(t, data, dataNoBanco(t, db, "vendedores", "data_admissao", v.ID), "gravação")

					for ciclo := 1; ciclo <= 3; ciclo++ {
						lido, err := repositories.NewVendedorRepository().GetByID(ctx, db, v.ID)
						require.NoError(t, err)
						assert.Equal(t, data, lido.DataAdmissao.In(time.Local).Format("2006-01-02"), "leitura ciclo %d", ciclo)
						assert.Equal(t, data, dataComoAPI(t, lido.DataAdmissao), "JSON ciclo %d", ciclo)

						in.DataAdmissao = dataComoAPI(t, lido.DataAdmissao)
						// Varia meta_mensal a cada ciclo para que o re-save
						// altere a linha de fato (o caso "sem alteração" é
						// coberto por TestIntegracao_UpdateSemAlteracao).
						in.MetaMensal = float64(ciclo)
						_, err = svc.UpdateVendedor(ctx, db, v.ID, in)
						require.NoError(t, err)
						assert.Equal(t, data, dataNoBanco(t, db, "vendedores", "data_admissao", v.ID), "re-save ciclo %d", ciclo)
					}
				})

				t.Run("produto.data_lancamento", func(t *testing.T) {
					svc := services.NewProdutoService(db, cfg)
					in := services.ProdutoInput{
						SKU: "ZZ-TEST-BUG01-" + sufixo, Descricao: "Produto teste BUG-01", Categoria: "Teste",
						Marca: "Teste", Unidade: "UN", PrecoTabela: 1, CustoUnitario: 1, DataLancamento: data,
					}
					p, err := svc.CreateProduto(ctx, db, in)
					require.NoError(t, err)
					require.NotZero(t, p.ID)
					t.Cleanup(func() {
						_, err := db.Exec("DELETE FROM produtos WHERE id = ? AND sku LIKE 'ZZ-TEST-BUG01-%'", p.ID)
						assert.NoError(t, err)
					})

					assert.Equal(t, data, dataNoBanco(t, db, "produtos", "data_lancamento", p.ID), "gravação")

					for ciclo := 1; ciclo <= 3; ciclo++ {
						lido, err := svc.GetProdutoByID(ctx, db, p.ID)
						require.NoError(t, err)
						require.NotNil(t, lido.DataLancamento)
						assert.Equal(t, data, dataComoAPI(t, *lido.DataLancamento), "JSON ciclo %d", ciclo)

						in.DataLancamento = dataComoAPI(t, *lido.DataLancamento)
						in.PrecoTabela = float64(ciclo + 1) // idem: força linha alterada
						_, err = svc.UpdateProduto(ctx, db, p.ID, in)
						require.NoError(t, err)
						assert.Equal(t, data, dataNoBanco(t, db, "produtos", "data_lancamento", p.ID), "re-save ciclo %d", ciclo)
					}
				})
			})
		}
	}
}

// TestIntegracao_UpdateSemAlteracao verifica o cenário "abrir a tela de
// edição e salvar sem mudar nada" (o mesmo fluxo de re-save do BUG-01).
//
// BUG-04 (corrigido): sem clientFoundRows=true no DSN o MySQL devolvia
// RowsAffected=0 para um UPDATE que não altera nenhuma coluna, e os
// repositórios traduziam n == 0 em ErrNotFound → 404 para um registro que
// existe. Com a flag, RowsAffected conta linhas encontradas e o re-save sem
// alteração responde sucesso.
func TestIntegracao_UpdateSemAlteracao(t *testing.T) {
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("teste de integração: defina INTEGRATION=1 (requer MySQL local)")
	}
	db := abrirDBIntegracao(t)
	ctx := context.Background()
	cfg := &config.Config{}
	sufixo := fmt.Sprintf("%d", time.Now().UnixNano())

	t.Run("vendedor", func(t *testing.T) {
		svc := services.NewVendedorService(db, cfg)
		in := services.VendedorInput{Nome: "ZZ-TEST-BUG01-" + sufixo, Regiao: "Teste", UF: "PR", DataAdmissao: "2024-01-01", MetaMensal: 1}
		v, err := svc.CreateVendedor(ctx, db, in)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := db.Exec("DELETE FROM vendedores WHERE id = ? AND nome LIKE 'ZZ-TEST-BUG01-%'", v.ID)
			assert.NoError(t, err)
		})

		_, err = svc.UpdateVendedor(ctx, db, v.ID, in)
		assert.NoError(t, err)
	})

	t.Run("produto", func(t *testing.T) {
		svc := services.NewProdutoService(db, cfg)
		in := services.ProdutoInput{
			SKU: "ZZ-TEST-BUG01-" + sufixo, Descricao: "Produto teste", Categoria: "Teste",
			Marca: "Teste", Unidade: "UN", PrecoTabela: 1, CustoUnitario: 1, DataLancamento: "2024-01-01",
		}
		p, err := svc.CreateProduto(ctx, db, in)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := db.Exec("DELETE FROM produtos WHERE id = ? AND sku LIKE 'ZZ-TEST-BUG01-%'", p.ID)
			assert.NoError(t, err)
		})

		_, err = svc.UpdateProduto(ctx, db, p.ID, in)
		assert.NoError(t, err)
	})
}

// ---------------------------------------------------------------------------
// BUG-05: inativar vendedor inativa os usuários vinculados.
//
// Cria 3 vendedores TEMPORÁRIOS (nome ZZ-TEST-BUG05-*):
//   - A: com 2 usuários vinculados (alvo do DeleteVendedor);
//   - B: com 1 usuário de controle (não pode ser afetado);
//   - C: sem usuários (DeleteVendedor deve funcionar mesmo assim).
// Mais 1 usuário de controle sem vendedor (id_vendedor NULL).
// Usuários usam e-mail zz-test-bug05-<sufixo>-*@teste.local. O t.Cleanup
// apaga os usuários ANTES dos vendedores (Cleanup é LIFO) e só por id +
// prefixo — nenhum dado pré-existente é tocado.
// ---------------------------------------------------------------------------

const bug05Prefixo = "ZZ-TEST-BUG05-"

func bug05CriarVendedor(t *testing.T, ctx context.Context, db *sql.DB, svc *services.VendedorService, nome string) int64 {
	t.Helper()
	v, err := svc.CreateVendedor(ctx, db, services.VendedorInput{
		Nome: nome, Regiao: "Teste", UF: "PR", DataAdmissao: "2024-01-01", MetaMensal: 1,
	})
	require.NoError(t, err)
	require.NotZero(t, v.ID)
	t.Cleanup(func() {
		_, err := db.Exec("DELETE FROM vendedores WHERE id = ? AND nome LIKE 'ZZ-TEST-BUG05-%'", v.ID)
		assert.NoError(t, err)
	})
	return v.ID
}

// bug05CriarUsuario insere direto no banco (sem service de usuário, para não
// gerar senha_historico/refresh_tokens). vendedorID <= 0 grava NULL.
func bug05CriarUsuario(t *testing.T, db *sql.DB, email string, vendedorID int64) int64 {
	t.Helper()
	var idVend sql.NullInt64
	if vendedorID > 0 {
		idVend = sql.NullInt64{Int64: vendedorID, Valid: true}
	}
	res, err := db.Exec(
		`INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo) VALUES (?, ?, ?, 'normal', ?, 1)`,
		bug05Prefixo+"usuario", email, "$2a$12$zztestbug05hashnaousadoparaloginxxxxxxxxxxxxxxxxxxxxxx", idVend,
	)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.Exec("DELETE FROM usuarios WHERE id = ? AND email LIKE 'zz-test-bug05-%'", id)
		assert.NoError(t, err)
	})
	return id
}

func bug05Ativo(t *testing.T, db *sql.DB, usuarioID int64) int {
	t.Helper()
	var ativo int
	require.NoError(t, db.QueryRow("SELECT ativo FROM usuarios WHERE id = ?", usuarioID).Scan(&ativo))
	return ativo
}

func bug05DataDesligamento(t *testing.T, db *sql.DB, vendedorID int64) sql.NullString {
	t.Helper()
	var s sql.NullString
	require.NoError(t, db.QueryRow(
		"SELECT DATE_FORMAT(data_desligamento, '%Y-%m-%d') FROM vendedores WHERE id = ?", vendedorID).Scan(&s))
	return s
}

func TestIntegracaoBUG05_InativarVendedorInativaUsuarios(t *testing.T) {
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("teste de integração: defina INTEGRATION=1 (requer MySQL local)")
	}
	db := abrirDBIntegracao(t)
	ctx := context.Background()
	svc := services.NewVendedorService(db, &config.Config{})
	sufixo := fmt.Sprintf("%d", time.Now().UnixNano())
	email := func(n string) string { return fmt.Sprintf("zz-test-bug05-%s-%s@teste.local", sufixo, n) }

	// Vendedores primeiro → seus Cleanups rodam por último (LIFO).
	vendA := bug05CriarVendedor(t, ctx, db, svc, bug05Prefixo+sufixo+"-A")
	vendB := bug05CriarVendedor(t, ctx, db, svc, bug05Prefixo+sufixo+"-B")
	vendC := bug05CriarVendedor(t, ctx, db, svc, bug05Prefixo+sufixo+"-C")

	u1 := bug05CriarUsuario(t, db, email("a1"), vendA)
	u2 := bug05CriarUsuario(t, db, email("a2"), vendA)
	ctrlOutroVend := bug05CriarUsuario(t, db, email("b1"), vendB)
	ctrlSemVend := bug05CriarUsuario(t, db, email("nulo"), 0)

	hoje := time.Now().Format("2006-01-02")

	t.Run("DeleteVendedor inativa usuarios vinculados", func(t *testing.T) {
		v, err := svc.DeleteVendedor(ctx, db, vendA)
		require.NoError(t, err)
		require.NotNil(t, v)
		require.NotNil(t, v.DataDesligamento, "retorno do service deve trazer data_desligamento")

		dd := bug05DataDesligamento(t, db, vendA)
		assert.True(t, dd.Valid, "data_desligamento deve ser preenchida")
		assert.Equal(t, hoje, dd.String)

		for _, tc := range []struct {
			nome  string
			id    int64
			ativo int
		}{
			{"usuario vinculado 1", u1, 0},
			{"usuario vinculado 2", u2, 0},
			{"controle de outro vendedor", ctrlOutroVend, 1},
			{"controle sem vendedor", ctrlSemVend, 1},
		} {
			t.Run(tc.nome, func(t *testing.T) {
				assert.Equal(t, tc.ativo, bug05Ativo(t, db, tc.id))
			})
		}
		assert.False(t, bug05DataDesligamento(t, db, vendB).Valid, "vendedor B não pode ser afetado")
	})

	t.Run("ReativarVendedor limpa data e NAO reativa usuarios", func(t *testing.T) {
		v, err := svc.ReativarVendedor(ctx, db, vendA)
		require.NoError(t, err)
		require.NotNil(t, v)
		assert.Nil(t, v.DataDesligamento)

		assert.False(t, bug05DataDesligamento(t, db, vendA).Valid, "data_desligamento deve voltar a NULL")
		for _, id := range []int64{u1, u2} {
			assert.Equal(t, 0, bug05Ativo(t, db, id), "usuario %d deve continuar inativo", id)
		}
		assert.Equal(t, 1, bug05Ativo(t, db, ctrlOutroVend))
		assert.Equal(t, 1, bug05Ativo(t, db, ctrlSemVend))
	})

	t.Run("DeleteVendedor sem usuarios", func(t *testing.T) {
		v, err := svc.DeleteVendedor(ctx, db, vendC)
		require.NoError(t, err)
		require.NotNil(t, v)
		dd := bug05DataDesligamento(t, db, vendC)
		assert.True(t, dd.Valid)
		assert.Equal(t, hoje, dd.String)
	})

	t.Run("DeleteVendedor inexistente nao altera usuarios", func(t *testing.T) {
		_, err := svc.DeleteVendedor(ctx, db, -1)
		assert.ErrorIs(t, err, services.ErrVendedorNaoEncontrado)
		assert.Equal(t, 1, bug05Ativo(t, db, ctrlOutroVend))
		assert.Equal(t, 1, bug05Ativo(t, db, ctrlSemVend))
	})
}

package services_test

// Testes do BUG-01 (deslocamento de data) pela API pública dos services:
// toda data AAAA-MM-DD recebida pela API deve ser interpretada como
// meia-noite em time.Local (mesma localização do DSN loc=Local), nunca como
// meia-noite UTC. Com time.Parse (UTC) e time.Local = UTC-3, a data
// "2024-03-10" viraria 2024-03-09 21:00 local e seria gravada no banco com um
// dia a menos.
//
// Cada caso chama o Create*/List* público com sqlmock e observa a data que
// chega ao banco (argumento do INSERT) ou o modelo devolvido pelo service.
//
// Os testes trocam time.Local por fusos fixos (UTC-3, UTC+9, UTC-12, UTC+14)
// e pelo fuso real America/Sao_Paulo (via time/tzdata, para independer do
// banco de fusos do SO). NÃO usam t.Parallel: time.Local é estado global.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tz"
)

const (
	dlReExisteCliente  = `SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`
	dlReExisteVendedor = `SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`
	dlReExisteSKU      = `SELECT 1 FROM produtos WHERE sku = \? LIMIT 1`
	dlReExistePedido   = `SELECT 1 FROM pedidos WHERE pedido_id_origem = \? LIMIT 1`
)

var errDLParar = errors.New("parar após capturar o INSERT")

// dlComLocal troca time.Local durante o teste e restaura ao final.
func dlComLocal(t *testing.T, loc *time.Location) {
	t.Helper()
	old := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = old })
}

// dlFusos devolve os fusos usados na matriz de testes.
func dlFusos(t *testing.T) []*time.Location {
	t.Helper()
	sp, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	return []*time.Location{
		time.FixedZone("UTC-3", -3*3600),
		time.FixedZone("UTC+9", 9*3600),
		time.FixedZone("UTC-12", -12*3600),
		time.FixedZone("UTC+14", 14*3600),
		time.UTC,
		sp,
	}
}

// dlAssertMeiaNoiteLocal garante que got é exatamente a meia-noite do dia
// informado em time.Local (mesmo instante, mesma data de calendário e mesmo
// offset do fuso local).
func dlAssertMeiaNoiteLocal(t *testing.T, got time.Time, y int, m time.Month, d int) {
	t.Helper()
	want := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	assert.True(t, got.Equal(want), "instante: got %s, want %s", got, want)

	gy, gm, gd := got.Date()
	assert.Equal(t, y, gy)
	assert.Equal(t, m, gm)
	assert.Equal(t, d, gd)
	assert.Equal(t, 0, got.Hour())
	assert.Equal(t, 0, got.Minute())
	assert.Equal(t, 0, got.Second())
	assert.Equal(t, 0, got.Nanosecond())

	_, gotOff := got.Zone()
	_, wantOff := want.Zone()
	assert.Equal(t, wantOff, gotOff, "offset deve ser o de time.Local")
	assert.Equal(t, time.Local.String(), got.Location().String())
}

func dlNewDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func dlExisteRow() *sqlmock.Rows { return sqlmock.NewRows([]string{"1"}).AddRow(1) }

// dlCapturaTempo é um sqlmock.Argument que aceita qualquer time.Time e o
// guarda em *dst (argumento do INSERT que vai ao banco).
type dlCapturaTempo struct{ dst *time.Time }

func (c dlCapturaTempo) Match(v driver.Value) bool {
	tv, ok := v.(time.Time)
	if ok {
		*c.dst = tv
	}
	return ok
}

// dlCapturaTexto guarda o argumento string recebido em *dst.
type dlCapturaTexto struct{ dst *string }

func (c dlCapturaTexto) Match(v driver.Value) bool {
	s, ok := v.(string)
	if ok {
		*c.dst = s
	}
	return ok
}

// dlParserCaso descreve um ponto público da API que converte uma string
// AAAA-MM-DD em time.Time.
type dlParserCaso struct {
	nome  string
	parse func(t *testing.T, data string) time.Time
}

func dlPagamentoInput(venc, pag string) services.PagamentoInput {
	return services.PagamentoInput{
		PedidoID: 1, FormaPagamento: "Dinheiro", Parcelas: 1, Valor: 100, ValorLiquido: 100,
		DataVencimento: venc, DataPagamento: pag, StatusPagamento: "Pago",
	}
}

func dlCriarPagamento(t *testing.T, venc, pag string) *time.Time {
	t.Helper()
	db, mock := dlNewDB(t)
	mock.ExpectQuery(dlReExistePedido).WithArgs(int64(1)).WillReturnRows(dlExisteRow())
	mock.ExpectExec(`INSERT INTO pagamentos`).WillReturnResult(sqlmock.NewResult(5, 1))
	p, err := services.NewPagamentoService(db, &config.Config{}).CreatePagamento(context.Background(), db, dlPagamentoInput(venc, pag))
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	if pag != "" {
		require.NotNil(t, p.DataPagamento)
		return p.DataPagamento
	}
	return &p.DataVencimento
}

func dlCriarOportunidade(t *testing.T, abertura, fechamento string) (time.Time, *time.Time) {
	t.Helper()
	db, mock := dlNewDB(t)
	mock.ExpectQuery(dlReExisteCliente).WithArgs(int64(1)).WillReturnRows(dlExisteRow())
	mock.ExpectQuery(dlReExisteVendedor).WithArgs(int64(2)).WillReturnRows(dlExisteRow())
	mock.ExpectExec(`INSERT INTO oportunidades`).WillReturnResult(sqlmock.NewResult(9, 1))
	o, err := services.NewOportunidadeService(db, &config.Config{}).CreateOportunidade(context.Background(), db, services.OportunidadeInput{
		ClienteID: 1, VendedorID: 2, Origem: "Indicação", DataAbertura: abertura,
		Etapa: "Prospecção", ProbabilidadePct: 10, ValorEstimado: 100, DataFechamento: fechamento,
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	return o.DataAbertura, o.DataFechamento
}

func dlParsers() []dlParserCaso {
	cfg := &config.Config{}
	ctx := context.Background()
	return []dlParserCaso{
		{"cliente.data_cadastro", func(t *testing.T, data string) time.Time {
			db, mock := dlNewDB(t)
			mock.ExpectExec(`INSERT INTO clientes`).WillReturnResult(sqlmock.NewResult(3, 1))
			// Releitura falha (sem expectativa): o service devolve o objeto em memória.
			c, err := services.NewClienteService(db, cfg).CreateCliente(ctx, db, services.ClienteInput{
				CNPJ: "11222333000181", RazaoSocial: "Cliente", Segmento: "Varejo",
				Cidade: "Curitiba", UF: "pr", DataCadastro: data,
			})
			require.NoError(t, err)
			return c.DataCadastro
		}},
		{"estoque.data_snapshot", func(t *testing.T, data string) time.Time {
			db, mock := dlNewDB(t)
			mock.ExpectQuery(dlReExisteSKU).WithArgs("SKU-1").WillReturnRows(dlExisteRow())
			var gravado string
			mock.ExpectExec(`INSERT INTO estoque`).
				WithArgs(dlCapturaTexto{&gravado}, "SKU-1", 5, false).
				WillReturnResult(sqlmock.NewResult(4, 1))
			e, err := services.NewEstoqueService(db, cfg).CreateEstoque(ctx, db,
				services.EstoqueInput{SKU: "SKU-1", DataSnapshot: data, Saldo: 5})
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
			assert.Equal(t, strings.TrimSpace(data), gravado, "data gravada no banco")
			return e.DataSnapshot
		}},
		{"oportunidade.data_abertura", func(t *testing.T, data string) time.Time {
			got, _ := dlCriarOportunidade(t, data, "")
			return got
		}},
		{"oportunidade.data_fechamento", func(t *testing.T, data string) time.Time {
			_, got := dlCriarOportunidade(t, "2020-01-01", data)
			require.NotNil(t, got)
			return *got
		}},
		{"pagamento.data_vencimento", func(t *testing.T, data string) time.Time {
			return *dlCriarPagamento(t, data, "")
		}},
		{"pagamento.data_pagamento", func(t *testing.T, data string) time.Time {
			return *dlCriarPagamento(t, "2020-01-01", data)
		}},
		{"pedido.data_pedido", func(t *testing.T, data string) time.Time {
			db, mock := dlNewDB(t)
			var got time.Time
			mock.ExpectBegin()
			mock.ExpectExec(`INSERT INTO pedidos`).
				WithArgs(int64(1), int64(2), dlCapturaTempo{&got}, "App", "Faturado", sqlmock.AnyArg()).
				WillReturnError(errDLParar)
			mock.ExpectRollback()
			_, err := services.NewPedidoService(db, cfg).CreatePedido(ctx, db, services.PedidoInput{
				ClienteID: 1, VendedorID: 2, DataPedido: data, Canal: "App", Status: "Faturado",
				Itens: []services.ItemPedidoInput{{ProdutoID: 3, Quantidade: 1, PrecoPraticado: 10}},
			})
			require.ErrorIs(t, err, errDLParar)
			require.NoError(t, mock.ExpectationsWereMet())
			return got
		}},
		{"produto.data_lancamento", func(t *testing.T, data string) time.Time {
			db, mock := dlNewDB(t)
			mock.ExpectExec(`INSERT INTO produtos`).WillReturnResult(sqlmock.NewResult(6, 1))
			p, err := services.NewProdutoService(db, cfg).CreateProduto(ctx, db, services.ProdutoInput{
				SKU: "SKU-1", Descricao: "Perfume", Categoria: "EDP", Marca: "M", Unidade: "UN",
				DataLancamento: data,
			})
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
			require.NotNil(t, p.DataLancamento)
			return *p.DataLancamento
		}},
		{"vendedor.data_admissao", func(t *testing.T, data string) time.Time {
			db, mock := dlNewDB(t)
			mock.ExpectExec(`INSERT INTO vendedores`).WillReturnResult(sqlmock.NewResult(7, 1))
			v, err := services.NewVendedorService(db, cfg).CreateVendedor(ctx, db, services.VendedorInput{
				Nome: "Vendedor", Regiao: "Sul", UF: "pr", DataAdmissao: data,
			})
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
			return v.DataAdmissao
		}},
		{"visita.data_visita", func(t *testing.T, data string) time.Time {
			db, mock := dlNewDB(t)
			mock.ExpectQuery(dlReExisteCliente).WithArgs(int64(1)).WillReturnRows(dlExisteRow())
			mock.ExpectQuery(dlReExisteVendedor).WithArgs(int64(2)).WillReturnRows(dlExisteRow())
			mock.ExpectExec(`INSERT INTO visitas`).WillReturnResult(sqlmock.NewResult(8, 1))
			v, err := services.NewVisitaService(db, cfg).CreateVisita(ctx, db,
				services.VisitaInput{ClienteID: 1, VendedorID: 2, DataVisita: data, Resultado: "Pedido", DuracaoMin: 30})
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
			return v.DataVisita
		}},
	}
}

var dlDatas = []struct {
	s       string
	y       int
	m       time.Month
	d       int
	comment string
}{
	{"2024-03-10", 2024, time.March, 10, "data comum"},
	{"2024-01-01", 2024, time.January, 1, "virada de ano (UTC-x cairia em 2023-12-31)"},
	{"2024-02-29", 2024, time.February, 29, "ano bissexto"},
	{"2023-03-01", 2023, time.March, 1, "primeiro dia do mês (UTC-x cairia em fevereiro)"},
	{" 2022-12-31 ", 2022, time.December, 31, "espaços são aparados"},
}

// TestBUG01_DatasParseadasComoMeiaNoiteLocal é a matriz parser x fuso x data:
// cada service que converte AAAA-MM-DD deve devolver meia-noite em
// time.Local, preservando o dia de calendário informado.
func TestBUG01_DatasParseadasComoMeiaNoiteLocal(t *testing.T) {
	for _, loc := range dlFusos(t) {
		for _, p := range dlParsers() {
			for _, d := range dlDatas {
				t.Run(loc.String()+"/"+p.nome+"/"+d.s, func(t *testing.T) {
					dlComLocal(t, loc)
					got := p.parse(t, d.s)
					dlAssertMeiaNoiteLocal(t, got, d.y, d.m, d.d)
				})
			}
		}
	}
}

// TestBUG01_FiltroDataEstoque: os filtros data_de/data_ate do ListEstoque
// chegam ao banco com o mesmo dia de calendário informado, em qualquer fuso.
func TestBUG01_FiltroDataEstoque(t *testing.T) {
	for _, loc := range dlFusos(t) {
		for _, d := range dlDatas {
			for _, historico := range []bool{false, true} {
				nome := loc.String() + "/" + d.s
				if historico {
					nome += "/historico"
				}
				t.Run(nome, func(t *testing.T) {
					dlComLocal(t, loc)
					db, mock := dlNewDB(t)
					var de, ate string
					mock.ExpectQuery(`SELECT COUNT`).
						WithArgs(dlCapturaTexto{&de}, dlCapturaTexto{&ate}).
						WillReturnError(errDLParar)
					_, _, err := services.NewEstoqueService(db, &config.Config{}).ListEstoque(context.Background(), db, 1, 10,
						services.EstoqueFiltro{DataDe: d.s, DataAte: d.s, Historico: historico})
					require.ErrorIs(t, err, errDLParar)
					require.NoError(t, mock.ExpectationsWereMet())
					assert.Equal(t, strings.TrimSpace(d.s), de)
					assert.Equal(t, strings.TrimSpace(d.s), ate)
				})
			}
		}
	}
}

// TestBUG01_RegressaoParseUTC documenta o bug original: com time.Parse (UTC)
// e time.Local = UTC-3, a data local cai no dia anterior. Serve de controle
// de que a matriz acima realmente distingue o comportamento corrigido.
func TestBUG01_RegressaoParseUTC(t *testing.T) {
	dlComLocal(t, time.FixedZone("UTC-3", -3*3600))

	antigo, err := time.Parse("2006-01-02", "2024-03-10")
	require.NoError(t, err)
	assert.Equal(t, 9, antigo.In(time.Local).Day(), "comportamento antigo: dia anterior no fuso local")

	for _, p := range dlParsers() {
		t.Run(p.nome, func(t *testing.T) {
			got := p.parse(t, "2024-03-10")
			assert.Equal(t, 10, got.In(time.Local).Day())
			assert.False(t, got.Equal(antigo), "não pode coincidir com a meia-noite UTC")
		})
	}
}

// TestBUG01_DataInvalidaContinuaRejeitada garante que a troca para
// ParseInLocation não afrouxou a validação de formato — e que a data
// inválida é rejeitada antes de qualquer escrita no banco.
func TestBUG01_DataInvalidaContinuaRejeitada(t *testing.T) {
	invalidas := []string{"10/03/2024", "2024-13-01", "2024-02-30", "2024-3-10", "2024-03-10T00:00:00Z", "abc"}
	cfg := &config.Config{}
	ctx := context.Background()

	for _, s := range invalidas {
		t.Run(s, func(t *testing.T) {
			db, mock := dlNewDB(t)

			_, err := services.NewClienteService(db, cfg).CreateCliente(ctx, db, services.ClienteInput{
				CNPJ: "11222333000181", RazaoSocial: "C", Segmento: "S", Cidade: "C", UF: "PR", DataCadastro: s,
			})
			assert.ErrorIs(t, err, services.ErrDataCadastroInvalida)

			_, _, err = services.NewEstoqueService(db, cfg).ListEstoque(ctx, db, 1, 10, services.EstoqueFiltro{DataDe: s})
			assert.ErrorIs(t, err, services.ErrEstoqueDataInvalida)
			_, _, err = services.NewEstoqueService(db, cfg).ListEstoque(ctx, db, 1, 10, services.EstoqueFiltro{DataAte: s})
			assert.ErrorIs(t, err, services.ErrEstoqueDataInvalida)

			_, err = services.NewEstoqueService(db, cfg).CreateEstoque(ctx, db, services.EstoqueInput{SKU: "SKU-1", DataSnapshot: s})
			assert.ErrorIs(t, err, services.ErrEstoqueDataInvalida)

			_, err = services.NewPedidoService(db, cfg).CreatePedido(ctx, db, services.PedidoInput{ClienteID: 1, VendedorID: 1, DataPedido: s})
			assert.ErrorIs(t, err, services.ErrDataPedidoInvalida)

			_, err = services.NewProdutoService(db, cfg).CreateProduto(ctx, db, services.ProdutoInput{
				SKU: "S", Descricao: "D", Categoria: "C", Marca: "M", Unidade: "UN", DataLancamento: s,
			})
			assert.ErrorIs(t, err, services.ErrDataLancamentoInvalida)

			_, err = services.NewVendedorService(db, cfg).CreateVendedor(ctx, db, services.VendedorInput{Nome: "N", Regiao: "R", UF: "PR", DataAdmissao: s})
			assert.ErrorIs(t, err, services.ErrVendedorDataAdmissaoInvalida)

			assert.NoError(t, mock.ExpectationsWereMet(), "data inválida deve falhar antes de consultar o banco")

			// Pagamento confere o pedido antes de validar as datas.
			for _, in := range []services.PagamentoInput{dlPagamentoInput(s, ""), dlPagamentoInput("2024-01-01", s)} {
				dbp, mockp := dlNewDB(t)
				mockp.ExpectQuery(dlReExistePedido).WithArgs(int64(1)).WillReturnRows(dlExisteRow())
				_, err = services.NewPagamentoService(dbp, cfg).CreatePagamento(ctx, dbp, in)
				if in.DataPagamento == s {
					assert.ErrorIs(t, err, services.ErrDataPagamentoInvalida)
				} else {
					assert.ErrorIs(t, err, services.ErrDataVencimentoInvalida)
				}
				assert.NoError(t, mockp.ExpectationsWereMet(), "nenhum INSERT com data inválida")
			}
		})
	}
}

// TestBUG01_HorarioDeVeraoHistorico documenta o caso-limite de fusos com
// horário de verão iniciando à meia-noite (America/Sao_Paulo até 2019):
// 2018-11-04 00:00 não existe no fuso e o Go normaliza para
// 2018-11-03 23:00 -03 — o dia de calendário recua 1 dia.
//
// CORRIGIDO (RISCO-01): o processo não usa mais o fuso do SO. O pacote
// shared/tz (importado em branco por shared/config) fixa time.Local em
// -03:00 sem horário de verão, em qualquer SO/container. O teste valida que
// o time.Local vigente é o fuso fixo e que, nele, as datas de antigo início
// de DST são preservadas.
func TestBUG01_HorarioDeVeraoHistorico(t *testing.T) {
	require.Same(t, tz.Local, time.Local, "time.Local deve ser o fuso fixo de shared/tz")
	dlComLocal(t, tz.Local)

	for _, p := range dlParsers() {
		t.Run(p.nome, func(t *testing.T) {
			got := p.parse(t, "2018-11-04")
			y, m, d := got.In(time.Local).Date()
			assert.Equal(t, 2018, y)
			assert.Equal(t, time.November, m)
			assert.Equal(t, 4, d)
		})
	}
}

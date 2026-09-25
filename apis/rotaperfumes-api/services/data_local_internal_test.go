package services

// Testes do BUG-01 (deslocamento de data): toda data AAAA-MM-DD recebida pela
// API deve ser interpretada como meia-noite em time.Local (mesma localização
// do DSN loc=Local), nunca como meia-noite UTC. Com time.Parse (UTC) e
// time.Local = UTC-3, a data "2024-03-10" viraria 2024-03-09 21:00 local e
// seria gravada no banco com um dia a menos.
//
// Os testes trocam time.Local por fusos fixos (UTC-3, UTC+9, UTC-12, UTC+14)
// e pelo fuso real America/Sao_Paulo (via time/tzdata, para independer do
// banco de fusos do SO). NÃO usam t.Parallel: time.Local é estado global.

import (
	"context"
	"database/sql"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tz"
)

// comLocal troca time.Local durante o teste e restaura ao final.
func comLocal(t *testing.T, loc *time.Location) {
	t.Helper()
	old := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = old })
}

// fusosTeste devolve os fusos usados na matriz de testes.
func fusosTeste(t *testing.T) []*time.Location {
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

// assertMeiaNoiteLocal garante que got é exatamente a meia-noite do dia
// informado em time.Local (mesmo instante, mesma data de calendário e mesmo
// offset do fuso local).
func assertMeiaNoiteLocal(t *testing.T, got time.Time, y int, m time.Month, d int) {
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

func newSqlmockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

const (
	reExisteCliente  = `SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`
	reExisteVendedor = `SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`
	reExisteSKU      = `SELECT 1 FROM produtos WHERE sku = \? LIMIT 1`
)

func existeRow() *sqlmock.Rows { return sqlmock.NewRows([]string{"1"}).AddRow(1) }

// parserDataCaso descreve um ponto do código de produção que converte uma
// string AAAA-MM-DD em time.Time.
type parserDataCaso struct {
	nome  string
	parse func(t *testing.T, data string) time.Time
}

func parsersDeData() []parserDataCaso {
	cfg := &config.Config{}
	return []parserDataCaso{
		{"cliente.data_cadastro", func(t *testing.T, data string) time.Time {
			_, _, _, _, _, _, got, err := validarClienteInput(ClienteInput{
				CNPJ: "11222333000181", RazaoSocial: "Cliente", Segmento: "Varejo",
				Cidade: "Curitiba", UF: "pr", DataCadastro: data,
			}, false)
			require.NoError(t, err)
			return got
		}},
		{"estoque.filtro_data", func(t *testing.T, data string) time.Time {
			got, err := parseFiltroData(data)
			require.NoError(t, err)
			require.NotNil(t, got)
			return *got
		}},
		{"estoque.data_snapshot", func(t *testing.T, data string) time.Time {
			db, mock := newSqlmockDB(t)
			mock.ExpectQuery(reExisteSKU).WithArgs("SKU-1").WillReturnRows(existeRow())
			_, got, _, err := NewEstoqueService(db, cfg).validarEstoqueInput(context.Background(), db,
				EstoqueInput{SKU: "SKU-1", DataSnapshot: data, Saldo: 5})
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
			return got
		}},
		{"oportunidade.data_abertura", func(t *testing.T, data string) time.Time {
			o := validarOportunidade(t, cfg, data, "")
			return o
		}},
		{"oportunidade.data_fechamento", func(t *testing.T, data string) time.Time {
			return validarOportunidadeFechamento(t, cfg, data)
		}},
		{"pagamento.data_vencimento", func(t *testing.T, data string) time.Time {
			_, _, got, _, err := validarPagamentoInput(pagamentoInputValido(data, ""))
			require.NoError(t, err)
			return got
		}},
		{"pagamento.data_pagamento", func(t *testing.T, data string) time.Time {
			_, _, _, got, err := validarPagamentoInput(pagamentoInputValido("2020-01-01", data))
			require.NoError(t, err)
			require.NotNil(t, got)
			return *got
		}},
		{"pedido.data_pedido", func(t *testing.T, data string) time.Time {
			p, _, err := validarPedidoInput(PedidoInput{
				ClienteID: 1, VendedorID: 2, DataPedido: data, Canal: "App", Status: "Faturado",
				Itens: []ItemPedidoInput{{ProdutoID: 3, Quantidade: 1, PrecoPraticado: 10}},
			})
			require.NoError(t, err)
			return p.DataPedido
		}},
		{"produto.data_lancamento", func(t *testing.T, data string) time.Time {
			_, _, _, _, _, _, _, _, got, err := validarProdutoInput(ProdutoInput{
				SKU: "SKU-1", Descricao: "Perfume", Categoria: "EDP", Marca: "M", Unidade: "UN",
				DataLancamento: data,
			}, true)
			require.NoError(t, err)
			require.NotNil(t, got)
			return *got
		}},
		{"vendedor.data_admissao", func(t *testing.T, data string) time.Time {
			_, _, _, got, _, err := validarVendedorInput(VendedorInput{
				Nome: "Vendedor", Regiao: "Sul", UF: "pr", DataAdmissao: data,
			}, false)
			require.NoError(t, err)
			return got
		}},
		{"visita.data_visita", func(t *testing.T, data string) time.Time {
			db, mock := newSqlmockDB(t)
			mock.ExpectQuery(reExisteCliente).WithArgs(int64(1)).WillReturnRows(existeRow())
			mock.ExpectQuery(reExisteVendedor).WithArgs(int64(2)).WillReturnRows(existeRow())
			v, err := NewVisitaService(db, cfg).validarVisitaInput(context.Background(), db,
				VisitaInput{ClienteID: 1, VendedorID: 2, DataVisita: data, Resultado: "Pedido", DuracaoMin: 30})
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
			return v.DataVisita
		}},
	}
}

func pagamentoInputValido(venc, pag string) PagamentoInput {
	return PagamentoInput{
		PedidoID: 1, FormaPagamento: "Dinheiro", Parcelas: 1, Valor: 100, ValorLiquido: 100,
		DataVencimento: venc, DataPagamento: pag, StatusPagamento: "Pago",
	}
}

func validarOportunidadeComDatas(t *testing.T, cfg *config.Config, abertura, fechamento string) (time.Time, *time.Time) {
	t.Helper()
	db, mock := newSqlmockDB(t)
	mock.ExpectQuery(reExisteCliente).WithArgs(int64(1)).WillReturnRows(existeRow())
	mock.ExpectQuery(reExisteVendedor).WithArgs(int64(2)).WillReturnRows(existeRow())
	o, err := NewOportunidadeService(db, cfg).validarOportunidadeInput(context.Background(), db, OportunidadeInput{
		ClienteID: 1, VendedorID: 2, Origem: "Indicação", DataAbertura: abertura,
		Etapa: "Prospecção", ProbabilidadePct: 10, ValorEstimado: 100, DataFechamento: fechamento,
	}, false)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	return o.DataAbertura, o.DataFechamento
}

func validarOportunidade(t *testing.T, cfg *config.Config, abertura, fechamento string) time.Time {
	got, _ := validarOportunidadeComDatas(t, cfg, abertura, fechamento)
	return got
}

func validarOportunidadeFechamento(t *testing.T, cfg *config.Config, fechamento string) time.Time {
	_, got := validarOportunidadeComDatas(t, cfg, "2020-01-01", fechamento)
	require.NotNil(t, got)
	return *got
}

// TestBUG01_DatasParseadasComoMeiaNoiteLocal é a matriz parser x fuso x data:
// cada service que converte AAAA-MM-DD deve devolver meia-noite em
// time.Local, preservando o dia de calendário informado.
func TestBUG01_DatasParseadasComoMeiaNoiteLocal(t *testing.T) {
	datas := []struct {
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

	for _, loc := range fusosTeste(t) {
		for _, p := range parsersDeData() {
			for _, d := range datas {
				t.Run(loc.String()+"/"+p.nome+"/"+d.s, func(t *testing.T) {
					comLocal(t, loc)
					got := p.parse(t, d.s)
					assertMeiaNoiteLocal(t, got, d.y, d.m, d.d)
				})
			}
		}
	}
}

// TestBUG01_RegressaoParseUTC documenta o bug original: com time.Parse (UTC)
// e time.Local = UTC-3, a data local cai no dia anterior. Serve de controle
// de que a matriz acima realmente distingue o comportamento corrigido.
func TestBUG01_RegressaoParseUTC(t *testing.T) {
	comLocal(t, time.FixedZone("UTC-3", -3*3600))

	antigo, err := time.Parse("2006-01-02", "2024-03-10")
	require.NoError(t, err)
	assert.Equal(t, 9, antigo.In(time.Local).Day(), "comportamento antigo: dia anterior no fuso local")

	for _, p := range parsersDeData() {
		t.Run(p.nome, func(t *testing.T) {
			got := p.parse(t, "2024-03-10")
			assert.Equal(t, 10, got.In(time.Local).Day())
			assert.False(t, got.Equal(antigo), "não pode coincidir com a meia-noite UTC")
		})
	}
}

// TestBUG01_DataInvalidaContinuaRejeitada garante que a troca para
// ParseInLocation não afrouxou a validação de formato.
func TestBUG01_DataInvalidaContinuaRejeitada(t *testing.T) {
	invalidas := []string{"10/03/2024", "2024-13-01", "2024-02-30", "2024-3-10", "2024-03-10T00:00:00Z", "abc"}
	cfg := &config.Config{}

	for _, s := range invalidas {
		t.Run(s, func(t *testing.T) {
			_, _, _, _, _, _, _, err := validarClienteInput(ClienteInput{
				CNPJ: "11222333000181", RazaoSocial: "C", Segmento: "S", Cidade: "C", UF: "PR", DataCadastro: s,
			}, false)
			assert.ErrorIs(t, err, ErrDataCadastroInvalida)

			_, err = parseFiltroData(s)
			assert.ErrorIs(t, err, ErrEstoqueDataInvalida)

			_, _, _, _, err = validarPagamentoInput(pagamentoInputValido(s, ""))
			assert.ErrorIs(t, err, ErrDataVencimentoInvalida)

			_, _, _, _, err = validarPagamentoInput(pagamentoInputValido("2024-01-01", s))
			assert.ErrorIs(t, err, ErrDataPagamentoInvalida)

			_, _, err = validarPedidoInput(PedidoInput{ClienteID: 1, VendedorID: 1, DataPedido: s})
			assert.ErrorIs(t, err, ErrDataPedidoInvalida)

			_, _, _, _, _, _, _, _, _, err = validarProdutoInput(ProdutoInput{
				SKU: "S", Descricao: "D", Categoria: "C", Marca: "M", Unidade: "UN", DataLancamento: s,
			}, true)
			assert.ErrorIs(t, err, ErrDataLancamentoInvalida)

			_, _, _, _, _, err = validarVendedorInput(VendedorInput{Nome: "N", Regiao: "R", UF: "PR", DataAdmissao: s}, false)
			assert.ErrorIs(t, err, ErrVendedorDataAdmissaoInvalida)

			db, mock := newSqlmockDB(t)
			_, _, _, err = NewEstoqueService(db, cfg).validarEstoqueInput(context.Background(), db,
				EstoqueInput{SKU: "SKU-1", DataSnapshot: s})
			assert.ErrorIs(t, err, ErrEstoqueDataInvalida)
			assert.NoError(t, mock.ExpectationsWereMet(), "data inválida deve falhar antes de consultar o banco")
		})
	}
}

// TestBUG01_HorarioDeVeraoHistorico documenta o caso-limite de fusos com
// horário de verão iniciando à meia-noite (America/Sao_Paulo até 2019):
// 2018-11-04 00:00 não existe no fuso e o Go normaliza para
// 2018-11-03 23:00 -03 — o dia de calendário recua 1 dia.
//
// CORRIGIDO (RISCO-01): o processo não usa mais o fuso do SO. O pacote
// shared/tz (importado em branco por shared/config, que este pacote importa)
// fixa time.Local em -03:00 sem horário de verão, em qualquer SO/container.
// O teste valida que o time.Local vigente no pacote é o fuso fixo e que,
// nele, as datas de antigo início de DST são preservadas.
func TestBUG01_HorarioDeVeraoHistorico(t *testing.T) {
	require.Same(t, tz.Local, time.Local, "time.Local deve ser o fuso fixo de shared/tz")
	comLocal(t, tz.Local)

	for _, p := range parsersDeData() {
		t.Run(p.nome, func(t *testing.T) {
			got := p.parse(t, "2018-11-04")
			y, m, d := got.In(time.Local).Date()
			assert.Equal(t, 2018, y)
			assert.Equal(t, time.November, m)
			assert.Equal(t, 4, d)
		})
	}
}

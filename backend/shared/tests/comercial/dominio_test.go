package comercial_test

// ARQ-04: domínio Comercial (VO CNPJ, fábricas com a ordem de validação do
// legado, política de escrita sob escopo, datas à meia-noite local).

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
)

func clienteValido() comercial.DadosCliente {
	return comercial.DadosCliente{CNPJ: "11.222.333/0001-81", RazaoSocial: " ACME ", Segmento: "varejo", Cidade: "Curitiba", UF: " pr ", Bairro: " Centro "}
}

func TestComercial_CNPJ(t *testing.T) {
	c, err := comercial.NovoCNPJ(" 12.abc.345/01de-35 ")
	require.NoError(t, err)
	assert.Equal(t, "12ABC34501DE35", c.String())
	assert.True(t, c.DigitosValidos())

	_, err = comercial.NovoCNPJ("   ")
	assert.ErrorIs(t, err, comercial.ErrCNPJObrigatorio)
	_, err = comercial.NovoCNPJ("12#")
	assert.ErrorIs(t, err, comercial.ErrCNPJInvalido)
	_, err = comercial.NovoCNPJ("1122233300018")
	assert.ErrorIs(t, err, comercial.ErrCNPJInvalido)

	ruim, err := comercial.NovoCNPJ("12345678000199")
	require.NoError(t, err, "formato válido")
	assert.False(t, ruim.DigitosValidos(), "DV inválido só no fim")

	assert.Equal(t, "", comercial.TermoBuscaCNPJ("acme"))
	assert.Equal(t, "12ABC", comercial.TermoBuscaCNPJ("12.abc"))
	assert.Equal(t, "", comercial.TermoBuscaCNPJ("12.#"))
}

func TestComercial_NovoCliente(t *testing.T) {
	agora := time.Date(2026, 3, 4, 15, 16, 17, 0, time.Local)
	c, err := comercial.NovoCliente(clienteValido(), agora)
	require.NoError(t, err)
	assert.Equal(t, "11222333000181", c.CNPJ)
	assert.Equal(t, "ACME", c.RazaoSocial)
	assert.Equal(t, "PR", c.UF)
	assert.Equal(t, "Centro", c.Bairro)
	assert.True(t, c.Ativo)
	assert.Equal(t, agora, c.DataCadastro, "vazio na criação = o instante de agora (legado)")

	d := clienteValido()
	d.DataCadastro = "2024-02-10"
	c, err = comercial.NovoCliente(d, agora)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 2, 10, 0, 0, 0, 0, time.Local), c.DataCadastro)

	// Ordem do legado: o DV do CNPJ é o último a ser conferido.
	casos := []struct {
		nome string
		mut  func(*comercial.DadosCliente)
		want error
	}{
		{"razão social", func(d *comercial.DadosCliente) { d.RazaoSocial = " " }, comercial.ErrRazaoSocialObrigatoria},
		{"cnpj vazio", func(d *comercial.DadosCliente) { d.CNPJ = "" }, comercial.ErrCNPJObrigatorio},
		{"segmento", func(d *comercial.DadosCliente) { d.Segmento = ""; d.CNPJ = "12345678000199" }, comercial.ErrSegmentoObrigatorio},
		{"cidade", func(d *comercial.DadosCliente) { d.Cidade = "" }, comercial.ErrCidadeObrigatoria},
		{"uf", func(d *comercial.DadosCliente) { d.UF = "PRR" }, comercial.ErrUFInvalida},
		{"data antes do DV", func(d *comercial.DadosCliente) { d.DataCadastro = "10/02/2024"; d.CNPJ = "12345678000199" }, comercial.ErrDataCadastroInvalida},
		{"DV por último", func(d *comercial.DadosCliente) { d.CNPJ = "12345678000199" }, comercial.ErrCNPJInvalido},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d := clienteValido()
			c.mut(&d)
			_, err := comercial.NovoCliente(d, agora)
			assert.ErrorIs(t, err, c.want)
		})
	}

	_, err = comercial.ClienteEditado(clienteValido())
	assert.ErrorIs(t, err, comercial.ErrDataCadastroInvalida, "na edição a data é obrigatória")
}

func TestComercial_ClienteDefinirAtivo(t *testing.T) {
	c := &comercial.Cliente{Ativo: true}
	c.DefinirAtivo(nil)
	assert.False(t, c.Ativo, "nil = toggle")
	v := true
	c.DefinirAtivo(&v)
	assert.True(t, c.Ativo)
}

func TestComercial_Vendedor(t *testing.T) {
	agora := time.Now()
	ok := comercial.DadosVendedor{Nome: " Ana ", Regiao: "Sul", UF: "pr", MetaMensal: 10}
	v, err := comercial.NovoVendedor(ok, agora)
	require.NoError(t, err)
	assert.Equal(t, "Ana", v.Nome)
	assert.Equal(t, "PR", v.UF)
	assert.Equal(t, agora, v.DataAdmissao)

	_, err = comercial.VendedorEditado(ok)
	assert.ErrorIs(t, err, comercial.ErrDataAdmissaoInvalida)

	casos := []struct {
		nome string
		mut  func(*comercial.DadosVendedor)
		want error
	}{
		{"nome", func(d *comercial.DadosVendedor) { d.Nome = "" }, comercial.ErrNomeObrigatorio},
		{"região", func(d *comercial.DadosVendedor) { d.Regiao = "" }, comercial.ErrRegiaoObrigatoria},
		{"uf", func(d *comercial.DadosVendedor) { d.UF = "X" }, comercial.ErrUFInvalida},
		{"meta", func(d *comercial.DadosVendedor) { d.MetaMensal = -1 }, comercial.ErrMetaMensalInvalida},
		{"data", func(d *comercial.DadosVendedor) { d.DataAdmissao = "x" }, comercial.ErrDataAdmissaoInvalida},
	}
	for _, c := range casos {
		d := ok
		c.mut(&d)
		_, err := comercial.NovoVendedor(d, agora)
		assert.ErrorIs(t, err, c.want, c.nome)
	}
}

func TestComercial_VisitaEOportunidade(t *testing.T) {
	vis := comercial.DadosVisita{ClienteID: 1, VendedorID: 2, DataVisita: "2024-05-06", Resultado: " ok ", DuracaoMin: 3}
	require.NoError(t, vis.Validar())
	v, err := vis.Visita()
	require.NoError(t, err)
	assert.Equal(t, "ok", v.Resultado)
	assert.Equal(t, time.Date(2024, 5, 6, 0, 0, 0, 0, time.Local), v.DataVisita)
	vis.DataVisita = "06/05/2024"
	_, err = vis.Visita()
	assert.ErrorIs(t, err, comercial.ErrDataVisitaInvalida)

	op := comercial.DadosOportunidade{ClienteID: 1, VendedorID: 2, Origem: "x", Etapa: comercial.EtapaFechadoPerdido,
		MotivoPerda: " preço ", DataFechamento: "2024-07-08", ProbabilidadePct: 50}
	require.NoError(t, op.Validar())
	agora := time.Now()
	o, err := op.Oportunidade(&agora)
	require.NoError(t, err)
	assert.Equal(t, agora, o.DataAbertura)
	require.NotNil(t, o.DataFechamento)
	assert.Equal(t, time.Date(2024, 7, 8, 0, 0, 0, 0, time.Local), *o.DataFechamento)
	require.NotNil(t, o.MotivoPerda)
	assert.Equal(t, "preço", *o.MotivoPerda)

	_, err = op.Oportunidade(nil)
	assert.ErrorIs(t, err, comercial.ErrDataAberturaInvalida, "na edição data_abertura é obrigatória")
	op.DataAbertura, op.DataFechamento = "2024-01-01", "x"
	_, err = op.Oportunidade(nil)
	assert.ErrorIs(t, err, comercial.ErrDataFechamentoInvalida)

	casosOp := []struct {
		mut  func(*comercial.DadosOportunidade)
		want error
	}{
		{func(d *comercial.DadosOportunidade) { d.ClienteID = 0 }, comercial.ErrClienteInvalido},
		{func(d *comercial.DadosOportunidade) { d.VendedorID = 0 }, comercial.ErrVendedorInvalido},
		{func(d *comercial.DadosOportunidade) { d.Origem = " " }, comercial.ErrOrigemObrigatoria},
		{func(d *comercial.DadosOportunidade) { d.Etapa = "" }, comercial.ErrEtapaObrigatoria},
		{func(d *comercial.DadosOportunidade) { d.ProbabilidadePct = 101 }, comercial.ErrProbabilidadeInvalida},
		{func(d *comercial.DadosOportunidade) { d.ValorEstimado = -1 }, comercial.ErrValorEstimadoInvalido},
		{func(d *comercial.DadosOportunidade) { d.MotivoPerda = "" }, comercial.ErrMotivoPerdaObrigatorio},
	}
	for i, c := range casosOp {
		d := comercial.DadosOportunidade{ClienteID: 1, VendedorID: 2, Origem: "x", Etapa: comercial.EtapaFechadoPerdido, MotivoPerda: "m"}
		c.mut(&d)
		assert.ErrorIs(t, d.Validar(), c.want, "caso %d", i)
	}
}

// E-05: a política de escrita força o vendedor do escopo e falha fechado
// para SemAcesso e zero value.
func TestComercial_VendedorDaEscrita(t *testing.T) {
	admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	irrestrito, _ := kernel.EscopoIrrestrito(admin)
	doVendedor, _ := kernel.EscopoDoVendedor(10)

	id, restrito, err := comercial.VendedorDaEscrita(irrestrito, 99)
	assert.Equal(t, []any{int64(99), false, nil}, []any{id, restrito, err}, "admin: vale o payload")

	id, restrito, err = comercial.VendedorDaEscrita(doVendedor, 99)
	assert.Equal(t, []any{int64(10), true, nil}, []any{id, restrito, err}, "restrito: payload ignorado")

	_, _, err = comercial.VendedorDaEscrita(kernel.EscopoSemAcesso(), 99)
	assert.Same(t, comercial.ErrSemVendedor, err, "SemAcesso: 403")
	_, _, err = comercial.VendedorDaEscrita(kernel.Escopo{}, 99)
	assert.Same(t, kernel.ErrEscopoInvalido, err, "N5: zero value é 500, não 403")
}

func TestComercial_Carteira(t *testing.T) {
	loc := time.FixedZone("-03", -3*3600)
	velho := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = velho })

	// 01:30 UTC do dia 5 ainda é dia 4 em -03.
	c := comercial.NovaCarteiraDoDia(1, 2, time.Date(2026, 1, 5, 1, 30, 0, 0, time.UTC))
	assert.Equal(t, time.Date(2026, 1, 4, 0, 0, 0, 0, loc), c.DataInicio)
	assert.True(t, c.Ativa())

	r := comercial.ResumoDoVinculo(&comercial.Cliente{ClienteIDOrigem: 7, CNPJ: "x"}, &comercial.Carteira{CarteiraIDOrigem: 3})
	assert.Equal(t, int64(7), r.ID)
	assert.Equal(t, int64(3), r.CarteiraID)
}

func TestKernel_ParseDataLocal(t *testing.T) {
	d, err := kernel.ParseDataLocal(" 2024-02-29 ", comercial.ErrDataVisitaInvalida)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 2, 29, 0, 0, 0, 0, time.Local), d)
	_, err = kernel.ParseDataLocal("2024-02-30", comercial.ErrDataVisitaInvalida)
	assert.ErrorIs(t, err, comercial.ErrDataVisitaInvalida)
}

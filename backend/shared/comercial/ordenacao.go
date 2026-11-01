package comercial

import "github.com/rotaperfumes/shared/kernel"

// CampoCliente é o enum de ordenação de clientes (C-02). O valor é o nome
// aceito em ?order_by=; a coluna sai de um switch no adapter.
type CampoCliente string

// Campos de ordenação de clientes.
const (
	OrdemClienteID           CampoCliente = "id"
	OrdemClienteRazaoSocial  CampoCliente = "razao_social"
	OrdemClienteCNPJ         CampoCliente = "cnpj"
	OrdemClienteSegmento     CampoCliente = "segmento"
	OrdemClienteCidade       CampoCliente = "cidade"
	OrdemClienteUF           CampoCliente = "uf"
	OrdemClienteDataCadastro CampoCliente = "data_cadastro"
	OrdemClienteAtivo        CampoCliente = "ativo"
	OrdemClienteCriadoEm     CampoCliente = "created_at"
	OrdemClienteAtualizadoEm CampoCliente = "updated_at"
)

// CamposOrdenacaoCliente devolve a whitelist de clientes.
func CamposOrdenacaoCliente() []CampoCliente {
	return []CampoCliente{
		OrdemClienteID, OrdemClienteRazaoSocial, OrdemClienteCNPJ, OrdemClienteSegmento, OrdemClienteCidade,
		OrdemClienteUF, OrdemClienteDataCadastro, OrdemClienteAtivo, OrdemClienteCriadoEm, OrdemClienteAtualizadoEm,
	}
}

// OrdenacaoClientePadrao é "id ASC".
func OrdenacaoClientePadrao() kernel.Ordenacao[CampoCliente] {
	return kernel.NovaOrdenacao(OrdemClienteID, kernel.Asc)
}

// CampoVisita é o enum de ordenação de visitas ("id" e "visita_id" são a
// mesma coluna, como no legado).
type CampoVisita string

// Campos de ordenação de visitas.
const (
	OrdemVisitaID           CampoVisita = "id"
	OrdemVisitaVisitaID     CampoVisita = "visita_id"
	OrdemVisitaData         CampoVisita = "data_visita"
	OrdemVisitaDuracao      CampoVisita = "duracao_min"
	OrdemVisitaResultado    CampoVisita = "resultado"
	OrdemVisitaCriadoEm     CampoVisita = "created_at"
	OrdemVisitaAtualizadoEm CampoVisita = "updated_at"
)

// CamposOrdenacaoVisita devolve a whitelist de visitas.
func CamposOrdenacaoVisita() []CampoVisita {
	return []CampoVisita{
		OrdemVisitaID, OrdemVisitaVisitaID, OrdemVisitaData, OrdemVisitaDuracao,
		OrdemVisitaResultado, OrdemVisitaCriadoEm, OrdemVisitaAtualizadoEm,
	}
}

// OrdenacaoVisitaPadrao é "id ASC".
func OrdenacaoVisitaPadrao() kernel.Ordenacao[CampoVisita] {
	return kernel.NovaOrdenacao(OrdemVisitaID, kernel.Asc)
}

// CampoOportunidade é o enum de ordenação de oportunidades.
type CampoOportunidade string

// Campos de ordenação de oportunidades.
const (
	OrdemOportunidadeID            CampoOportunidade = "id"
	OrdemOportunidadeDataAbertura  CampoOportunidade = "data_abertura"
	OrdemOportunidadeValorEstimado CampoOportunidade = "valor_estimado"
	OrdemOportunidadeProbabilidade CampoOportunidade = "probabilidade_pct"
	OrdemOportunidadeEtapa         CampoOportunidade = "etapa"
	OrdemOportunidadeOrigem        CampoOportunidade = "origem"
	OrdemOportunidadeCriadoEm      CampoOportunidade = "created_at"
	OrdemOportunidadeAtualizadoEm  CampoOportunidade = "updated_at"
)

// CamposOrdenacaoOportunidade devolve a whitelist de oportunidades.
func CamposOrdenacaoOportunidade() []CampoOportunidade {
	return []CampoOportunidade{
		OrdemOportunidadeID, OrdemOportunidadeDataAbertura, OrdemOportunidadeValorEstimado,
		OrdemOportunidadeProbabilidade, OrdemOportunidadeEtapa, OrdemOportunidadeOrigem,
		OrdemOportunidadeCriadoEm, OrdemOportunidadeAtualizadoEm,
	}
}

// OrdenacaoOportunidadePadrao é "id ASC".
func OrdenacaoOportunidadePadrao() kernel.Ordenacao[CampoOportunidade] {
	return kernel.NovaOrdenacao(OrdemOportunidadeID, kernel.Asc)
}

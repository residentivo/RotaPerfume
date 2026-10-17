package comercial

import (
	"strings"
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// EtapaFechadoPerdido é a etapa que exige motivo_perda.
const EtapaFechadoPerdido = "Fechado perdido"

// Oportunidade é uma oportunidade do funil. DataFechamento, CicloDias e
// MotivoPerda são opcionais (NULL no banco).
type Oportunidade struct {
	OportunidadeID   int64
	ClienteID        int64
	VendedorID       int64
	Origem           string
	DataAbertura     time.Time
	Etapa            string
	ProbabilidadePct float64
	ValorEstimado    float64
	DataFechamento   *time.Time
	CicloDias        *int
	MotivoPerda      *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// DadosOportunidade é a entrada crua de criação/edição (o comando). Para
// escopo restrito, VendedorID é sobrescrito pelo do escopo (E-05).
type DadosOportunidade struct {
	ClienteID        int64
	VendedorID       int64
	Origem           string
	DataAbertura     string // AAAA-MM-DD; vazio = agora só na criação
	Etapa            string
	ProbabilidadePct float64
	ValorEstimado    float64
	DataFechamento   string // AAAA-MM-DD, opcional
	CicloDias        *int
	MotivoPerda      string // obrigatório na etapa "Fechado perdido"
}

// Validar confere os campos que não dependem do banco, na ordem do legado.
// A existência de cliente e vendedor vem depois; as datas, por último
// (Oportunidade).
func (d DadosOportunidade) Validar() error {
	etapa := strings.TrimSpace(d.Etapa)
	switch {
	case d.ClienteID <= 0:
		return ErrClienteInvalido
	case d.VendedorID <= 0:
		return ErrVendedorInvalido
	case strings.TrimSpace(d.Origem) == "":
		return ErrOrigemObrigatoria
	case etapa == "":
		return ErrEtapaObrigatoria
	case d.ProbabilidadePct < 0 || d.ProbabilidadePct > 100:
		return ErrProbabilidadeInvalida
	case d.ValorEstimado < 0:
		return ErrValorEstimadoInvalido
	case etapa == EtapaFechadoPerdido && strings.TrimSpace(d.MotivoPerda) == "":
		return ErrMotivoPerdaObrigatorio
	}
	return nil
}

// Oportunidade converte os dados já validados na entidade. agora != nil (a
// criação) faz data_abertura vazia virar agora; na edição é obrigatória.
func (d DadosOportunidade) Oportunidade(agora *time.Time) (*Oportunidade, error) {
	abertura, err := dataOuAgora(d.DataAbertura, agora, ErrDataAberturaInvalida)
	if err != nil {
		return nil, err
	}
	o := &Oportunidade{
		ClienteID: d.ClienteID, VendedorID: d.VendedorID, Origem: strings.TrimSpace(d.Origem),
		DataAbertura: abertura, Etapa: strings.TrimSpace(d.Etapa), ProbabilidadePct: d.ProbabilidadePct,
		ValorEstimado: d.ValorEstimado, CicloDias: d.CicloDias,
	}
	if strings.TrimSpace(d.DataFechamento) != "" {
		fechamento, err := kernel.ParseDataLocal(d.DataFechamento, ErrDataFechamentoInvalida)
		if err != nil {
			return nil, err
		}
		o.DataFechamento = &fechamento
	}
	if motivo := strings.TrimSpace(d.MotivoPerda); motivo != "" {
		o.MotivoPerda = &motivo
	}
	return o, nil
}

// FiltroOportunidade é o query object da listagem (VendedorID: ver
// FiltroVisita).
type FiltroOportunidade struct {
	ClienteID  int64
	VendedorID int64
	Etapa      string
	Origem     string
	De, Ate    string // data_abertura AAAA-MM-DD (inclusive)
	Busca      string // em origem OU etapa (LIKE)
	Ordenacao  kernel.Ordenacao[CampoOportunidade]
}

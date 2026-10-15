package comercial

import (
	"strings"
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// Visita é a visita de um vendedor a um cliente.
type Visita struct {
	VisitaID   int64
	ClienteID  int64
	VendedorID int64
	DataVisita time.Time
	Resultado  string
	DuracaoMin int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// DadosVisita é a entrada crua de criação/edição (o comando). Para escopo
// restrito, VendedorID é sobrescrito pelo do escopo antes de validar (E-05).
type DadosVisita struct {
	ClienteID  int64
	VendedorID int64
	DataVisita string // AAAA-MM-DD, obrigatória
	Resultado  string
	DuracaoMin int
}

// Validar confere os campos que não dependem do banco, na ordem do legado:
// cliente, vendedor, resultado, duração e data presente. A existência de
// cliente e vendedor (banco) vem depois, e só então o parse da data
// (Visita).
func (d DadosVisita) Validar() error {
	switch {
	case d.ClienteID <= 0:
		return ErrClienteInvalido
	case d.VendedorID <= 0:
		return ErrVendedorInvalido
	case strings.TrimSpace(d.Resultado) == "":
		return ErrResultadoObrigatorio
	case d.DuracaoMin < 0:
		return ErrDuracaoInvalida
	case strings.TrimSpace(d.DataVisita) == "":
		return ErrDataVisitaInvalida
	}
	return nil
}

// Visita converte os dados já validados (inclusive a existência) na
// entidade, com a data à meia-noite local (BUG-01).
func (d DadosVisita) Visita() (*Visita, error) {
	data, err := kernel.ParseDataLocal(d.DataVisita, ErrDataVisitaInvalida)
	if err != nil {
		return nil, err
	}
	return &Visita{ClienteID: d.ClienteID, VendedorID: d.VendedorID, DataVisita: data,
		Resultado: strings.TrimSpace(d.Resultado), DuracaoMin: d.DuracaoMin}, nil
}

// FiltroVisita é o query object da listagem. VendedorID é filtro de
// negócio (admin); para escopo restrito o adapter o sobrescreve com o
// vendedor do escopo, ignorando a query (E-03).
type FiltroVisita struct {
	ClienteID  int64
	VendedorID int64
	Resultado  string
	De, Ate    string // AAAA-MM-DD (inclusive), como texto, igual ao legado
	Busca      string // em resultado (LIKE)
	Ordenacao  kernel.Ordenacao[CampoVisita]
}

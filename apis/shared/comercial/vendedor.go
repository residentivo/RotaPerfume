package comercial

import (
	"strings"
	"time"
)

// Vendedor é o agregado do vendedor. DataDesligamento nil = ativo; o
// desligamento é idempotente (o adapter grava com COALESCE e preserva a
// data original, BUG-04) e, na mesma tx, inativa os usuários e revoga as
// sessões (I-09, comercialapp.DesligarVendedor).
type Vendedor struct {
	ID               int64
	Nome             string
	Regiao           string
	UF               string
	DataAdmissao     time.Time
	DataDesligamento *time.Time
	MetaMensal       float64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// VendedorResumo é a forma reduzida das listas de seleção (inclui
// DataDesligamento para o frontend marcar os inativos).
type VendedorResumo struct {
	ID               int64
	Nome             string
	Regiao           string
	UF               string
	DataDesligamento *time.Time
}

// DetalheVendedor é o vendedor com os clientes da carteira ativa.
type DetalheVendedor struct {
	Vendedor
	Clientes []ClienteResumo
}

// DadosVendedor é a entrada crua de criação/edição (o comando).
type DadosVendedor struct {
	Nome         string
	Regiao       string
	UF           string
	DataAdmissao string // AAAA-MM-DD; vazio = hoje só na criação
	MetaMensal   float64
}

// NovoVendedor valida a criação: data_admissao vazia vira agora.
func NovoVendedor(d DadosVendedor, agora time.Time) (*Vendedor, error) {
	return validarVendedor(d, &agora)
}

// VendedorEditado valida o PUT (data_admissao obrigatória; o desligamento
// não muda por aqui).
func VendedorEditado(d DadosVendedor) (*Vendedor, error) {
	return validarVendedor(d, nil)
}

// validarVendedor segue a ordem do legado: nome, região, UF, meta e data.
func validarVendedor(d DadosVendedor, agora *time.Time) (*Vendedor, error) {
	v := &Vendedor{Nome: strings.TrimSpace(d.Nome), Regiao: strings.TrimSpace(d.Regiao), MetaMensal: d.MetaMensal}
	var err error
	switch {
	case v.Nome == "":
		return nil, ErrNomeObrigatorio
	case v.Regiao == "":
		return nil, ErrRegiaoObrigatoria
	}
	if v.UF, err = normalizarUF(d.UF); err != nil {
		return nil, err
	}
	if v.MetaMensal < 0 {
		return nil, ErrMetaMensalInvalida
	}
	if v.DataAdmissao, err = dataOuAgora(d.DataAdmissao, agora, ErrDataAdmissaoInvalida); err != nil {
		return nil, err
	}
	return v, nil
}

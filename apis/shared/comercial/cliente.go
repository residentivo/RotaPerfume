package comercial

import (
	"strings"
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// Cliente é o cliente do CRM. ClienteIDOrigem é a PK (AUTO_INCREMENT,
// 1:1 com o cliente_id do CSV de origem). Nasce válido por NovoCliente; a
// edição valida por ClienteEditado.
type Cliente struct {
	ClienteIDOrigem int64
	CNPJ            string // normalizado (VO CNPJ); nunca logado (M-04)
	RazaoSocial     string
	Segmento        string
	Cidade          string
	UF              string
	Bairro          string
	DataCadastro    time.Time
	Ativo           bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// DadosCliente é a entrada crua de criação/edição (o comando).
type DadosCliente struct {
	CNPJ         string
	RazaoSocial  string
	Segmento     string
	Cidade       string
	UF           string
	Bairro       string
	DataCadastro string // AAAA-MM-DD; vazio = hoje só na criação
}

// NovoCliente valida e cria um cliente ATIVO; data_cadastro vazia vira
// agora (o instante, como no legado).
func NovoCliente(d DadosCliente, agora time.Time) (*Cliente, error) {
	c, err := validarCliente(d, &agora)
	if err != nil {
		return nil, err
	}
	c.Ativo = true
	return c, nil
}

// ClienteEditado valida os campos editáveis de um PUT (data_cadastro é
// obrigatória; ativo e id não mudam por aqui).
func ClienteEditado(d DadosCliente) (*Cliente, error) {
	return validarCliente(d, nil)
}

// validarCliente aplica a ordem do legado: razão social, CNPJ (obrigatório
// e formato), segmento, cidade, UF, data e, por último, o DV do CNPJ.
func validarCliente(d DadosCliente, agora *time.Time) (*Cliente, error) {
	c := &Cliente{
		RazaoSocial: strings.TrimSpace(d.RazaoSocial),
		Segmento:    strings.TrimSpace(d.Segmento),
		Cidade:      strings.TrimSpace(d.Cidade),
		Bairro:      strings.TrimSpace(d.Bairro),
	}
	if c.RazaoSocial == "" {
		return nil, ErrRazaoSocialObrigatoria
	}
	doc, err := NovoCNPJ(d.CNPJ)
	if err != nil {
		return nil, err
	}
	if err := c.validarLocal(d.UF); err != nil {
		return nil, err
	}
	if c.DataCadastro, err = dataOuAgora(d.DataCadastro, agora, ErrDataCadastroInvalida); err != nil {
		return nil, err
	}
	if !doc.DigitosValidos() {
		return nil, ErrCNPJInvalido
	}
	c.CNPJ = doc.String()
	return c, nil
}

// validarLocal confere segmento, cidade e UF (normalizada).
func (c *Cliente) validarLocal(uf string) (err error) {
	switch {
	case c.Segmento == "":
		return ErrSegmentoObrigatorio
	case c.Cidade == "":
		return ErrCidadeObrigatoria
	}
	c.UF, err = normalizarUF(uf)
	return err
}

// DefinirAtivo aplica o PATCH /inativar: nil inverte o status (toggle); um
// valor o define.
func (c *Cliente) DefinirAtivo(ativo *bool) {
	if ativo == nil {
		c.Ativo = !c.Ativo
		return
	}
	c.Ativo = *ativo
}

// FiltroCliente é o query object da listagem. O escopo de carteira NÃO é
// filtro: vem obrigatório como kernel.Escopo (E-02/O1).
type FiltroCliente struct {
	UF        string
	Segmento  string
	Ativo     *bool
	Busca     string // razão social OU cnpj (LIKE; ver TermoBuscaCNPJ)
	Ordenacao kernel.Ordenacao[CampoCliente]
}

// dataOuAgora: vazio vira *agora (criação) ou errCampo (edição); senão é a
// meia-noite local da data (kernel.ParseDataLocal, BUG-01).
func dataOuAgora(s string, agora *time.Time, errCampo error) (time.Time, error) {
	if strings.TrimSpace(s) != "" {
		return kernel.ParseDataLocal(s, errCampo)
	}
	if agora == nil {
		return time.Time{}, errCampo
	}
	return *agora, nil
}

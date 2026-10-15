package catalogo

import (
	"strings"
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// RegistroEstoque é o snapshot diário de saldo de um SKU (série temporal:
// vários registros por SKU). A ruptura NUNCA vem da entrada: é sempre
// derivada de saldo <= 0 (exigência do SecBrain).
type RegistroEstoque struct {
	ID           int64
	DataSnapshot time.Time // meia-noite em time.Local (BUG-01)
	SKU          SKU
	// ProdutoDescricao vem do JOIN com produtos (só leitura).
	ProdutoDescricao string
	Saldo            int
	Ruptura          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// DadosEstoque é a entrada crua de um ajuste manual (o comando).
type DadosEstoque struct {
	SKU          string
	DataSnapshot string // AAAA-MM-DD, obrigatória e não futura
	Saldo        int
}

// NovoRegistroEstoque valida um ajuste manual na ordem do legado: SKU, data
// (obrigatória, formato, não futura em relação a agora) e saldo. A
// existência do SKU em produtos é conferida pelo caso de uso (precisa do
// banco), depois destas regras.
func NovoRegistroEstoque(d DadosEstoque, agora time.Time) (*RegistroEstoque, error) {
	sku, err := NovoSKU(d.SKU)
	if err != nil {
		return nil, err
	}
	data, err := dataSnapshot(d.DataSnapshot, agora)
	if err != nil {
		return nil, err
	}
	e := &RegistroEstoque{SKU: sku, DataSnapshot: data}
	if err := e.AjustarSaldo(d.Saldo); err != nil {
		return nil, err
	}
	return e, nil
}

func dataSnapshot(s string, agora time.Time) (time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return time.Time{}, ErrEstoqueDataObrigatoria
	}
	data, err := kernel.ParseDataLocal(s, ErrEstoqueDataInvalida)
	if err != nil {
		return time.Time{}, err
	}
	if data.After(agora) {
		return time.Time{}, ErrEstoqueDataFutura
	}
	return data, nil
}

// AjustarSaldo define o saldo (>= 0) e recalcula a ruptura.
func (e *RegistroEstoque) AjustarSaldo(saldo int) error {
	if saldo < 0 {
		return ErrEstoqueSaldoInvalido
	}
	e.Saldo = saldo
	e.Ruptura = saldo <= 0
	return nil
}

// FiltroEstoque é o query object da listagem. Campos vazios/nil não filtram;
// as datas são inclusivas.
type FiltroEstoque struct {
	SKU       string
	De        *kernel.Data
	Ate       *kernel.Data
	Ruptura   *bool
	Ordenacao kernel.Ordenacao[CampoEstoque]
}

// NovoFiltroEstoque lê data_de/data_ate (AAAA-MM-DD, opcionais). Data
// malformada devolve ErrEstoqueDataInvalida antes de qualquer consulta.
func NovoFiltroEstoque(sku, de, ate string, ruptura *bool, o kernel.Ordenacao[CampoEstoque]) (FiltroEstoque, error) {
	dataDe, err := dataOpcional(de)
	if err != nil {
		return FiltroEstoque{}, err
	}
	dataAte, err := dataOpcional(ate)
	if err != nil {
		return FiltroEstoque{}, err
	}
	return FiltroEstoque{SKU: strings.TrimSpace(sku), De: dataDe, Ate: dataAte, Ruptura: ruptura, Ordenacao: o}, nil
}

func dataOpcional(s string) (*kernel.Data, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	d, err := kernel.ParseData(s, ErrEstoqueDataInvalida)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

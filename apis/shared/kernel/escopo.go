package kernel

type tipoEscopo uint8

const (
	escopoInvalido tipoEscopo = iota // zero value: NUNCA utilizável
	escopoIrrestrito
	escopoVendedor
	escopoSemAcesso
)

// Escopo é a restrição de carteira de uma requisição: Irrestrito,
// Vendedor(id>0) ou SemAcesso.
//
// Não existe "0 = global": o zero value é inválido e todo consumidor chama
// Validar ou Filtro (fail-closed, E-02). Os construtores Escopo* só podem ser
// chamados pelo AutorizadorEscopo (shared/comercial/comercialapp), pelo
// próprio kernel e por testes (TestARQ_Escopo_SoAutorizador, O5).
type Escopo struct {
	tipo       tipoEscopo
	vendedorID int64
}

// ErrEscopoInvalido sinaliza escopo zero value ou vendedor sem id (500).
var ErrEscopoInvalido = NovoErro(KindInterno, "erro interno")

// ErrEscopoSemAcesso sinaliza ao adapter que NÃO deve executar SQL. O caso de
// uso responde vazio (E-03) ou 404 (E-04) antes de chegar ao adapter; se
// chegar, é a segunda barreira.
var ErrEscopoSemAcesso = NovoErro(KindInterno, "erro interno")

// EscopoIrrestrito só nasce de um Principal admin (E-01).
func EscopoIrrestrito(p Principal) (Escopo, error) {
	if !p.EhAdmin() {
		return Escopo{}, ErrEscopoInvalido
	}
	return Escopo{tipo: escopoIrrestrito}, nil
}

// EscopoDoVendedor restringe à carteira do vendedor id (> 0).
func EscopoDoVendedor(id int64) (Escopo, error) {
	if id <= 0 {
		return Escopo{}, ErrEscopoInvalido
	}
	return Escopo{tipo: escopoVendedor, vendedorID: id}, nil
}

// EscopoSemAcesso é o escopo de quem não tem carteira (nenhum registro).
func EscopoSemAcesso() Escopo { return Escopo{tipo: escopoSemAcesso} }

// Validar devolve ErrEscopoInvalido para o zero value.
func (e Escopo) Validar() error {
	if e.tipo == escopoInvalido || (e.tipo == escopoVendedor && e.vendedorID <= 0) {
		return ErrEscopoInvalido
	}
	return nil
}

// SemAcesso informa se o escopo é SemAcesso.
func (e Escopo) SemAcesso() bool { return e.tipo == escopoSemAcesso }

// Irrestrito informa se o escopo é Irrestrito. Não use para decidir SQL.
func (e Escopo) Irrestrito() bool { return e.tipo == escopoIrrestrito }

// Filtro é a ÚNICA forma de traduzir Escopo em SQL (O1). switch exaustivo:
//   - Irrestrito      → filtra=false (sem filtro de vendedor);
//   - Vendedor(id>0)  → filtra=true, id;
//   - SemAcesso       → ErrEscopoSemAcesso (não executar SQL);
//   - inválido/zero   → ErrEscopoInvalido  (não executar SQL).
//
// O adapter só deduz "global" de err == nil && !filtra.
func (e Escopo) Filtro() (vendedorID int64, filtra bool, err error) {
	switch e.tipo {
	case escopoIrrestrito:
		return 0, false, nil
	case escopoVendedor:
		if e.vendedorID <= 0 {
			return 0, false, ErrEscopoInvalido
		}
		return e.vendedorID, true, nil
	case escopoSemAcesso:
		return 0, false, ErrEscopoSemAcesso
	default:
		return 0, false, ErrEscopoInvalido
	}
}

// VendedorID devolve o id só no escopo de vendedor; 0 em qualquer outro caso.
// Serve para log e para forçar o VendedorID em comandos (E-05). NÃO serve
// para decidir filtro SQL (use Filtro).
func (e Escopo) VendedorID() int64 {
	if e.tipo == escopoVendedor {
		return e.vendedorID
	}
	return 0
}

// Permite reporta se um registro do vendedorID pode ser visto (E-04).
func (e Escopo) Permite(vendedorID int64) bool {
	switch e.tipo {
	case escopoIrrestrito:
		return true
	case escopoVendedor:
		return vendedorID == e.vendedorID
	default:
		return false
	}
}

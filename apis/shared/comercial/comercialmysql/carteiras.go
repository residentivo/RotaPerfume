package comercialmysql

import (
	"context"
	"time"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/vlog"
)

const arqCarteiras = "carteiras.go"

// carteiraSelect é a base dos SELECTs de vínculo (ordem do scan).
const carteiraSelect = `
		SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at
		FROM carteiras
		`

// clienteResumoColunas/From: o cliente com o vínculo de carteira (JOIN).
const (
	clienteResumoColunas = `c.cliente_id_origem, c.cnpj, c.razao_social, c.segmento, c.cidade, c.uf, ca.carteira_id_origem, ca.data_inicio, ca.data_fim`
	clienteResumoFrom    = ` FROM carteiras ca JOIN clientes c ON c.cliente_id_origem = ca.cliente_id`
)

// qualquerIndice: em carteiras o único índice único além da PK é
// (cliente_id, vendedor_id, data_inicio); qualquer 1062 é esse.
const qualquerIndice = ""

// Carteiras é o repositório dos vínculos cliente↔vendedor (porta
// comercialapp.Carteiras).
type Carteiras struct{ q db.Querier }

// NovoCarteiras liga o repositório ao pool (ou a uma tx). q nil: panic.
func NovoCarteiras(q db.Querier) *Carteiras {
	if q == nil {
		panic("comercialmysql: NovoCarteiras exige Querier")
	}
	return &Carteiras{q: q}
}

// ClientesDoVendedor lista os clientes da carteira ATIVA do vendedor, por
// razão social. Sem linhas devolve [] (não nulo): o JSON legado é
// "clientes": [].
func (r *Carteiras) ClientesDoVendedor(ctx context.Context, vendedorID int64) ([]comercial.ClienteResumo, error) {
	q := "SELECT " + clienteResumoColunas + clienteResumoFrom +
		" WHERE ca.vendedor_id = ? AND ca.data_fim IS NULL ORDER BY c.razao_social ASC"
	out, err := db.Consultar(ctx, r.q, "comercialmysql: listar clientes por vendedor", q, []any{vendedorID}, scanClienteResumo)
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqCarteiras, "Carteiras.ClientesDoVendedor", "SELECT carteiras linhas=%d", len(out))
	if out == nil {
		out = []comercial.ClienteResumo{}
	}
	return out, nil
}

func scanClienteResumo(s db.Linha) (*comercial.ClienteResumo, error) {
	return db.LerUma(s, comercial.ErrVinculoNaoEncontrado, "comercialmysql: ler cliente resumo", func(c *comercial.ClienteResumo) []any {
		return []any{&c.ID, &c.CNPJ, &c.RazaoSocial, &c.Segmento, &c.Cidade, &c.UF, &c.CarteiraID, &c.DataInicio, &c.DataFim}
	})
}

// PorID busca o vínculo pelo id (sem chamador HTTP; mantido com os testes
// do repositório legado).
func (r *Carteiras) PorID(ctx context.Context, id int64) (*comercial.Carteira, error) {
	return r.vinculo(ctx, "Carteiras.PorID", carteiraSelect+`WHERE carteira_id_origem = ?
		LIMIT 1`, id)
}

// VinculoAtivo busca o vínculo ativo entre vendedor e cliente (consulta
// própria a cada chamada: base da checagem de carteira, E-05).
func (r *Carteiras) VinculoAtivo(ctx context.Context, vendedorID, clienteID int64) (*comercial.Carteira, error) {
	return r.vinculo(ctx, "Carteiras.VinculoAtivo", carteiraSelect+`WHERE vendedor_id = ? AND cliente_id = ? AND data_fim IS NULL
		LIMIT 1`, vendedorID, clienteID)
}

// VinculoAtivoDoCliente busca o vínculo ativo do cliente com qualquer
// vendedor (detecta transferência de carteira).
func (r *Carteiras) VinculoAtivoDoCliente(ctx context.Context, clienteID int64) (*comercial.Carteira, error) {
	return r.vinculo(ctx, "Carteiras.VinculoAtivoDoCliente", carteiraSelect+`WHERE cliente_id = ? AND data_fim IS NULL
		LIMIT 1`, clienteID)
}

// VinculoDoDia busca o vínculo (ativo ou encerrado) do par com data_inicio
// no dia informado (revinculação no mesmo dia, sem violar a chave única).
func (r *Carteiras) VinculoDoDia(ctx context.Context, clienteID, vendedorID int64, dia time.Time) (*comercial.Carteira, error) {
	return r.vinculo(ctx, "Carteiras.VinculoDoDia", carteiraSelect+`WHERE cliente_id = ? AND vendedor_id = ? AND data_inicio = ?
		LIMIT 1`, clienteID, vendedorID, dia.Format("2006-01-02"))
}

func (r *Carteiras) vinculo(ctx context.Context, rotulo, q string, args ...any) (*comercial.Carteira, error) {
	c, err := db.LerUma(r.q.QueryRowContext(ctx, q, args...), comercial.ErrVinculoNaoEncontrado, "comercialmysql: ler carteira", func(c *comercial.Carteira) []any {
		return []any{&c.CarteiraIDOrigem, &c.ClienteID, &c.VendedorID, &c.DataInicio, &c.DataFim, &c.CreatedAt, &c.UpdatedAt}
	})
	vlog.Printf(arqCarteiras, rotulo, "SELECT carteiras encontrado=%t", err == nil)
	return c, err
}

// Inserir grava o vínculo e preenche c.CarteiraIDOrigem. O 1062 (chave
// cliente+vendedor+data) vira ErrVinculoDuplicado SEM o erro do driver
// (N-8); continua 500, como no legado.
func (r *Carteiras) Inserir(ctx context.Context, c *comercial.Carteira) error {
	const q = `
		INSERT INTO carteiras (cliente_id, vendedor_id, data_inicio, data_fim)
		VALUES (?, ?, ?, ?)`
	id, err := db.Inserir(ctx, r.q, "comercialmysql: inserir carteira", q, c.ClienteID, c.VendedorID, c.DataInicio, c.DataFim)
	if db.ChaveDuplicada(err, qualquerIndice) {
		return comercial.ErrVinculoDuplicado
	}
	if err != nil {
		return err
	}
	c.CarteiraIDOrigem = id
	vlog.Printf(arqCarteiras, "Carteiras.Inserir", "INSERT carteiras id=%d", id)
	return nil
}

// Encerrar grava data_fim (troca de vendedor sem apagar o histórico).
func (r *Carteiras) Encerrar(ctx context.Context, id int64, fim time.Time) error {
	return db.ExecPorID(ctx, r.q, arqCarteiras, "Carteiras.Encerrar", "comercialmysql: encerrar vinculo carteira", comercial.ErrVinculoNaoEncontrado,
		`UPDATE carteiras SET data_fim = ? WHERE carteira_id_origem = ?`, fim, id)
}

// Reabrir limpa data_fim (revinculação no mesmo dia do encerramento).
func (r *Carteiras) Reabrir(ctx context.Context, id int64) error {
	return db.ExecPorID(ctx, r.q, arqCarteiras, "Carteiras.Reabrir", "comercialmysql: reativar vinculo carteira", comercial.ErrVinculoNaoEncontrado,
		`UPDATE carteiras SET data_fim = NULL WHERE carteira_id_origem = ?`, id)
}

// Excluir remove o vínculo (sem chamador HTTP; mantido com os testes do
// repositório legado).
func (r *Carteiras) Excluir(ctx context.Context, id int64) error {
	return db.ExecPorID(ctx, r.q, arqCarteiras, "Carteiras.Excluir", "comercialmysql: delete carteira", comercial.ErrVinculoNaoEncontrado,
		`DELETE FROM carteiras WHERE carteira_id_origem = ?`, id)
}

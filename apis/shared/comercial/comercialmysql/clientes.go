package comercialmysql

import (
	"context"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqClientes = "clientes.go"

// clienteColunas: bairro é NULLable e vira "" (COALESCE).
const clienteColunas = `cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, COALESCE(bairro, ''), data_cadastro, ativo, created_at, updated_at`

// clienteNaCarteiraCond restringe à carteira ATIVA de um vendedor; o
// vendedor_id vai sempre por placeholder.
const clienteNaCarteiraCond = "cliente_id_origem IN (SELECT cliente_id FROM carteiras WHERE vendedor_id = ? AND data_fim IS NULL)"

// uqClientesCNPJ é o índice único de clientes.cnpj (NEG-01).
const uqClientesCNPJ = "uq_clientes_cnpj"

// Clientes é o repositório de clientes (porta comercialapp.Clientes).
type Clientes struct{ q db.Querier }

// NovoClientes liga o repositório ao pool (ou a uma tx). q nil: panic.
func NovoClientes(q db.Querier) *Clientes {
	if q == nil {
		panic("comercialmysql: NovoClientes exige Querier")
	}
	return &Clientes{q: q}
}

// Listar pagina os clientes do escopo que casam com o filtro. No escopo de
// vendedor, a carteira ativa entra no WHERE por Escopo.Filtro (O1).
func (r *Clientes) Listar(ctx context.Context, e kernel.Escopo, f comercial.FiltroCliente, p kernel.Pagina) (kernel.Resultado[comercial.Cliente], error) {
	vendedorID, filtra, err := e.Filtro()
	if err != nil {
		return kernel.Resultado[comercial.Cliente]{}, err
	}
	w, args := whereCliente(f)
	if filtra {
		w, args = append(w, clienteNaCarteiraCond), append(args, vendedorID)
	}
	res, err := paginar(ctx, r.q, "clientes", "clientes", clienteColunas, db.Where(w), ordemCliente(f.Ordenacao), args, p, scanCliente)
	if err != nil {
		return res, err
	}
	vlog.Printf(arqClientes, "Clientes.Listar", "SELECT clientes linhas=%d total=%d carteira=%t", len(res.Itens), res.Total, filtra)
	return res, nil
}

// whereCliente traduz o filtro (C-03). Busca com máscara de CNPJ procura o
// CNPJ sem máscara (TermoBuscaCNPJ).
func whereCliente(f comercial.FiltroCliente) ([]string, []any) {
	var conds []string
	var args []any
	if f.UF != "" {
		conds, args = append(conds, "uf = ?"), append(args, f.UF)
	}
	if f.Segmento != "" {
		conds, args = append(conds, "segmento = ?"), append(args, f.Segmento)
	}
	if f.Ativo != nil {
		conds, args = append(conds, "ativo = ?"), append(args, *f.Ativo)
	}
	if f.Busca != "" {
		termoCNPJ := comercial.TermoBuscaCNPJ(f.Busca)
		if termoCNPJ == "" {
			termoCNPJ = f.Busca
		}
		conds = append(conds, "(razao_social LIKE ? OR cnpj LIKE ?)")
		args = append(args, "%"+f.Busca+"%", "%"+termoCNPJ+"%")
	}
	return conds, args
}

// PorID busca o cliente (ativo ou não); ErrClienteNaoEncontrado sem linha.
func (r *Clientes) PorID(ctx context.Context, id int64) (*comercial.Cliente, error) {
	q := "SELECT " + clienteColunas + " FROM clientes WHERE cliente_id_origem = ? LIMIT 1"
	c, err := scanCliente(r.q.QueryRowContext(ctx, q, id))
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqClientes, "Clientes.PorID", "SELECT clientes linhas=1")
	return c, nil
}

func scanCliente(s db.Linha) (*comercial.Cliente, error) {
	return db.LerUma(s, comercial.ErrClienteNaoEncontrado, "comercialmysql: ler cliente", func(c *comercial.Cliente) []any {
		return []any{&c.ClienteIDOrigem, &c.CNPJ, &c.RazaoSocial, &c.Segmento, &c.Cidade, &c.UF, &c.Bairro,
			&c.DataCadastro, &c.Ativo, &c.CreatedAt, &c.UpdatedAt}
	})
}

// Existe informa se há cliente com o id (ativo ou não).
func (r *Clientes) Existe(ctx context.Context, id int64) (bool, error) {
	return db.Existe(ctx, r.q, arqClientes, "Clientes.Existe", "comercialmysql: existe cliente", `SELECT 1 FROM clientes WHERE cliente_id_origem = ? LIMIT 1`, id)
}

// Inserir grava o cliente e preenche ClienteIDOrigem. O 1062 do CNPJ vira
// ErrCNPJDuplicado SEM o erro do driver, que traz o CNPJ (N-8, M-04).
func (r *Clientes) Inserir(ctx context.Context, c *comercial.Cliente) error {
	const q = `
		INSERT INTO clientes (cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	id, err := db.Inserir(ctx, r.q, "comercialmysql: inserir cliente", q, c.CNPJ, c.RazaoSocial, c.Segmento, c.Cidade, c.UF, c.Bairro, c.DataCadastro, c.Ativo)
	if db.ChaveDuplicada(err, uqClientesCNPJ) {
		return comercial.ErrCNPJDuplicado
	}
	if err != nil {
		return err
	}
	c.ClienteIDOrigem = id
	vlog.Printf(arqClientes, "Clientes.Inserir", "INSERT clientes id=%d", id)
	return nil
}

// Atualizar grava os campos editáveis (id e ativo não mudam por aqui).
func (r *Clientes) Atualizar(ctx context.Context, id int64, c *comercial.Cliente) error {
	const q = `
		UPDATE clientes
		SET cnpj = ?, razao_social = ?, segmento = ?, cidade = ?, uf = ?, bairro = ?, data_cadastro = ?
		WHERE cliente_id_origem = ?`
	err := db.ExecPorID(ctx, r.q, arqClientes, "Clientes.Atualizar", "comercialmysql: atualizar cliente", comercial.ErrClienteNaoEncontrado,
		q, c.CNPJ, c.RazaoSocial, c.Segmento, c.Cidade, c.UF, c.Bairro, c.DataCadastro, id)
	if db.ChaveDuplicada(err, uqClientesCNPJ) {
		return comercial.ErrCNPJDuplicado
	}
	return err
}

// DefinirAtivo ativa/inativa o cliente. Como no legado, um erro de
// RowsAffected conta como "nenhuma linha" (404).
func (r *Clientes) DefinirAtivo(ctx context.Context, id int64, ativo bool) error {
	res, err := r.q.ExecContext(ctx, `UPDATE clientes SET ativo = ? WHERE cliente_id_origem = ?`, ativo, id)
	if err != nil {
		return wrap("definir ativo cliente", err)
	}
	n, _ := res.RowsAffected()
	vlog.Printf(arqClientes, "Clientes.DefinirAtivo", "UPDATE clientes ativo afetadas=%d", n)
	if n == 0 {
		return comercial.ErrClienteNaoEncontrado
	}
	return nil
}

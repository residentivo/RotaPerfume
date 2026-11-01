package comercialapp

import (
	"context"
	"errors"
	"log"

	"github.com/rotaperfumes/shared/appkit"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const (
	arqClientes         = "clientes.go"
	rotuloClienteRelido = "[clientes] criado, mas falhou a releitura: cliente_id_origem"
)

// ListarClientes pagina os clientes do escopo (E-03): restrito vê só a
// carteira ativa (filtro forçado no SQL); sem carteira, 200 vazio sem SQL.
func (c *Casos) ListarClientes(ctx context.Context, p kernel.Principal, f comercial.FiltroCliente, pg kernel.Pagina) (r kernel.Resultado[comercial.Cliente], err error) {
	defer vlog.Trace(arqClientes, "Comercial.ListarClientes", "page=%d limit=%d busca=%t ordem=%s",
		pg.Numero(), pg.Limite(), f.Busca != "", f.Ordenacao.Campo())(&err)
	e, err := c.escopo.Resolver(ctx, p)
	if err != nil {
		return r, err
	}
	if e.SemAcesso() {
		return vazio[comercial.Cliente](pg), nil
	}
	return c.d.Clientes.Listar(ctx, e, f, pg)
}

// ObterCliente devolve o cliente; fora da carteira ou sem carteira responde
// como inexistente (E-04).
func (c *Casos) ObterCliente(ctx context.Context, p kernel.Principal, id int64) (cl *comercial.Cliente, err error) {
	defer vlog.Trace(arqClientes, "Comercial.ObterCliente", "id=%d", id)(&err)
	e, err := c.noEscopo(ctx, p, comercial.ErrClienteNaoEncontrado)
	if err != nil {
		return nil, err
	}
	if cl, err = c.d.Clientes.PorID(ctx, id); err != nil {
		return nil, err
	}
	if err = c.exigirNaCarteira(ctx, e, id); err != nil {
		return nil, err
	}
	return cl, nil
}

// CriarCliente cadastra o cliente (E-05). Sem carteira: 403 antes do
// decode. Restrito: cliente + vínculo com o próprio vendedor numa tx única
// (data_inicio = hoje). Admin: só o cliente, sem carteira.
func (c *Casos) CriarCliente(ctx context.Context, p kernel.Principal, ler Leitor[comercial.DadosCliente]) (cl *comercial.Cliente, err error) {
	defer vlog.Trace(arqClientes, "Comercial.CriarCliente", "user_id=%d", p.UserID())(&err)
	e, err := c.escopo.Resolver(ctx, p)
	if err != nil {
		return nil, err
	}
	// Política de escrita ANTES do decode: SemAcesso → 403 "usuário sem
	// vendedor vinculado"; escopo inválido segue como 500 (N5).
	vendedorID, restrito, err := comercial.VendedorDaEscrita(e, 0)
	if err != nil {
		return nil, err
	}
	d, err := ler()
	if err != nil {
		return nil, err
	}
	if cl, err = comercial.NovoCliente(d, c.agora()); err != nil {
		return nil, err
	}
	if err = c.gravarCliente(ctx, cl, vendedorID, restrito); err != nil {
		return nil, cnpjDuplicado(err)
	}
	logCriado(cl.ClienteIDOrigem, p, vendedorID, restrito)
	return appkit.RelerCriado(ctx, rotuloClienteRelido, cl.ClienteIDOrigem, cl, c.d.Clientes.PorID), nil
}

// gravarCliente: restrito grava cliente + vínculo na mesma tx; admin, só
// o cliente.
func (c *Casos) gravarCliente(ctx context.Context, cl *comercial.Cliente, vendedorID int64, restrito bool) error {
	if !restrito {
		return c.d.Clientes.Inserir(ctx, cl)
	}
	return c.d.ClienteNaCarteira.WithTx(ctx, func(r TxClienteNaCarteira) error {
		if err := r.Clientes.Inserir(ctx, cl); err != nil {
			return err
		}
		return r.Carteiras.Inserir(ctx, comercial.NovaCarteiraDoDia(cl.ClienteIDOrigem, vendedorID, c.agora()))
	})
}

func logCriado(id int64, p kernel.Principal, vendedorID int64, restrito bool) {
	if restrito {
		log.Printf("[clientes] criado: id=%d por usuario role=%s vendedor_id=%d carteira_vinculada=true", id, p.Role(), vendedorID)
		return
	}
	log.Printf("[clientes] criado: id=%d por usuario role=%s", id, p.Role())
}

// AtualizarCliente grava os campos editáveis. A posse é checada ANTES de
// ler o body (E-05): fora da carteira → 404 mesmo com body inválido.
func (c *Casos) AtualizarCliente(ctx context.Context, p kernel.Principal, id int64, ler Leitor[comercial.DadosCliente]) (cl *comercial.Cliente, err error) {
	defer vlog.Trace(arqClientes, "Comercial.AtualizarCliente", "id=%d", id)(&err)
	if err = c.autorizarEscritaCliente(ctx, p, id); err != nil {
		return nil, err
	}
	d, err := ler()
	if err != nil {
		return nil, err
	}
	if cl, err = comercial.ClienteEditado(d); err != nil {
		return nil, err
	}
	if err = c.d.Clientes.Atualizar(ctx, id, cl); err != nil {
		return nil, cnpjDuplicado(err)
	}
	log.Printf("[clientes] atualizado: id=%d por usuario role=%s", id, p.Role())
	return appkit.RelerAtualizado(ctx, "comercialapp", id, c.d.Clientes.PorID)
}

// AlternarAtivoCliente aplica o PATCH /inativar (posse antes do body; ativo
// nil = toggle).
func (c *Casos) AlternarAtivoCliente(ctx context.Context, p kernel.Principal, id int64, ler Leitor[*bool]) (cl *comercial.Cliente, err error) {
	defer vlog.Trace(arqClientes, "Comercial.AlternarAtivoCliente", "id=%d", id)(&err)
	if err = c.autorizarEscritaCliente(ctx, p, id); err != nil {
		return nil, err
	}
	ativo, err := ler()
	if err != nil {
		return nil, err
	}
	if cl, err = c.d.Clientes.PorID(ctx, id); err != nil {
		return nil, err
	}
	cl.DefinirAtivo(ativo)
	if err = c.d.Clientes.DefinirAtivo(ctx, id, cl.Ativo); err != nil {
		return nil, err
	}
	log.Printf("[clientes] ativo=%t: id=%d por usuario role=%s", cl.Ativo, id, p.Role())
	return cl, nil
}

// autorizarEscritaCliente: sem carteira ou cliente fora da carteira ativa
// → 404 "cliente não encontrado" (mesma mensagem de inexistente); admin
// sem restrição.
func (c *Casos) autorizarEscritaCliente(ctx context.Context, p kernel.Principal, id int64) error {
	e, err := c.noEscopo(ctx, p, comercial.ErrClienteNaoEncontrado)
	if err != nil {
		return err
	}
	return c.exigirNaCarteira(ctx, e, id)
}

// exigirNaCarteira traduz "fora da carteira" em ErrClienteNaoEncontrado.
func (c *Casos) exigirNaCarteira(ctx context.Context, e kernel.Escopo, clienteID int64) error {
	naCarteira, err := c.escopo.ClienteNaCarteira(ctx, e, clienteID)
	if err != nil {
		return err
	}
	if !naCarteira {
		vlog.Printf(arqClientes, "Comercial.exigirNaCarteira", "decisão fora_da_carteira cliente_id=%d", clienteID)
		return comercial.ErrClienteNaoEncontrado
	}
	return nil
}

// cnpjDuplicado loga a recusa do 1062 do CNPJ (sem o valor, M-04/E-07).
func cnpjDuplicado(err error) error {
	if errors.Is(err, comercial.ErrCNPJDuplicado) {
		log.Printf("[clientes] cnpj duplicado recusado (uq_clientes_cnpj)")
	}
	return err
}

// Package comercialapp tem os casos de uso do contexto Comercial (ARQ-04),
// o AutorizadorEscopo (o ÚNICO lugar que constrói kernel.Escopo, O5) e as
// portas que eles consomem. As portas são declaradas aqui, pelo consumidor;
// comercialmysql as satisfaz implicitamente. As portas para Identidade
// (VinculoUsuario, IdentidadeTx) são ligadas no bootstrap (§3.1).
package comercialapp

import (
	"context"
	"time"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
)

// Vendedores é o repositório de vendedores.
type Vendedores interface {
	// ListarResumo: Irrestrito → todos; Vendedor → só o próprio (filtro no
	// SQL, via Escopo.Filtro); SemAcesso/zero → erro sem SQL (O1).
	ListarResumo(ctx context.Context, e kernel.Escopo) ([]comercial.VendedorResumo, error)
	// PorID devolve comercial.ErrVendedorNaoEncontrado se o id não existir.
	PorID(ctx context.Context, id int64) (*comercial.Vendedor, error)
	Existe(ctx context.Context, id int64) (bool, error)
	// Desligado devolve comercial.ErrVendedorNaoEncontrado se não existir.
	Desligado(ctx context.Context, id int64) (bool, error)
	// Inserir grava e preenche v.ID.
	Inserir(ctx context.Context, v *comercial.Vendedor) error
	// Atualizar e DefinirDesligamento devolvem ErrVendedorNaoEncontrado se
	// nenhuma linha casar (clientFoundRows=true). quando nil limpa a data.
	Atualizar(ctx context.Context, id int64, v *comercial.Vendedor) error
	DefinirDesligamento(ctx context.Context, id int64, quando *time.Time) error
}

// Carteiras é o repositório dos vínculos cliente↔vendedor.
type Carteiras interface {
	ClientesDoVendedor(ctx context.Context, vendedorID int64) ([]comercial.ClienteResumo, error)
	// Os três "Vinculo*" devolvem comercial.ErrVinculoNaoEncontrado sem linha.
	VinculoAtivo(ctx context.Context, vendedorID, clienteID int64) (*comercial.Carteira, error)
	VinculoAtivoDoCliente(ctx context.Context, clienteID int64) (*comercial.Carteira, error)
	VinculoDoDia(ctx context.Context, clienteID, vendedorID int64, dia time.Time) (*comercial.Carteira, error)
	// Inserir grava e preenche c.CarteiraIDOrigem.
	Inserir(ctx context.Context, c *comercial.Carteira) error
	Encerrar(ctx context.Context, id int64, fim time.Time) error
	Reabrir(ctx context.Context, id int64) error
}

// Clientes é o repositório de clientes.
type Clientes interface {
	Listar(ctx context.Context, e kernel.Escopo, f comercial.FiltroCliente, p kernel.Pagina) (kernel.Resultado[comercial.Cliente], error)
	// PorID devolve comercial.ErrClienteNaoEncontrado se o id não existir.
	PorID(ctx context.Context, id int64) (*comercial.Cliente, error)
	Existe(ctx context.Context, id int64) (bool, error)
	// Inserir/Atualizar devolvem comercial.ErrCNPJDuplicado no 1062 do CNPJ.
	Inserir(ctx context.Context, c *comercial.Cliente) error
	Atualizar(ctx context.Context, id int64, c *comercial.Cliente) error
	DefinirAtivo(ctx context.Context, id int64, ativo bool) error
}

// Visitas é o repositório de visitas.
type Visitas interface {
	Listar(ctx context.Context, e kernel.Escopo, f comercial.FiltroVisita, p kernel.Pagina) (kernel.Resultado[comercial.Visita], error)
	// PorID devolve comercial.ErrVisitaNaoEncontrada se o id não existir.
	PorID(ctx context.Context, id int64) (*comercial.Visita, error)
	Inserir(ctx context.Context, v *comercial.Visita) error
	Atualizar(ctx context.Context, id int64, v *comercial.Visita) error
	Excluir(ctx context.Context, id int64) error
}

// Oportunidades é o repositório de oportunidades.
type Oportunidades interface {
	Listar(ctx context.Context, e kernel.Escopo, f comercial.FiltroOportunidade, p kernel.Pagina) (kernel.Resultado[comercial.Oportunidade], error)
	// PorID devolve comercial.ErrOportunidadeNaoEncontrada se não existir.
	PorID(ctx context.Context, id int64) (*comercial.Oportunidade, error)
	Inserir(ctx context.Context, o *comercial.Oportunidade) error
	Atualizar(ctx context.Context, id int64, o *comercial.Oportunidade) error
	Excluir(ctx context.Context, id int64) error
}

// VinculoUsuario é a porta para Identidade: o vendedor vinculado ao usuário
// (usuarios.id_vendedor). vinculado=false para usuário sem vínculo ou
// inexistente (o legado tratava os dois como "sem vendedor").
type VinculoUsuario interface {
	VendedorDoUsuario(ctx context.Context, usuarioID int64) (vendedorID int64, vinculado bool, err error)
}

// IdentidadeTx é a porta para Identidade DENTRO da tx do desligamento
// (I-09): inativa os usuários do vendedor gravando o corte de sessão e
// revoga os refresh tokens deles. Devolve as contagens (0 não é erro).
type IdentidadeTx interface {
	InativarUsuariosDoVendedor(ctx context.Context, vendedorID int64) (int64, error)
	RevogarSessoesDoVendedor(ctx context.Context, vendedorID int64) (int64, error)
}

// TxDesligamento são os repositórios ligados à tx do desligamento.
type TxDesligamento struct {
	Vendedores interface {
		MarcarDesligamento(ctx context.Context, id int64, quando time.Time) error
	}
	Identidade IdentidadeTx
}

// TxClienteNaCarteira são os repositórios ligados à tx do cadastro de
// cliente por usuário restrito (cliente + vínculo, E-05).
type TxClienteNaCarteira struct {
	Clientes interface {
		Inserir(ctx context.Context, c *comercial.Cliente) error
	}
	Carteiras interface {
		Inserir(ctx context.Context, c *comercial.Carteira) error
	}
}

// Transactor é o Unit of Work (db.Transactor satisfaz).
type Transactor[R any] interface {
	WithTx(ctx context.Context, fn func(R) error) error
}

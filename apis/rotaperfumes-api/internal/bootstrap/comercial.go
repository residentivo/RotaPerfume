package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/comercialhttp"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialapp"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
)

// NovoComercial liga o contexto Comercial (ARQ-04) sobre o pool: adapters
// MySQL, as portas para Identidade (vínculo do usuário e a parte de
// Identidade da tx do desligamento, I-09) e os dois UoW.
func NovoComercial(pool *sql.DB) *comercialapp.Casos {
	return comercialapp.Novo(comercialapp.Deps{
		Vendedores:    comercialmysql.NovoVendedores(pool),
		Carteiras:     comercialmysql.NovoCarteiras(pool),
		Clientes:      comercialmysql.NovoClientes(pool),
		Visitas:       comercialmysql.NovoVisitas(pool),
		Oportunidades: comercialmysql.NovoOportunidades(pool),
		Vinculo:       vinculoUsuario{usuarios: identidademysql.NovoUsuarios(pool)},
		Desligamento: db.NewTransactor(pool, func(q db.Querier) comercialapp.TxDesligamento {
			return comercialapp.TxDesligamento{
				Vendedores: comercialmysql.NovoVendedores(q),
				Identidade: identidadeNaTx{usuarios: identidademysql.NovoUsuarios(q), tokens: identidademysql.NovoRefreshTokens(q)},
			}
		}),
		ClienteNaCarteira: db.NewTransactor(pool, func(q db.Querier) comercialapp.TxClienteNaCarteira {
			return comercialapp.TxClienteNaCarteira{Clientes: comercialmysql.NovoClientes(q), Carteiras: comercialmysql.NovoCarteiras(q)}
		}),
		Relogio: time.Now,
	})
}

// ComercialHTTP são os handlers do contexto.
type ComercialHTTP struct {
	Vendedor     *comercialhttp.VendedorHandler
	Cliente      *comercialhttp.ClienteHandler
	Visita       *comercialhttp.VisitaHandler
	Oportunidade *comercialhttp.OportunidadeHandler
}

// NovoComercialHTTP monta os handlers sobre os casos de uso.
func NovoComercialHTTP(c *comercialapp.Casos) ComercialHTTP {
	return ComercialHTTP{
		Vendedor:     comercialhttp.NovoVendedorHandler(c),
		Cliente:      comercialhttp.NovoClienteHandler(c),
		Visita:       comercialhttp.NovoVisitaHandler(c),
		Oportunidade: comercialhttp.NovoOportunidadeHandler(c),
	}
}

// vinculoUsuario liga comercialapp.VinculoUsuario ao adapter de usuários:
// usuário inexistente conta como "sem vínculo" (SemAcesso), como no legado.
type vinculoUsuario struct{ usuarios *identidademysql.Usuarios }

func (v vinculoUsuario) VendedorDoUsuario(ctx context.Context, usuarioID int64) (int64, bool, error) {
	id, err := v.usuarios.VendedorDoUsuario(ctx, usuarioID)
	switch {
	case errors.Is(err, identidade.ErrUsuarioNaoEncontrado):
		return 0, false, nil
	case err != nil:
		return 0, false, err
	case id == nil:
		return 0, false, nil
	}
	return *id, true, nil
}

// identidadeNaTx é a parte de Identidade da tx do desligamento (I-09):
// inativa os usuários (com o corte de sessão) e revoga os refresh tokens
// com o motivo "inativacao", no MESMO *sql.Tx do vendedor.
type identidadeNaTx struct {
	usuarios *identidademysql.Usuarios
	tokens   *identidademysql.RefreshTokens
}

func (i identidadeNaTx) InativarUsuariosDoVendedor(ctx context.Context, vendedorID int64) (int64, error) {
	return i.usuarios.InativarDoVendedor(ctx, vendedorID)
}

func (i identidadeNaTx) RevogarSessoesDoVendedor(ctx context.Context, vendedorID int64) (int64, error) {
	return i.tokens.RevogarDoVendedor(ctx, vendedorID, identidade.MotivoInativacao)
}

// vendedoresParaIdentidade liga a porta identidadeapp.Vendedores (validação
// do id_vendedor de usuário) ao adapter de Comercial.
type vendedoresParaIdentidade struct{ vendedores *comercialmysql.Vendedores }

func (v vendedoresParaIdentidade) Existe(ctx context.Context, id int64) (bool, error) {
	return v.vendedores.Existe(ctx, id)
}

// Desligado: vendedor inexistente (vínculo órfão) não é desligado.
func (v vendedoresParaIdentidade) Desligado(ctx context.Context, id int64) (bool, error) {
	desligado, err := v.vendedores.Desligado(ctx, id)
	if errors.Is(err, comercial.ErrVendedorNaoEncontrado) {
		return false, nil
	}
	return desligado, err
}

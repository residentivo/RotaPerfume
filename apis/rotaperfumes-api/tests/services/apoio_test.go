package services_test

// ARQ-07 (pendência i): os testes de pedidos e pagamentos chamam os casos de
// uso de vendasapp diretamente (os repassadores PedidoService/
// PagamentoService saíram). Ficam aqui a montagem pelo bootstrap e as
// expectativas de SQL comuns.

import (
	"context"
	"database/sql"
	"errors"
	"github.com/rotaperfumes/shared/adapters/ratelimit"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialapp"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	"github.com/rotaperfumes/shared/config"
	shareddb "github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
)

// casosVendas monta os casos de uso pelo bootstrap (adapters reais sobre o
// sqlmock); vendedorID > 0 liga o vínculo fixo do usuário restrito.
func casosVendas(db *sql.DB, vendedorID int64) *vendasapp.Casos {
	return bootstrap.NovoVendas(db, casosComercial(db, vendedorID).Escopo())
}

// lerFixo é o Leitor do comando já decodificado (o handler decodifica o body).
func lerFixo[T any](v T) vendasapp.Leitor[T] { return func() (T, error) { return v, nil } }

// itensETotal devolve itens, total e erro de uma listagem paginada.
func itensETotal[T any](r kernel.Resultado[T], err error) ([]T, int, error) {
	return r.Itens, r.Total, err
}

// esperarPossePagamento: idem para o pagamento.
func esperarPossePagamento(m sqlmock.Sqlmock, id int64) {
	m.ExpectQuery(`SELECT ` + pagamentoColunasRegex + pagamentoFromRegex + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(id).WillReturnRows(pagamentoRows(id, 1, 100.0))
}

// novosCasosCatalogo monta os casos de uso de Catálogo (catalogoapp) sobre os
// adapters reais (catalogomysql) e o db do sqlmock, como o bootstrap.
func novosCasosCatalogo(db *sql.DB) *catalogoapp.Casos {
	return catalogoapp.Novo(catalogomysql.NovoProdutos(db), catalogomysql.NovoEstoques(db))
}

// ---- Comercial (ARQ-07: os repassadores ClienteService/VendedorService/
// VisitaService/OportunidadeService saíram; ficam a montagem dos casos de
// uso com os dublês sem SQL e as expectativas de posse) ----

// vinculoFixo é o vínculo usuário→vendedor do usuário restrito (sem SQL).
type vinculoFixo struct{ vendedorID int64 }

func (v vinculoFixo) VendedorDoUsuario(context.Context, int64) (int64, bool, error) {
	return v.vendedorID, v.vendedorID > 0, nil
}

// vendedoresAtivos não consulta o desligamento (o resto é o adapter real).
type vendedoresAtivos struct{ comercialapp.Vendedores }

func (vendedoresAtivos) Desligado(context.Context, int64) (bool, error) { return false, nil }

// casosComercial monta os casos de uso sobre o db; vendedorID > 0 liga o
// vínculo fixo do usuário restrito.
func casosComercial(db *sql.DB, vendedorID int64) *comercialapp.Casos {
	return comercialapp.Novo(comercialapp.Deps{
		Vendedores:    vendedoresAtivos{comercialmysql.NovoVendedores(db)},
		Carteiras:     comercialmysql.NovoCarteiras(db),
		Clientes:      comercialmysql.NovoClientes(db),
		Visitas:       comercialmysql.NovoVisitas(db),
		Oportunidades: comercialmysql.NovoOportunidades(db),
		Vinculo:       vinculoFixo{vendedorID: vendedorID},
		Desligamento: shareddb.NewTransactor(db, func(q shareddb.Querier) comercialapp.TxDesligamento {
			return comercialapp.TxDesligamento{Vendedores: comercialmysql.NovoVendedores(q), Identidade: identidadeNaTx{q: q}}
		}),
		ClienteNaCarteira: shareddb.NewTransactor(db, func(q shareddb.Querier) comercialapp.TxClienteNaCarteira {
			return comercialapp.TxClienteNaCarteira{Clientes: comercialmysql.NovoClientes(q), Carteiras: comercialmysql.NovoCarteiras(q)}
		}),
		Vinculacao: shareddb.NewTransactor(db, func(q shareddb.Querier) comercialapp.TxVinculo {
			return comercialapp.TxVinculo{Clientes: comercialmysql.NovoClientes(q), Carteiras: comercialmysql.NovoCarteiras(q)}
		}),
		Relogio: time.Now,
	})
}

// identidadeNaTx é a parte de Identidade da tx do desligamento (I-09), como
// no bootstrap.
type identidadeNaTx struct{ q shareddb.Querier }

func (i identidadeNaTx) InativarUsuariosDoVendedor(ctx context.Context, vendedorID int64) (int64, error) {
	return identidademysql.NovoUsuarios(i.q).InativarDoVendedor(ctx, vendedorID)
}

func (i identidadeNaTx) RevogarSessoesDoVendedor(ctx context.Context, vendedorID int64) (int64, error) {
	return identidademysql.NovoRefreshTokens(i.q).RevogarDoVendedor(ctx, vendedorID, "inativacao")
}

func principalAdmin() kernel.Principal {
	p, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	return p
}

func principalRestrito() kernel.Principal {
	p, _ := kernel.NovoPrincipal(7, kernel.RoleNormal)
	return p
}

// fixo é o Leitor de um comando já pronto.
func fixo[T any](v T) comercialapp.Leitor[T] { return func() (T, error) { return v, nil } }

// ARQ-04: editar ou excluir visita/oportunidade lê o registro antes, para
// checar a posse (E-04). No legado essa leitura ficava no handler (que os
// testes de handler já esperam); nos testes de service ela entra por estes
// helpers, uma linha antes das expectativas de cada caso.
func esperarPosseVisita(m sqlmock.Sqlmock, id int64) {
	m.ExpectQuery(`SELECT ` + visitaColunasRegex + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(id).WillReturnRows(visitaRows())
}

func esperarPosseOportunidade(m sqlmock.Sqlmock, id int64) {
	m.ExpectQuery(`SELECT ` + oportunidadeColunasRegex + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(id).WillReturnRows(oportunidadeRows())
}

// ---- Identidade (ARQ-07: os repassadores UsuarioService/RefreshTokenService/
// SenhaHistoricoService/ResetSenhasAtivosService saíram; fica a montagem dos
// casos de uso sobre os adapters reais e o sqlmock) ----

var cfgIdentidadeTeste = &config.Config{
	JWTSecret: "segredo-teste-servicos-com-pelo-menos-32-bytes", JWTIssuer: "rotaperfumes-test", JWTTTL: time.Hour,
	HashSenha: config.HashSenha{Pepper: "pepper-de-teste-com-pelo-menos-32-bytes", MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1},
}

type captchaAprova struct{}

func (captchaAprova) Verificar(context.Context, string, string) error { return nil }

// vendedoresDoComercial: a porta identidadeapp.Vendedores sobre o adapter de
// Comercial (ARQ-04; como o bootstrap).
type vendedoresDoComercial struct{ db *sql.DB }

func (v vendedoresDoComercial) Existe(ctx context.Context, id int64) (bool, error) {
	return comercialmysql.NovoVendedores(v.db).Existe(ctx, id)
}

func (v vendedoresDoComercial) Desligado(ctx context.Context, id int64) (bool, error) {
	d, err := comercialmysql.NovoVendedores(v.db).Desligado(ctx, id)
	if errors.Is(err, comercial.ErrVendedorNaoEncontrado) {
		return false, nil
	}
	return d, err
}

func novosCasos(db *sql.DB, cfg *config.Config, email identidadeapp.EnviadorSenhaInicial, agora func() time.Time, o identidadeapp.Opcoes) *identidadeapp.Casos {
	if cfg == nil {
		cfg = cfgIdentidadeTeste
	}
	if email == nil {
		email = &fakeEmailService{}
	}
	return identidadeapp.Novo(identidadeapp.Deps{
		Usuarios:  identidademysql.NovoUsuarios(db),
		Tokens:    identidademysql.NovoRefreshTokens(db),
		Historico: identidademysql.NovoHistoricoSenhas(db),
		Rotacao: shareddb.NewTransactor(db, func(q shareddb.Querier) identidadeapp.TxRotacao {
			return identidadeapp.TxRotacao{Tokens: identidademysql.NovoRefreshTokens(q)}
		}),
		TrocaSenha: shareddb.NewTransactor(db, func(q shareddb.Querier) identidadeapp.TxSenha {
			return identidadeapp.TxSenha{Usuarios: identidademysql.NovoUsuarios(q), Historico: identidademysql.NovoHistoricoSenhas(q),
				Tokens: identidademysql.NovoRefreshTokens(q)}
		}),
		Hasher:     identidadecrypto.NovoHasher(cfg.HashSenha),
		Emissor:    identidadecrypto.NovoJWT(cfg),
		Refresh:    identidadecrypto.GeradorRefresh{},
		Senhas:     identidadecrypto.GeradorSenha{},
		Captcha:    captchaAprova{},
		Limites:    identidadeapp.Limites{Login: ratelimit.Nova(5, time.Minute, time.Minute), Refresh: ratelimit.Nova(10, time.Minute, time.Minute), Reset: ratelimit.Nova(10, time.Minute, time.Minute)},
		Publicador: identidadeapp.SemPublicador{},
		Email:      email,
		Vendedores: vendedoresDoComercial{db: db},
		Relogio:    agora,
	}, o)
}

// adminTeste é o Principal admin dos casos de uso de gestão (I-05).
func adminTeste() kernel.Principal {
	p, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	return p
}

// ordemUsuario e ordemHistorico: o parse de order_by/order_dir do HTTP.
func ordemUsuario(campo, dir string) kernel.Ordenacao[identidade.CampoUsuario] {
	return kernel.ParseOrdenacao(campo, dir, identidade.CamposOrdenacaoUsuario(), identidade.OrdenacaoUsuarioPadrao())
}

func ordemHistorico(campo, dir string) kernel.Ordenacao[identidade.CampoHistorico] {
	return kernel.ParseOrdenacao(campo, dir, identidade.CamposOrdenacaoHistorico(), identidade.OrdenacaoHistoricoPadrao())
}

// verificarSenha / precisaRehash: o hasher de produção (ops01).
func verificarSenha(hs config.HashSenha, hash, senha string) bool {
	return identidadecrypto.NovoHasher(hs).Verificar(identidade.NovoHashSenha(hash), identidade.NovaSenha(senha))
}

func precisaRehash(hs config.HashSenha, hash string) bool {
	return identidadecrypto.NovoHasher(hs).PrecisaRehash(identidade.NovoHashSenha(hash))
}

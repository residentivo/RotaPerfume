// Package identidadeapp tem os casos de uso de Identidade e Acesso (ARQ-03)
// e as portas que eles consomem. As portas são declaradas aqui, pelo
// consumidor; identidademysql, identidadecrypto, adapters/turnstile e
// adapters/ratelimit as satisfazem implicitamente, sem importar este pacote
// (docs/arquitetura-ddd.md §4.1).
package identidadeapp

import (
	"context"
	"time"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

// Usuarios é o repositório de usuários. "Não encontrado" é sempre
// identidade.ErrUsuarioNaoEncontrado.
type Usuarios interface {
	PorEmail(ctx context.Context, email string) (*identidade.Usuario, error)
	PorID(ctx context.Context, id int64) (*identidade.Usuario, error)
	// Status é a consulta enxuta de cada request protegido (SEC-06/08).
	StatusParaAutenticacao(ctx context.Context, id int64) (identidade.StatusAcesso, error)
	Listar(ctx context.Context, o kernel.Ordenacao[identidade.CampoUsuario], p kernel.Pagina) (kernel.Resultado[identidade.Usuario], error)
	ListarAtivos(ctx context.Context) ([]identidade.Usuario, error)
	// Inserir preenche u.ID; e-mail duplicado → identidade.ErrEmailDuplicado.
	Inserir(ctx context.Context, u *identidade.Usuario) error
	Atualizar(ctx context.Context, id int64, v identidade.UsuarioValidado) error
	// DefinirAtivo(false) grava também o corte de sessão (I-04).
	DefinirAtivo(ctx context.Context, id int64, ativo bool) error
	// AtualizarSenha grava hash, deve_trocar_senha e o corte de sessão (I-04).
	AtualizarSenha(ctx context.Context, id int64, h identidade.HashSenha, deveTrocarSenha bool) error
	// RegravarHash troca o hash só se ainda for antigo (condição no WHERE);
	// não mexe no corte de sessão (SEC-13).
	RegravarHash(ctx context.Context, id int64, antigo, novo identidade.HashSenha) error
	RegistrarLogin(ctx context.Context, id int64, quando time.Time) error
	InvalidarSessoes(ctx context.Context, id int64, corte time.Time) error
}

// RefreshTokens é o repositório de refresh tokens.
type RefreshTokens interface {
	// PorHash devolve identidade.ErrRefreshInvalido se o hash não existir.
	PorHash(ctx context.Context, hash string) (*identidade.RefreshToken, error)
	// Inserir preenche rt.ID.
	Inserir(ctx context.Context, rt *identidade.RefreshToken) error
	// Revogar é o UPDATE condicional (revoked_at IS NULL): sem linha afetada
	// devolve identidade.ErrRefreshRevogado (corrida, F-07).
	Revogar(ctx context.Context, id int64, m identidade.MotivoRevogacao) error
	RevogarTodosDoUsuario(ctx context.Context, usuarioID int64, m identidade.MotivoRevogacao) error
	// MarcarReuso é o UPDATE condicional atômico do SEC-12: true = marcou.
	MarcarReuso(ctx context.Context, id int64, agora, limite time.Time) (bool, error)
	DesfazerReuso(ctx context.Context, id int64, marca time.Time) error
	ApagarExpirados(ctx context.Context, corte time.Time, lote int) (int64, error)
}

// HistoricoSenhas é o repositório de senha_historico.
type HistoricoSenhas interface {
	Registrar(ctx context.Context, r *identidade.RegistroSenha) error
	Todos(ctx context.Context, o kernel.Ordenacao[identidade.CampoHistorico], p kernel.Pagina) (kernel.Resultado[identidade.RegistroSenha], error)
	DoUsuario(ctx context.Context, usuarioID int64, o kernel.Ordenacao[identidade.CampoHistorico], p kernel.Pagina) (kernel.Resultado[identidade.RegistroSenha], error)
}

// TxRotacao são os repositórios ligados à transação da rotação do refresh
// (I-06): dentro do WithTx só eles são usados (O8-ii).
type TxRotacao struct{ Tokens RefreshTokens }

// Transactor é o Unit of Work da rotação (db.Transactor[TxRotacao]).
type Transactor interface {
	WithTx(ctx context.Context, fn func(TxRotacao) error) error
}

// Hasher é a Strategy de hash de senha (Argon2id+pepper / bcrypt legado).
type Hasher interface {
	Gerar(s identidade.Senha) (identidade.HashSenha, error)
	Verificar(h identidade.HashSenha, s identidade.Senha) bool
	PrecisaRehash(h identidade.HashSenha) bool
	// ExecutarDescartavel gasta o custo de um hash (anti-enumeração, I-16).
	ExecutarDescartavel(s identidade.Senha)
}

// EmissorAcesso emite o access token (JWT HS256).
type EmissorAcesso interface {
	Emitir(userID int64, role kernel.Role) (string, error)
}

// GeradorRefresh gera o token opaco e o hash que vai para o banco.
type GeradorRefresh interface {
	Novo() identidade.TokenOpaco
	Hash(t identidade.TokenOpaco) string
}

// GeradorSenha gera a senha aleatória de criação/reset de usuário.
type GeradorSenha interface {
	Gerar() (identidade.Senha, error)
}

// VerificadorCaptcha confere o token do Turnstile. Qualquer erro recusa
// (fail-closed, I-13).
type VerificadorCaptcha interface {
	Verificar(ctx context.Context, token, ip string) error
}

// Limitador é o rate limit em memória por chave (I-14).
type Limitador interface {
	Bloqueado(chave string) (bool, time.Duration)
	RegistrarFalha(chave string)
	RegistrarSucesso(chave string)
}

// Limites são os três limitadores, singletons por processo (I-14).
type Limites struct {
	Login, Refresh, Reset Limitador
}

// Publicador entrega o evento de reuso aos assinantes (Observer, §5.12).
type Publicador interface {
	Publicar(ctx context.Context, e identidade.ReusoDeRefreshDetectado)
}

// EnviadorSenhaInicial envia por e-mail a senha gerada pelo sistema (o
// EmailService legado satisfaz). A senha nunca é logada nem devolvida (M-02).
type EnviadorSenhaInicial interface {
	EnviarSenhaInicial(ctx context.Context, destinatario, nomeUsuario, senha string) error
}

// Vendedores é a porta para o contexto Comercial (ligada no bootstrap).
type Vendedores interface {
	Existe(ctx context.Context, id int64) (bool, error)
	// Desligado devolve false, sem erro, para vendedor inexistente.
	Desligado(ctx context.Context, id int64) (bool, error)
}

// Origem identifica de onde veio o pedido (auditoria e chaves de rate
// limit). Nunca vai para o vlog (§6.3).
type Origem struct {
	IP        string
	UserAgent string
}

// Package identidade é o domínio do contexto Identidade e Acesso (ARQ-03):
// Usuario, os VOs de segredo (Senha, HashSenha, TokenOpaco), a política de
// senha, o RefreshToken com as regras de expiração/revogação/reuso, o
// histórico de senhas e o evento de reuso de refresh token.
//
// O pacote é puro (docs/arquitetura-ddd.md §4.1): importa só a stdlib e o
// kernel. Não loga, não conhece SQL, HTTP nem criptografia, e não tem tags
// JSON (o DTO HTTP é que serializa).
package identidade

import "github.com/rotaperfumes/shared/kernel"

// Erros de domínio. A mensagem é a resposta HTTP pública: as mesmas do
// legado (auth_handler.go, usuario_handler.go, password_policy.go), sem
// valores.
var (
	ErrEmailESenhaObrigatorios = kernel.NovoErro(kernel.KindInvalido, "email e senha são obrigatórios")
	ErrCredenciaisInvalidas    = kernel.NovoErro(kernel.KindNaoAutenticado, "credenciais inválidas")
	// Usuário inativo no login responde ErrCredenciaisInvalidas e no refresh
	// ErrRefreshRevogado (SEC-22): a API nunca informa "conta inativa".
	ErrCaptchaInvalido  = kernel.NovoErro(kernel.KindInvalido, "captcha inválido ou ausente")
	ErrMuitasTentativas = kernel.NovoErro(kernel.KindBloqueado, "muitas tentativas — tente novamente mais tarde")
	// ErrGerarToken e ErrGerarAccessToken são 500 com mensagem própria (o
	// legado não usava "erro interno" nesses dois pontos).
	ErrGerarToken       = kernel.NovoErro(kernel.KindInterno, "erro ao gerar token")
	ErrGerarAccessToken = kernel.NovoErro(kernel.KindInterno, "erro ao gerar access token")

	ErrRefreshObrigatorio = kernel.NovoErro(kernel.KindInvalido, "refresh_token é obrigatório (body ou cookie)")
	// ErrRefreshInvalido: token inexistente (o adapter devolve este sentinela).
	ErrRefreshInvalido = kernel.NovoErro(kernel.KindNaoAutenticado, "refresh token inválido")
	ErrRefreshExpirado = kernel.NovoErro(kernel.KindNaoAutenticado, "refresh token expirado")
	// ErrRefreshRevogado: revogado (dentro ou fora da janela de graça) e a
	// corrida na rotação (UPDATE condicional sem linha afetada).
	ErrRefreshRevogado = kernel.NovoErro(kernel.KindNaoAutenticado, "refresh token revogado")

	ErrSenhaAtualObrigatoria = kernel.NovoErro(kernel.KindInvalido, "senha_atual é obrigatória")
	ErrNovaSenhaObrigatoria  = kernel.NovoErro(kernel.KindInvalido, "nova_senha é obrigatória")
	// ErrSenhaAtualIncorreta é 400, não 401 (BUG-03): o usuário está
	// autenticado e o frontend trata todo 401 como sessão expirada.
	ErrSenhaAtualIncorreta = kernel.NovoErro(kernel.KindInvalido, "senha atual incorreta")
	ErrNovaSenhaIgualAtual = kernel.NovoErro(kernel.KindInvalido, "a nova senha não pode ser igual à senha atual")
	ErrSenhaReutilizada    = kernel.NovoErro(kernel.KindInvalido, "a nova senha não pode ser igual a uma das últimas senhas utilizadas")
	ErrSenhaCurta          = kernel.NovoErro(kernel.KindInvalido, "senha deve ter pelo menos 8 caracteres")
	ErrSenhaMuitoLonga     = kernel.NovoErro(kernel.KindInvalido, "senha excede o tamanho máximo permitido (72 bytes)")
	ErrSenhaFraca          = kernel.NovoErro(kernel.KindInvalido, "senha deve conter ao menos 3 dos 4 tipos: letra minúscula, letra maiúscula, dígito e símbolo")

	ErrUsuarioNaoEncontrado = kernel.NovoErro(kernel.KindNaoEncontrado, "usuário não encontrado")
	// ErrAcessoRestritoAdmin: defesa em profundidade dos casos de uso de
	// gestão de usuários (I-05), além da política AdminOnly da rota.
	ErrAcessoRestritoAdmin   = kernel.NovoErro(kernel.KindProibido, "acesso restrito a administradores")
	ErrNomeObrigatorio       = kernel.NovoErro(kernel.KindInvalido, "nome é obrigatório")
	ErrEmailObrigatorio      = kernel.NovoErro(kernel.KindInvalido, "email é obrigatório")
	ErrRoleInvalido          = kernel.NovoErro(kernel.KindInvalido, "role deve ser 'admin' ou 'normal'")
	ErrVendedorInexistente   = kernel.NovoErro(kernel.KindInvalido, "vendedor não encontrado")
	ErrEmailDuplicado        = kernel.NovoErro(kernel.KindConflito, "email já cadastrado")
	ErrUsuarioAlvoInvalido   = kernel.NovoErro(kernel.KindInvalido, "usuario_id é obrigatório e deve ser > 0")
	ErrHistoricoSemUsuarioID = kernel.NovoErro(kernel.KindInvalido, "usuario_id é obrigatório na URL")
	ErrHistoricoUsuarioID    = kernel.NovoErro(kernel.KindInvalido, "usuario_id inválido")
)

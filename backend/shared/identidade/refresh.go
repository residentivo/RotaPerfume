package identidade

import "time"

// Regras do refresh token (I-06), inalteradas desde o legado.
const (
	// TTLRefresh é a validade do refresh token.
	TTLRefresh = 7 * 24 * time.Hour
	// JanelaGracaRevogacao (SEC-04): um token revogado há no máximo esse
	// tempo (em qualquer direção, tolerando relógios diferentes entre API e
	// banco) é tratado como corrida legítima de rotação, não como reuso.
	JanelaGracaRevogacao = 30 * time.Second
)

// MotivoRevogacao espelha o ENUM de refresh_tokens.revoked_reason (SEC-07).
type MotivoRevogacao string

// Motivos de revogação.
const (
	MotivoRotacao        MotivoRevogacao = "rotacao"
	MotivoLogout         MotivoRevogacao = "logout"
	MotivoRevogacaoMassa MotivoRevogacao = "revogacao_massa"
	MotivoSenha          MotivoRevogacao = "senha"
	MotivoInativacao     MotivoRevogacao = "inativacao"
	// MotivoDesconhecido é revoked_reason NULL em token revogado (legado,
	// anterior ao SEC-07): tratado de forma conservadora, nunca como reuso.
	MotivoDesconhecido MotivoRevogacao = ""
)

// RefreshToken é um registro de refresh_tokens. O banco guarda só o hash
// (SHA-256) do TokenOpaco.
type RefreshToken struct {
	ID         int64
	UsuarioID  int64
	HashToken  string
	ExpiraEm   time.Time
	RevogadoEm *time.Time
	Motivo     MotivoRevogacao
	IP         string
	UserAgent  string
}

// NovoRefreshToken monta o registro de um token recém-gerado, válido por
// TTLRefresh a partir de agora.
func NovoRefreshToken(usuarioID int64, hashToken string, agora time.Time, ip, userAgent string) *RefreshToken {
	return &RefreshToken{
		UsuarioID: usuarioID,
		HashToken: hashToken,
		ExpiraEm:  agora.Add(TTLRefresh),
		IP:        ip,
		UserAgent: userAgent,
	}
}

// EstadoRefresh é o resultado de RefreshToken.Avaliar. A decisão de fluxo
// (I-07) é por este enum, não por cadeia de erros: RevogadoRecente nunca é
// confundido com Revogado.
type EstadoRefresh uint8

// Estados do refresh token.
const (
	RefreshValido EstadoRefresh = iota + 1
	RefreshExpirado
	// RefreshRevogadoRecente: revogado dentro da JanelaGracaRevogacao.
	RefreshRevogadoRecente
	// RefreshRevogado: revogado fora da janela (possível reuso).
	RefreshRevogado
)

// Avaliar classifica o token em agora. A expiração é checada ANTES da
// revogação (I-06): um token expirado é sempre Expirado.
func (t *RefreshToken) Avaliar(agora time.Time) EstadoRefresh {
	if agora.After(t.ExpiraEm) {
		return RefreshExpirado
	}
	if t.RevogadoEm == nil {
		return RefreshValido
	}
	idade := agora.Sub(*t.RevogadoEm)
	if idade >= -JanelaGracaRevogacao && idade <= JanelaGracaRevogacao {
		return RefreshRevogadoRecente
	}
	return RefreshRevogado
}

// ReusoDeRotacao reporta o sinal de possível roubo (SEC-07): token
// revogado por rotação apresentado fora da janela de graça. Motivo NULL
// (legado) não conta.
func (t *RefreshToken) ReusoDeRotacao(e EstadoRefresh) bool {
	return e == RefreshRevogado && t.Motivo == MotivoRotacao
}

// ReusoDeRefreshDetectado é o evento publicado depois do corte de sessões
// por reuso de refresh token rotacionado (SEC-07/SEC-09). O assinante atual
// é o notificador de alerta de segurança.
type ReusoDeRefreshDetectado struct {
	UsuarioID int64
	TokenID   int64
	IP        string
	UserAgent string
}

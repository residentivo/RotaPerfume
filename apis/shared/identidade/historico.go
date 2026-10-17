package identidade

import (
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// TipoReset é a origem de uma troca de senha (senha_historico.tipo_reset).
type TipoReset string

// Tipos de troca de senha.
const (
	TipoResetUsuario        TipoReset = "usuario"
	TipoResetAdmin          TipoReset = "admin"
	TipoResetPrimeiroAcesso TipoReset = "primeiro_acesso"
	TipoResetEsquecimento   TipoReset = "esquecimento"
)

// UltimasSenhasNoHistorico é quantos registros do histórico, além do hash
// atual, entram no bloqueio de reuso (atual + 2 últimas, I-12).
const UltimasSenhasNoHistorico = 2

// RegistroSenha é uma linha de senha_historico. O hash anterior fica só para
// o bloqueio de reuso: nunca vai para a resposta HTTP.
type RegistroSenha struct {
	ID              int64
	UsuarioID       int64
	UsuarioNome     string
	ResetadoPorID   *int64
	ResetadoPorNome *string
	HashAnterior    HashSenha
	IP              string
	UserAgent       string
	Tipo            TipoReset
	CriadoEm        time.Time
}

// VerificadorSenha confere uma senha contra um hash (o hasher de
// identidadecrypto satisfaz).
type VerificadorSenha interface {
	Verificar(h HashSenha, s Senha) bool
}

// SenhaReutilizada reporta se candidata confere com algum dos hashes (o
// atual e as últimas do histórico). Avalia TODOS os hashes, sem
// short-circuit: o tempo não revela qual deles casou (I-12). Nada de
// slices.ContainsFunc/any com retorno antecipado.
func SenhaReutilizada(candidata Senha, hashes []HashSenha, v VerificadorSenha) bool {
	reutilizada := false
	for _, h := range hashes {
		if v.Verificar(h, candidata) {
			reutilizada = true
		}
	}
	return reutilizada
}

// CampoHistorico é o enum de ordenação do histórico de senhas (C-02, FE-13).
type CampoHistorico string

// Campos de ordenação do histórico.
const (
	OrdemHistoricoID              CampoHistorico = "id"
	OrdemHistoricoUsuarioID       CampoHistorico = "usuario_id"
	OrdemHistoricoTipo            CampoHistorico = "tipo_reset"
	OrdemHistoricoCriadoEm        CampoHistorico = "created_at"
	OrdemHistoricoUsuarioNome     CampoHistorico = "usuario_nome"
	OrdemHistoricoResetadoPorNome CampoHistorico = "resetado_por_nome"
	OrdemHistoricoIP              CampoHistorico = "ip_origem"
)

// CamposOrdenacaoHistorico devolve a whitelist do histórico.
func CamposOrdenacaoHistorico() []CampoHistorico {
	return []CampoHistorico{
		OrdemHistoricoID, OrdemHistoricoUsuarioID, OrdemHistoricoTipo, OrdemHistoricoCriadoEm,
		OrdemHistoricoUsuarioNome, OrdemHistoricoResetadoPorNome, OrdemHistoricoIP,
	}
}

// OrdenacaoHistoricoPadrao é "id DESC" (mais recente primeiro).
func OrdenacaoHistoricoPadrao() kernel.Ordenacao[CampoHistorico] {
	return kernel.NovaOrdenacao(OrdemHistoricoID, kernel.Desc)
}

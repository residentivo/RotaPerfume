package identidadeapp

import (
	"context"
	"errors"
	"fmt"

	"github.com/rotaperfumes/shared/vlog"
)

// ErrEmailResetFalhou: o e-mail com a nova senha de um usuário não saiu. O
// reset dele já foi gravado; o lote para ali (OPS-01) para não trancar os
// demais se o SMTP caiu.
var ErrEmailResetFalhou = errors.New("reset em massa: e-mail com a nova senha não foi enviado")

// Origem gravada em senha_historico pelo reset em massa.
const (
	ResetMassaIPOrigem  = "127.0.0.1"
	ResetMassaUserAgent = "cmd/resetsenhas (OPS-01)"
)

// Status de cada usuário no reset em massa.
const (
	StatusSimulado   = "simulado"
	StatusResetado   = "resetado"
	StatusFalhaEmail = "falha_email"
)

// ItemResetMassa é o resultado de um usuário no reset em massa.
type ItemResetMassa struct {
	ID     int64
	Email  string
	Role   string
	Status string
}

// origemResetMassa é a Origem gravada no histórico pelo reset em massa.
var origemResetMassa = Origem{IP: ResetMassaIPOrigem, UserAgent: ResetMassaUserAgent}

// ResetarSenhasAtivos redefine a senha de cada usuário ativo, em ordem de
// id, com o mesmo fluxo do reset administrativo (CLI cmd/resetsenhas,
// OPS-01): uma tx por usuário (BUG-15) e o e-mail depois do commit. Caso
// de uso de CLI: NÃO recebe Principal (O5). simular=true não grava nem
// envia. Para na primeira tx ou no primeiro e-mail que falhar (o usuário
// da tx que falhou não recebe e-mail).
func (c *Casos) ResetarSenhasAtivos(ctx context.Context, simular bool) (itens []ItemResetMassa, err error) {
	defer vlog.Trace(arqSenha, "Identidade.ResetarSenhasAtivos", "simular=%t", simular)(&err)
	ativos, err := c.d.Usuarios.ListarAtivos(ctx)
	if err != nil {
		return nil, err
	}
	itens = make([]ItemResetMassa, 0, len(ativos))
	for i := range ativos {
		u := &ativos[i]
		item := ItemResetMassa{ID: u.ID, Email: u.Email, Role: u.Role.String(), Status: StatusSimulado}
		if simular {
			itens = append(itens, item)
			continue
		}
		enviado, err := c.redefinirSenhaGerada(ctx, u.ID, u.Email, u.Nome, nil, origemResetMassa)
		if err != nil {
			return itens, fmt.Errorf("reset em massa: usuario_id=%d: %w", u.ID, err)
		}
		if !enviado {
			item.Status = StatusFalhaEmail
			return append(itens, item), fmt.Errorf("%w: usuario_id=%d", ErrEmailResetFalhou, u.ID)
		}
		item.Status = StatusResetado
		itens = append(itens, item)
	}
	return itens, nil
}

package comercialmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqVendedores = "vendedores.go"

// vendedorColunas são as colunas de PorID (ordem do scan).
const vendedorColunas = `id, nome, regiao, uf, data_admissao, data_desligamento, meta_mensal, created_at, updated_at`

// Vendedores é o repositório de vendedores (porta comercialapp.Vendedores).
// Cada método executa a própria consulta: sem cache entre requests (R4).
type Vendedores struct{ q db.Querier }

// NovoVendedores liga o repositório ao pool (ou a uma tx). q nil: panic.
func NovoVendedores(q db.Querier) *Vendedores {
	if q == nil {
		panic("comercialmysql: NovoVendedores exige Querier")
	}
	return &Vendedores{q: q}
}

// ListarResumo devolve a lista de seleção no escopo (O1): Irrestrito → todos
// (ativos e desligados, por nome; sem linhas = nil); Vendedor → só o
// próprio, filtrado no SQL (sem linhas = [] não nulo); SemAcesso/zero →
// erro sem SQL.
func (r *Vendedores) ListarResumo(ctx context.Context, e kernel.Escopo) ([]comercial.VendedorResumo, error) {
	vendedorID, filtra, err := e.Filtro()
	if err != nil {
		return nil, err
	}
	if !filtra {
		const q = `
		SELECT id, nome, regiao, uf, data_desligamento
		FROM vendedores
		ORDER BY nome ASC`
		return r.resumos(ctx, "listar vendedores", q)
	}
	const q = `
		SELECT id, nome, regiao, uf, data_desligamento
		FROM vendedores
		WHERE id = ?
		LIMIT 1`
	out, err := r.resumos(ctx, "listar vendedor por id", q, vendedorID)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []comercial.VendedorResumo{}
	}
	return out, nil
}

func (r *Vendedores) resumos(ctx context.Context, op, q string, args ...any) ([]comercial.VendedorResumo, error) {
	out, err := db.Consultar(ctx, r.q, "comercialmysql: "+op, q, args, func(s db.Linha) (*comercial.VendedorResumo, error) {
		var v comercial.VendedorResumo
		if err := s.Scan(&v.ID, &v.Nome, &v.Regiao, &v.UF, &v.DataDesligamento); err != nil {
			return nil, fmt.Errorf("comercialmysql: ler vendedor: %w", err)
		}
		return &v, nil
	})
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqVendedores, "Vendedores.ListarResumo", "SELECT vendedores linhas=%d", len(out))
	return out, nil
}

// PorID busca o vendedor (ativo ou não); ErrVendedorNaoEncontrado sem linha.
func (r *Vendedores) PorID(ctx context.Context, id int64) (*comercial.Vendedor, error) {
	q := "SELECT " + vendedorColunas + " FROM vendedores WHERE id = ? LIMIT 1"
	v, err := lerUma(r.q.QueryRowContext(ctx, q, id), comercial.ErrVendedorNaoEncontrado, "vendedor", func(v *comercial.Vendedor) []any {
		return []any{&v.ID, &v.Nome, &v.Regiao, &v.UF, &v.DataAdmissao, &v.DataDesligamento, &v.MetaMensal, &v.CreatedAt, &v.UpdatedAt}
	})
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqVendedores, "Vendedores.PorID", "SELECT vendedores linhas=1")
	return v, nil
}

// Existe informa se há vendedor com o id (ativo ou não).
func (r *Vendedores) Existe(ctx context.Context, id int64) (bool, error) {
	return existe(ctx, r.q, arqVendedores, "Vendedores.Existe", "existe vendedor", `SELECT 1 FROM vendedores WHERE id = ? LIMIT 1`, id)
}

// Desligado informa se o vendedor tem data_desligamento. Consulta própria a
// cada chamada (sem cache): o desligamento vale na requisição seguinte.
func (r *Vendedores) Desligado(ctx context.Context, id int64) (bool, error) {
	const q = `SELECT data_desligamento IS NOT NULL FROM vendedores WHERE id = ? LIMIT 1`
	var desligado int64
	err := r.q.QueryRowContext(ctx, q, id).Scan(&desligado)
	if errors.Is(err, sql.ErrNoRows) {
		return false, comercial.ErrVendedorNaoEncontrado
	}
	if err != nil {
		return false, fmt.Errorf("comercialmysql: vendedor desligado: %w", err)
	}
	vlog.Printf(arqVendedores, "Vendedores.Desligado", "SELECT vendedores desligado=%t", desligado != 0)
	return desligado != 0, nil
}

// Inserir grava o vendedor e preenche v.ID.
func (r *Vendedores) Inserir(ctx context.Context, v *comercial.Vendedor) error {
	const q = `
		INSERT INTO vendedores (nome, regiao, uf, data_admissao, data_desligamento, meta_mensal)
		VALUES (?, ?, ?, ?, ?, ?)`
	id, err := inserir(ctx, r.q, "inserir vendedor", q, v.Nome, v.Regiao, v.UF, v.DataAdmissao, v.DataDesligamento, v.MetaMensal)
	if err != nil {
		return err
	}
	v.ID = id
	vlog.Printf(arqVendedores, "Vendedores.Inserir", "INSERT vendedores id=%d", id)
	return nil
}

// Atualizar grava os campos editáveis (o desligamento não muda por aqui).
func (r *Vendedores) Atualizar(ctx context.Context, id int64, v *comercial.Vendedor) error {
	const q = `
		UPDATE vendedores
		SET nome = ?, regiao = ?, uf = ?, data_admissao = ?, meta_mensal = ?
		WHERE id = ?`
	return execPorID(ctx, r.q, arqVendedores, "Vendedores.Atualizar", "atualizar vendedor", comercial.ErrVendedorNaoEncontrado,
		q, v.Nome, v.Regiao, v.UF, v.DataAdmissao, v.MetaMensal, id)
}

// DefinirDesligamento grava (ou limpa, com quando nil) data_desligamento.
func (r *Vendedores) DefinirDesligamento(ctx context.Context, id int64, quando *time.Time) error {
	const q = `UPDATE vendedores SET data_desligamento = ? WHERE id = ?`
	data := sql.NullTime{}
	if quando != nil {
		data = sql.NullTime{Time: *quando, Valid: true}
	}
	return execPorID(ctx, r.q, arqVendedores, "Vendedores.DefinirDesligamento", "definir desligamento vendedor",
		comercial.ErrVendedorNaoEncontrado, q, data, id)
}

// MarcarDesligamento grava a data SEM sobrescrever uma já existente
// (COALESCE, BUG-04): desligar de novo preserva a data original e responde
// sucesso. Idempotente; 0 linhas casadas = vendedor inexistente. Roda na tx
// do desligamento (I-09).
func (r *Vendedores) MarcarDesligamento(ctx context.Context, id int64, quando time.Time) error {
	const q = `UPDATE vendedores SET data_desligamento = COALESCE(data_desligamento, ?) WHERE id = ?`
	return execPorID(ctx, r.q, arqVendedores, "Vendedores.MarcarDesligamento", "marcar desligamento vendedor",
		comercial.ErrVendedorNaoEncontrado, q, quando, id)
}

package comercialmysql

import (
	"context"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqOportunidades = "oportunidades.go"

const oportunidadeColunas = `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda, created_at, updated_at`

// Oportunidades é o repositório de oportunidades (porta
// comercialapp.Oportunidades).
type Oportunidades struct{ q db.Querier }

// NovoOportunidades liga o repositório ao pool (ou a uma tx). q nil: panic.
func NovoOportunidades(q db.Querier) *Oportunidades {
	if q == nil {
		panic("comercialmysql: NovoOportunidades exige Querier")
	}
	return &Oportunidades{q: q}
}

// Listar pagina as oportunidades do escopo que casam com o filtro.
func (r *Oportunidades) Listar(ctx context.Context, e kernel.Escopo, f comercial.FiltroOportunidade, p kernel.Pagina) (kernel.Resultado[comercial.Oportunidade], error) {
	vendedorID, err := vendedorDaListagem(e, f.VendedorID)
	if err != nil {
		return kernel.Resultado[comercial.Oportunidade]{}, err
	}
	w, args := whereOportunidade(f, vendedorID)
	res, err := paginar(ctx, r.q, "oportunidades", "oportunidades", oportunidadeColunas, db.Where(w), ordemOportunidade(f.Ordenacao), args, p, scanOportunidade)
	if err != nil {
		return res, err
	}
	vlog.Printf(arqOportunidades, "Oportunidades.Listar", "SELECT oportunidades linhas=%d total=%d", len(res.Itens), res.Total)
	return res, nil
}

func whereOportunidade(f comercial.FiltroOportunidade, vendedorID int64) ([]string, []any) {
	var conds []string
	var args []any
	if f.ClienteID > 0 {
		conds, args = append(conds, "cliente_id = ?"), append(args, f.ClienteID)
	}
	if vendedorID > 0 {
		conds, args = append(conds, "vendedor_id = ?"), append(args, vendedorID)
	}
	if f.Etapa != "" {
		conds, args = append(conds, "etapa = ?"), append(args, f.Etapa)
	}
	if f.Origem != "" {
		conds, args = append(conds, "origem = ?"), append(args, f.Origem)
	}
	if f.De != "" {
		conds, args = append(conds, "data_abertura >= ?"), append(args, f.De)
	}
	if f.Ate != "" {
		conds, args = append(conds, "data_abertura <= ?"), append(args, f.Ate)
	}
	if f.Busca != "" {
		like := "%" + f.Busca + "%"
		conds, args = append(conds, "(origem LIKE ? OR etapa LIKE ?)"), append(args, like, like)
	}
	return conds, args
}

// PorID busca a oportunidade; ErrOportunidadeNaoEncontrada sem linha.
func (r *Oportunidades) PorID(ctx context.Context, id int64) (*comercial.Oportunidade, error) {
	q := "SELECT " + oportunidadeColunas + " FROM oportunidades WHERE oportunidade_id = ? LIMIT 1"
	o, err := scanOportunidade(r.q.QueryRowContext(ctx, q, id))
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqOportunidades, "Oportunidades.PorID", "SELECT oportunidades linhas=1")
	return o, nil
}

func scanOportunidade(s db.Linha) (*comercial.Oportunidade, error) {
	return lerUma(s, comercial.ErrOportunidadeNaoEncontrada, "oportunidade", func(o *comercial.Oportunidade) []any {
		return []any{&o.OportunidadeID, &o.ClienteID, &o.VendedorID, &o.Origem, &o.DataAbertura, &o.Etapa, &o.ProbabilidadePct,
			&o.ValorEstimado, &o.DataFechamento, &o.CicloDias, &o.MotivoPerda, &o.CreatedAt, &o.UpdatedAt}
	})
}

// Inserir grava a oportunidade e preenche OportunidadeID.
func (r *Oportunidades) Inserir(ctx context.Context, o *comercial.Oportunidade) error {
	const q = `
		INSERT INTO oportunidades (cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	id, err := inserir(ctx, r.q, "inserir oportunidade", q, o.ClienteID, o.VendedorID, o.Origem, o.DataAbertura, o.Etapa,
		o.ProbabilidadePct, o.ValorEstimado, o.DataFechamento, o.CicloDias, o.MotivoPerda)
	if err != nil {
		return err
	}
	o.OportunidadeID = id
	vlog.Printf(arqOportunidades, "Oportunidades.Inserir", "INSERT oportunidades id=%d", id)
	return nil
}

// Atualizar grava os campos editáveis.
func (r *Oportunidades) Atualizar(ctx context.Context, id int64, o *comercial.Oportunidade) error {
	const q = `
		UPDATE oportunidades
		SET cliente_id = ?, vendedor_id = ?, origem = ?, data_abertura = ?, etapa = ?, probabilidade_pct = ?, valor_estimado = ?, data_fechamento = ?, ciclo_dias = ?, motivo_perda = ?
		WHERE oportunidade_id = ?`
	return execPorID(ctx, r.q, arqOportunidades, "Oportunidades.Atualizar", "atualizar oportunidade", comercial.ErrOportunidadeNaoEncontrada,
		q, o.ClienteID, o.VendedorID, o.Origem, o.DataAbertura, o.Etapa, o.ProbabilidadePct, o.ValorEstimado,
		o.DataFechamento, o.CicloDias, o.MotivoPerda, id)
}

// Excluir remove a oportunidade (hard delete).
func (r *Oportunidades) Excluir(ctx context.Context, id int64) error {
	return execPorID(ctx, r.q, arqOportunidades, "Oportunidades.Excluir", "delete oportunidade", comercial.ErrOportunidadeNaoEncontrada,
		`DELETE FROM oportunidades WHERE oportunidade_id = ?`, id)
}

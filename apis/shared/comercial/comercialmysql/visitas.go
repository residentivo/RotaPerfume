package comercialmysql

import (
	"context"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqVisitas = "visitas.go"

const visitaColunas = `visita_id, cliente_id, vendedor_id, data_visita, resultado, duracao_min, created_at, updated_at`

// Visitas é o repositório de visitas (porta comercialapp.Visitas).
type Visitas struct{ q db.Querier }

// NovoVisitas liga o repositório ao pool (ou a uma tx). q nil: panic.
func NovoVisitas(q db.Querier) *Visitas {
	if q == nil {
		panic("comercialmysql: NovoVisitas exige Querier")
	}
	return &Visitas{q: q}
}

// Listar pagina as visitas do escopo que casam com o filtro (o vendedor do
// escopo sobrescreve o da query, vendedorDaListagem).
func (r *Visitas) Listar(ctx context.Context, e kernel.Escopo, f comercial.FiltroVisita, p kernel.Pagina) (kernel.Resultado[comercial.Visita], error) {
	vendedorID, err := vendedorDaListagem(e, f.VendedorID)
	if err != nil {
		return kernel.Resultado[comercial.Visita]{}, err
	}
	w, args := whereVisita(f, vendedorID)
	res, err := paginar(ctx, r.q, "visitas", "visitas", visitaColunas, db.Where(w), ordemVisita(f.Ordenacao), args, p, scanVisita)
	if err != nil {
		return res, err
	}
	vlog.Printf(arqVisitas, "Visitas.Listar", "SELECT visitas linhas=%d total=%d", len(res.Itens), res.Total)
	return res, nil
}

func whereVisita(f comercial.FiltroVisita, vendedorID int64) ([]string, []any) {
	var conds []string
	var args []any
	if f.ClienteID > 0 {
		conds, args = append(conds, "cliente_id = ?"), append(args, f.ClienteID)
	}
	if vendedorID > 0 {
		conds, args = append(conds, "vendedor_id = ?"), append(args, vendedorID)
	}
	if f.Resultado != "" {
		conds, args = append(conds, "resultado = ?"), append(args, f.Resultado)
	}
	if f.De != "" {
		conds, args = append(conds, "data_visita >= ?"), append(args, f.De)
	}
	if f.Ate != "" {
		conds, args = append(conds, "data_visita <= ?"), append(args, f.Ate)
	}
	if f.Busca != "" {
		conds, args = append(conds, "resultado LIKE ?"), append(args, "%"+f.Busca+"%")
	}
	return conds, args
}

// PorID busca a visita; ErrVisitaNaoEncontrada sem linha. A posse (E-04) é
// do caso de uso, pelo vendedor_id lido aqui.
func (r *Visitas) PorID(ctx context.Context, id int64) (*comercial.Visita, error) {
	q := "SELECT " + visitaColunas + " FROM visitas WHERE visita_id = ? LIMIT 1"
	v, err := scanVisita(r.q.QueryRowContext(ctx, q, id))
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqVisitas, "Visitas.PorID", "SELECT visitas linhas=1")
	return v, nil
}

func scanVisita(s db.Linha) (*comercial.Visita, error) {
	return db.LerUma(s, comercial.ErrVisitaNaoEncontrada, "comercialmysql: ler visita", func(v *comercial.Visita) []any {
		return []any{&v.VisitaID, &v.ClienteID, &v.VendedorID, &v.DataVisita, &v.Resultado, &v.DuracaoMin, &v.CreatedAt, &v.UpdatedAt}
	})
}

// Inserir grava a visita e preenche VisitaID.
func (r *Visitas) Inserir(ctx context.Context, v *comercial.Visita) error {
	const q = `
		INSERT INTO visitas (cliente_id, vendedor_id, data_visita, resultado, duracao_min)
		VALUES (?, ?, ?, ?, ?)`
	id, err := db.Inserir(ctx, r.q, "comercialmysql: inserir visita", q, v.ClienteID, v.VendedorID, v.DataVisita, v.Resultado, v.DuracaoMin)
	if err != nil {
		return err
	}
	v.VisitaID = id
	vlog.Printf(arqVisitas, "Visitas.Inserir", "INSERT visitas id=%d", id)
	return nil
}

// Atualizar grava os campos editáveis.
func (r *Visitas) Atualizar(ctx context.Context, id int64, v *comercial.Visita) error {
	const q = `
		UPDATE visitas
		SET cliente_id = ?, vendedor_id = ?, data_visita = ?, resultado = ?, duracao_min = ?
		WHERE visita_id = ?`
	return db.ExecPorID(ctx, r.q, arqVisitas, "Visitas.Atualizar", "comercialmysql: atualizar visita", comercial.ErrVisitaNaoEncontrada,
		q, v.ClienteID, v.VendedorID, v.DataVisita, v.Resultado, v.DuracaoMin, id)
}

// Excluir remove a visita (hard delete: não há deleted_at).
func (r *Visitas) Excluir(ctx context.Context, id int64) error {
	return db.ExecPorID(ctx, r.q, arqVisitas, "Visitas.Excluir", "comercialmysql: delete visita", comercial.ErrVisitaNaoEncontrada,
		`DELETE FROM visitas WHERE visita_id = ?`, id)
}

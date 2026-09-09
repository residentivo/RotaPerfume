// Package repositories contém funções puras de acesso a dados (stateless).
// Dashboard queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
)

// DashboardRepository agrupa queries do dashboard.
type DashboardRepository struct{}

// NewDashboardRepository cria um repositório stateless.
func NewDashboardRepository() *DashboardRepository {
	return &DashboardRepository{}
}

// GetVendasTotais retorna (valor_total, quantidade) de vendas no periodo.
// Se a tabela pedidos não existir, retorna (0, 0).
// periodo: "today" = dia atual, "month" = mes atual.
func (r *DashboardRepository) GetVendasTotais(ctx context.Context, db *sql.DB, periodo string) (float64, int, error) {
	// Tenta buscar da tabela pedidos (se existir).
	// A query abaixo usa um filtro de periodo baseado na coluna data_pedido (se existir).
	// Se a tabela nao existir, a query falha e retornamos 0,0.
	var valorTotal sql.NullFloat64
	var quantidade sql.NullInt64

	var whereClause string
	if periodo == "today" {
		whereClause = "DATE(data_pedido) = CURDATE()"
	} else {
		// month: primeiro ao ultimo dia do mes atual.
		whereClause = "YEAR(data_pedido) = YEAR(CURDATE()) AND MONTH(data_pedido) = MONTH(CURDATE())"
	}

	// Query genérica que só funciona se a tabela pedidos existir.
	q := fmt.Sprintf(`
		SELECT COALESCE(SUM(valor_total), 0), COUNT(*)
		FROM pedidos
		WHERE %s AND status NOT IN ('cancelado', 'devolvido')`, whereClause)

	err := db.QueryRowContext(ctx, q).Scan(&valorTotal, &quantidade)
	if err != nil {
		if isTableNotFound(err) {
			// Tabela pedidos ainda não existe - retorna zeros.
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("GetVendasTotais: %w", err)
	}

	v := float64(0)
	qtd := int(0)
	if valorTotal.Valid {
		v = valorTotal.Float64
	}
	if quantidade.Valid {
		qtd = int(quantidade.Int64)
	}
	return v, qtd, nil
}

// GetTotalPedidos retorna o total de pedidos no periodo.
func (r *DashboardRepository) GetTotalPedidos(ctx context.Context, db *sql.DB, periodo string) (int, error) {
	var whereClause string
	if periodo == "today" {
		whereClause = "DATE(data_pedido) = CURDATE()"
	} else {
		whereClause = "YEAR(data_pedido) = YEAR(CURDATE()) AND MONTH(data_pedido) = MONTH(CURDATE())"
	}

	q := fmt.Sprintf(`
		SELECT COUNT(*) FROM pedidos
		WHERE %s AND status NOT IN ('cancelado', 'devolvido')`, whereClause)

	var total sql.NullInt64
	err := db.QueryRowContext(ctx, q).Scan(&total)
	if err != nil {
		if isTableNotFound(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("GetTotalPedidos: %w", err)
	}

	if total.Valid {
		return int(total.Int64), nil
	}
	return 0, nil
}

// VendedorRanking representa um vendedor no ranking de vendas.
type VendedorRanking struct {
	ID              int64   `json:"id"`
	Nome            string  `json:"nome"`
	TotalVendas     float64 `json:"total_vendas"`
	Meta            float64 `json:"meta"`
	PercentualMeta  float64 `json:"percentual_meta"`
	QuantidadeVendas int    `json:"quantidade_vendas"`
}

// GetTopVendedores retorna o ranking dos top N vendedores por valor de vendas no mes atual.
func (r *DashboardRepository) GetTopVendedores(ctx context.Context, db *sql.DB, limit int) ([]VendedorRanking, error) {
	// Join entre vendedores e pedidos (se existir).
	// Para funcionar sem pedidos, retornamos vendedores ativos ordenados por meta.
	q := `
		SELECT
			v.id,
			v.nome,
			v.meta_mensal AS meta
		FROM vendedores v
		WHERE v.data_desligamento IS NULL
		ORDER BY v.meta_mensal DESC
		LIMIT ?`

	rows, err := db.QueryContext(ctx, q, limit)
	if err != nil {
		if isTableNotFound(err) {
			return []VendedorRanking{}, nil
		}
		return nil, fmt.Errorf("GetTopVendedores: %w", err)
	}
	defer rows.Close()

	var result []VendedorRanking
	for rows.Next() {
		var vr VendedorRanking
		if err := rows.Scan(&vr.ID, &vr.Nome, &vr.Meta); err != nil {
			return nil, fmt.Errorf("GetTopVendedores scan: %w", err)
		}
		vr.TotalVendas = 0
		vr.QuantidadeVendas = 0
		vr.PercentualMeta = 0
		result = append(result, vr)
	}

	// Se a tabela pedidos existir, enriquecemos com dados de vendas.
	if len(result) > 0 {
		r.enrichWithVendas(ctx, db, result)
	}

	return result, nil
}

// enrichWithVendas atualiza TotalVendas e PercentualMeta a partir de pedidos.
func (r *DashboardRepository) enrichWithVendas(ctx context.Context, db *sql.DB, ranking []VendedorRanking) {
	if len(ranking) == 0 {
		return
	}

	// Monta placeholders (?) para IN.
	placeholders := ""
	args := make([]any, len(ranking))
	for i, v := range ranking {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = v.ID
	}

	q := fmt.Sprintf(`
		SELECT id_vendedor,
			   COALESCE(SUM(valor_total), 0),
			   COUNT(*)
		FROM pedidos
		WHERE id_vendedor IN (%s)
		  AND YEAR(data_pedido) = YEAR(CURDATE())
		  AND MONTH(data_pedido) = MONTH(CURDATE())
		  AND status NOT IN ('cancelado', 'devolvido')
		GROUP BY id_vendedor`, placeholders)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return // silent fail - ranking ja tem dados
	}
	defer rows.Close()

	salesMap := make(map[int64]struct {
		total int
		qtd   int
	})

	for rows.Next() {
		var vid int64
		var total sql.NullFloat64
		var qtd sql.NullInt64
		if err := rows.Scan(&vid, &total, &qtd); err != nil {
			continue
		}
		salesMap[vid] = struct {
			total int
			qtd   int
		}{total: int(total.Float64), qtd: int(qtd.Int64)}
	}

	for i := range ranking {
		if sale, ok := salesMap[ranking[i].ID]; ok {
			ranking[i].TotalVendas = float64(sale.total)
			ranking[i].QuantidadeVendas = sale.qtd
			if ranking[i].Meta > 0 {
				ranking[i].PercentualMeta = (ranking[i].TotalVendas / ranking[i].Meta) * 100
			}
		}
	}
}

// MetaVendedor representa a meta de um vendedor comparada com realizacao.
type MetaVendedor struct {
	ID             int64   `json:"id"`
	Nome           string  `json:"nome"`
	Regiao         string  `json:"regiao"`
	UF             string  `json:"uf"`
	Meta           float64 `json:"meta"`
	Realizado      float64 `json:"realizado"`
	Percentual     float64 `json:"percentual"`
	QuantidadeVendas int  `json:"quantidade_vendas"`
}

// GetMetasVendedores retorna todas as metas dos vendedores ativos com comparativo.
func (r *DashboardRepository) GetMetasVendedores(ctx context.Context, db *sql.DB) ([]MetaVendedor, error) {
	q := `
		SELECT
			v.id,
			v.nome,
			v.regiao,
			v.uf,
			v.meta_mensal AS meta
		FROM vendedores v
		WHERE v.data_desligamento IS NULL
		ORDER BY v.meta_mensal DESC`

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("GetMetasVendedores: %w", err)
	}
	defer rows.Close()

	var result []MetaVendedor
	for rows.Next() {
		var mv MetaVendedor
		if err := rows.Scan(&mv.ID, &mv.Nome, &mv.Regiao, &mv.UF, &mv.Meta); err != nil {
			return nil, fmt.Errorf("GetMetasVendedores scan: %w", err)
		}
		mv.Realizado = 0
		mv.Percentual = 0
		mv.QuantidadeVendas = 0
		result = append(result, mv)
	}

	// Enriquecer com vendas do mes atual.
	r.enrichMetasWithVendas(ctx, db, result)

	return result, nil
}

// enrichMetasWithVendas atualiza Realizado e Percentual a partir de pedidos.
func (r *DashboardRepository) enrichMetasWithVendas(ctx context.Context, db *sql.DB, metas []MetaVendedor) {
	if len(metas) == 0 {
		return
	}

	placeholders := ""
	args := make([]any, len(metas))
	for i, v := range metas {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = v.ID
	}

	q := fmt.Sprintf(`
		SELECT id_vendedor,
			   COALESCE(SUM(valor_total), 0),
			   COUNT(*)
		FROM pedidos
		WHERE id_vendedor IN (%s)
		  AND YEAR(data_pedido) = YEAR(CURDATE())
		  AND MONTH(data_pedido) = MONTH(CURDATE())
		  AND status NOT IN ('cancelado', 'devolvido')
		GROUP BY id_vendedor`, placeholders)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return
	}
	defer rows.Close()

	salesMap := make(map[int64]struct {
		total int
		qtd   int
	})

	for rows.Next() {
		var vid int64
		var total sql.NullFloat64
		var qtd sql.NullInt64
		if err := rows.Scan(&vid, &total, &qtd); err != nil {
			continue
		}
		salesMap[vid] = struct {
			total int
			qtd   int
		}{total: int(total.Float64), qtd: int(qtd.Int64)}
	}

	for i := range metas {
		if sale, ok := salesMap[metas[i].ID]; ok {
			metas[i].Realizado = float64(sale.total)
			metas[i].QuantidadeVendas = sale.qtd
			if metas[i].Meta > 0 {
				metas[i].Percentual = (metas[i].Realizado / metas[i].Meta) * 100
			}
		}
	}
}

// GetVendasSeries retorna serie temporal (data -> valor, quantidade) dos ultimos N dias.
func (r *DashboardRepository) GetVendasSeries(ctx context.Context, db *sql.DB, dias int) ([]map[string]any, error) {
	q := `
		SELECT DATE(data_pedido) AS data,
			   COALESCE(SUM(valor_total), 0) AS valor,
			   COUNT(*) AS quantidade
		FROM pedidos
		WHERE data_pedido >= DATE_SUB(CURDATE(), INTERVAL ? DAY)
		  AND status NOT IN ('cancelado', 'devolvido')
		GROUP BY DATE(data_pedido)
		ORDER BY data ASC`

	rows, err := db.QueryContext(ctx, q, dias)
	if err != nil {
		if isTableNotFound(err) {
			// Gera serie vazia com dias zeros.
			return r.emptySeries(dias), nil
		}
		return nil, fmt.Errorf("GetVendasSeries: %w", err)
	}
	defer rows.Close()

	seriesMap := make(map[string]struct {
		valor      float64
		quantidade int
	})

	for rows.Next() {
		var data string
		var valor sql.NullFloat64
		var qtd sql.NullInt64
		if err := rows.Scan(&data, &valor, &qtd); err != nil {
			continue
		}
		v := 0.0
		q := 0
		if valor.Valid {
			v = valor.Float64
		}
		if qtd.Valid {
			q = int(qtd.Int64)
		}
		seriesMap[data] = struct {
			valor      float64
			quantidade int
		}{valor: v, quantidade: q}
	}

	// Se nao teve dados, retorna serie vazia.
	if len(seriesMap) == 0 {
		return r.emptySeries(dias), nil
	}

	// Monta serie completa (todos os dias do range, mesmo sem vendas).
	series := r.buildFullSeries(ctx, db, dias, seriesMap)
	return series, nil
}

// emptySeries retorna uma serie com zeros para os ultimos N dias.
func (r *DashboardRepository) emptySeries(dias int) []map[string]any {
	series := make([]map[string]any, 0, dias)
	for i := dias - 1; i >= 0; i-- {
		series = append(series, map[string]any{
			"data":       fmt.Sprintf("%d dias atras", i),
			"valor":      0.0,
			"quantidade": 0,
		})
	}
	return series
}

// buildFullSeries gera entrada para cada dia do range.
func (r *DashboardRepository) buildFullSeries(ctx context.Context, db *sql.DB, dias int, data map[string]struct {
	valor      float64
	quantidade int
}) []map[string]any {
	series := make([]map[string]any, 0, dias)

	// Pega datas do banco para ter a sequencia correta.
	q := `SELECT DATE_SUB(CURDATE(), INTERVAL n DAY) AS dia
	      FROM (
	          SELECT 0 AS n UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4
	          UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9
	          UNION SELECT 10 UNION SELECT 11 UNION SELECT 12 UNION SELECT 13 UNION SELECT 14
	          UNION SELECT 15 UNION SELECT 16 UNION SELECT 17 UNION SELECT 18 UNION SELECT 19
	          UNION SELECT 20 UNION SELECT 21 UNION SELECT 22 UNION SELECT 23 UNION SELECT 24
	          UNION SELECT 25 UNION SELECT 26 UNION SELECT 27 UNION SELECT 28 UNION SELECT 29
	          UNION SELECT 30 UNION SELECT 31 UNION SELECT 32 UNION SELECT 33 UNION SELECT 34
	          UNION SELECT 35 UNION SELECT 36 UNION SELECT 37 UNION SELECT 38 UNION SELECT 39
	          UNION SELECT 40 UNION SELECT 41 UNION SELECT 42 UNION SELECT 43 UNION SELECT 44
	          UNION SELECT 45 UNION SELECT 46 UNION SELECT 47 UNION SELECT 48 UNION SELECT 49
	          UNION SELECT 50 UNION SELECT 51 UNION SELECT 52 UNION SELECT 53 UNION SELECT 54
	          UNION SELECT 55 UNION SELECT 56 UNION SELECT 57 UNION SELECT 58 UNION SELECT 59
	          UNION SELECT 60 UNION SELECT 61 UNION SELECT 62 UNION SELECT 63 UNION SELECT 64
	          UNION SELECT 65 UNION SELECT 66 UNION SELECT 67 UNION SELECT 68 UNION SELECT 69
	          UNION SELECT 70 UNION SELECT 71 UNION SELECT 72 UNION SELECT 73 UNION SELECT 74
	          UNION SELECT 75 UNION SELECT 76 UNION SELECT 77 UNION SELECT 78 UNION SELECT 79
	          UNION SELECT 80 UNION SELECT 81 UNION SELECT 82 UNION SELECT 83 UNION SELECT 84
	          UNION SELECT 85 UNION SELECT 86 UNION SELECT 87 UNION SELECT 88 UNION SELECT 89
	          UNION SELECT 90 UNION SELECT 91 UNION SELECT 92 UNION SELECT 93 UNION SELECT 94
	          UNION SELECT 95 UNION SELECT 96 UNION SELECT 97 UNION SELECT 98 UNION SELECT 99
	          UNION SELECT 100 UNION SELECT 101 UNION SELECT 102 UNION SELECT 103 UNION SELECT 104
	          UNION SELECT 105 UNION SELECT 106 UNION SELECT 107 UNION SELECT 108 UNION SELECT 109
	          UNION SELECT 110 UNION SELECT 111 UNION SELECT 112 UNION SELECT 113 UNION SELECT 114
	          UNION SELECT 115 UNION SELECT 116 UNION SELECT 117 UNION SELECT 118 UNION SELECT 119
	          UNION SELECT 120 UNION SELECT 121 UNION SELECT 122 UNION SELECT 123 UNION SELECT 124
	          UNION SELECT 125 UNION SELECT 126 UNION SELECT 127 UNION SELECT 128 UNION SELECT 129
	          UNION SELECT 130 UNION SELECT 131 UNION SELECT 132 UNION SELECT 133 UNION SELECT 134
	          UNION SELECT 135 UNION SELECT 136 UNION SELECT 137 UNION SELECT 138 UNION SELECT 139
	          UNION SELECT 140 UNION SELECT 141 UNION SELECT 142 UNION SELECT 143 UNION SELECT 144
	          UNION SELECT 145 UNION SELECT 146 UNION SELECT 147 UNION SELECT 148 UNION SELECT 149
	          UNION SELECT 150 UNION SELECT 151 UNION SELECT 152 UNION SELECT 153 UNION SELECT 154
	          UNION SELECT 155 UNION SELECT 156 UNION SELECT 157 UNION SELECT 158 UNION SELECT 159
	          UNION SELECT 160 UNION SELECT 161 UNION SELECT 162 UNION SELECT 163 UNION SELECT 164
	          UNION SELECT 165 UNION SELECT 166 UNION SELECT 167 UNION SELECT 168 UNION SELECT 169
	          UNION SELECT 170 UNION SELECT 171 UNION SELECT 172 UNION SELECT 173 UNION SELECT 174
	          UNION SELECT 175 UNION SELECT 176 UNION SELECT 177 UNION SELECT 178 UNION SELECT 179
	          UNION SELECT 180 UNION SELECT 181 UNION SELECT 182 UNION SELECT 183 UNION SELECT 184
	          UNION SELECT 185 UNION SELECT 186 UNION SELECT 187 UNION SELECT 188 UNION SELECT 189
	          UNION SELECT 190 UNION SELECT 191 UNION SELECT 192 UNION SELECT 193 UNION SELECT 194
	          UNION SELECT 195 UNION SELECT 196 UNION SELECT 197 UNION SELECT 198 UNION SELECT 199
	          UNION SELECT 200 UNION SELECT 201 UNION SELECT 202 UNION SELECT 203 UNION SELECT 204
	          UNION SELECT 205 UNION SELECT 206 UNION SELECT 207 UNION SELECT 208 UNION SELECT 209
	          UNION SELECT 210 UNION SELECT 211 UNION SELECT 212 UNION SELECT 213 UNION SELECT 214
	          UNION SELECT 215 UNION SELECT 216 UNION SELECT 217 UNION SELECT 218 UNION SELECT 219
	          UNION SELECT 220 UNION SELECT 221 UNION SELECT 222 UNION SELECT 223 UNION SELECT 224
	          UNION SELECT 225 UNION SELECT 226 UNION SELECT 227 UNION SELECT 228 UNION SELECT 229
	          UNION SELECT 230 UNION SELECT 231 UNION SELECT 232 UNION SELECT 233 UNION SELECT 234
	          UNION SELECT 235 UNION SELECT 236 UNION SELECT 237 UNION SELECT 238 UNION SELECT 239
	          UNION SELECT 240 UNION SELECT 241 UNION SELECT 242 UNION SELECT 243 UNION SELECT 244
	          UNION SELECT 245 UNION SELECT 246 UNION SELECT 247 UNION SELECT 248 UNION SELECT 249
	          UNION SELECT 250 UNION SELECT 251 UNION SELECT 252 UNION SELECT 253 UNION SELECT 254
	          UNION SELECT 255 UNION SELECT 256 UNION SELECT 257 UNION SELECT 258 UNION SELECT 259
	          UNION SELECT 260 UNION SELECT 261 UNION SELECT 262 UNION SELECT 263 UNION SELECT 264
	          UNION SELECT 265 UNION SELECT 266 UNION SELECT 267 UNION SELECT 268 UNION SELECT 269
	          UNION SELECT 270 UNION SELECT 271 UNION SELECT 272 UNION SELECT 273 UNION SELECT 274
	          UNION SELECT 275 UNION SELECT 276 UNION SELECT 277 UNION SELECT 278 UNION SELECT 279
	          UNION SELECT 280 UNION SELECT 281 UNION SELECT 282 UNION SELECT 283 UNION SELECT 284
	          UNION SELECT 285 UNION SELECT 286 UNION SELECT 287 UNION SELECT 288 UNION SELECT 289
	          UNION SELECT 290 UNION SELECT 291 UNION SELECT 292 UNION SELECT 293 UNION SELECT 294
	          UNION SELECT 295 UNION SELECT 296 UNION SELECT 297 UNION SELECT 298 UNION SELECT 299
	          UNION SELECT 300 UNION SELECT 301 UNION SELECT 302 UNION SELECT 303 UNION SELECT 304
	          UNION SELECT 305 UNION SELECT 306 UNION SELECT 307 UNION SELECT 308 UNION SELECT 309
	          UNION SELECT 310 UNION SELECT 311 UNION SELECT 312 UNION SELECT 313 UNION SELECT 314
	          UNION SELECT 315 UNION SELECT 316 UNION SELECT 317 UNION SELECT 318 UNION SELECT 319
	          UNION SELECT 320 UNION SELECT 321 UNION SELECT 322 UNION SELECT 323 UNION SELECT 324
	          UNION SELECT 325 UNION SELECT 326 UNION SELECT 327 UNION SELECT 328 UNION SELECT 329
	          UNION SELECT 330 UNION SELECT 331 UNION SELECT 332 UNION SELECT 333 UNION SELECT 334
	          UNION SELECT 335 UNION SELECT 336 UNION SELECT 337 UNION SELECT 338 UNION SELECT 339
	          UNION SELECT 340 UNION SELECT 341 UNION SELECT 342 UNION SELECT 343 UNION SELECT 344
	          UNION SELECT 345 UNION SELECT 346 UNION SELECT 347 UNION SELECT 348 UNION SELECT 349
	          UNION SELECT 350 UNION SELECT 351 UNION SELECT 352 UNION SELECT 353 UNION SELECT 354
	          UNION SELECT 355 UNION SELECT 356 UNION SELECT 357 UNION SELECT 358 UNION SELECT 359
	          UNION SELECT 360 UNION SELECT 361 UNION SELECT 362 UNION SELECT 363 UNION SELECT 364
	      ) nums
	      WHERE n < ?
	      ORDER BY dia ASC`

	rows, err := db.QueryContext(ctx, q, dias)
	if err != nil {
		return r.emptySeries(dias)
	}
	defer rows.Close()

	for rows.Next() {
		var dia string
		if err := rows.Scan(&dia); err != nil {
			continue
		}
		if d, ok := data[dia]; ok {
			series = append(series, map[string]any{
				"data":       dia,
				"valor":      d.valor,
				"quantidade": d.quantidade,
			})
		} else {
			series = append(series, map[string]any{
				"data":       dia,
				"valor":      0.0,
				"quantidade": 0,
			})
		}
	}

	return series
}

// GetVendedoresRanking retorna ranking paginado de vendedores com vendas, meta e percentual.
func (r *DashboardRepository) GetVendedoresRanking(ctx context.Context, db *sql.DB, page, limit int) ([]map[string]any, int,  error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	// Total de vendedores ativos.
	var total int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM vendedores WHERE data_desligamento IS NULL`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("GetVendedoresRanking count: %w", err)
	}

	// Lista de vendedores ordenados por meta.
	q := `
		SELECT id, nome, regiao, uf, meta_mensal
		FROM vendedores
		WHERE data_desligamento IS NULL
		ORDER BY meta_mensal DESC
		LIMIT ? OFFSET ?`

	rows, err := db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("GetVendedoresRanking: %w", err)
	}
	defer rows.Close()

	var vendedorIDs []int64
	var raw []struct {
		ID    int64
		Nome  string
		Regiao string
		UF    string
		Meta  float64
	}

	for rows.Next() {
		var v struct {
			ID    int64
			Nome  string
			Regiao string
			UF    string
			Meta  float64
		}
		if err := rows.Scan(&v.ID, &v.Nome, &v.Regiao, &v.UF, &v.Meta); err != nil {
			return nil, 0, fmt.Errorf("GetVendedoresRanking scan: %w", err)
		}
		raw = append(raw, v)
		vendedorIDs = append(vendedorIDs, v.ID)
	}

	// Busca vendas do mes para os IDs retornados.
	vendasMap := r.getVendasMap(ctx, db, vendedorIDs)

	// Monta output.
	result := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		venda := vendasMap[v.ID]
		percentual := 0.0
		if v.Meta > 0 {
			percentual = (venda.total / v.Meta) * 100
		}
		result = append(result, map[string]any{
			"id":                v.ID,
			"nome":              v.Nome,
			"regiao":            v.Regiao,
			"uf":                v.UF,
			"total_vendas":      venda.total,
			"quantidade_vendas": venda.qtd,
			"meta":              v.Meta,
			"percentual_meta":   mathRound(percentual, 2),
		})
	}

	return result, total, nil
}

// vendaData agrega dados de venda.
type vendaData struct {
	total float64
	qtd   int
}

// getVendasMap retorna mapa de id_vendedor -> vendas do mes.
func (r *DashboardRepository) getVendasMap(ctx context.Context, db *sql.DB, ids []int64) map[int64]vendaData {
	result := make(map[int64]vendaData)
	if len(ids) == 0 {
		return result
	}

	placeholders := ""
	args := make([]any, len(ids))
	for i, id := range ids {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = id
	}

	q := fmt.Sprintf(`
		SELECT id_vendedor,
			   COALESCE(SUM(valor_total), 0),
			   COUNT(*)
		FROM pedidos
		WHERE id_vendedor IN (%s)
		  AND YEAR(data_pedido) = YEAR(CURDATE())
		  AND MONTH(data_pedido) = MONTH(CURDATE())
		  AND status NOT IN ('cancelado', 'devolvido')
		GROUP BY id_vendedor`, placeholders)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var vid int64
		var total sql.NullFloat64
		var qtd sql.NullInt64
		if err := rows.Scan(&vid, &total, &qtd); err != nil {
			continue
		}
		t := 0.0
		q := 0
		if total.Valid {
			t = total.Float64
		}
		if qtd.Valid {
			q = int(qtd.Int64)
		}
		result[vid] = vendaData{total: t, qtd: q}
	}

	return result
}

// isTableNotFound retorna true se o erro indica que a tabela nao existe.
func isTableNotFound(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "doesn't exist") || contains(errStr, "not found") ||
		contains(errStr, "Error 1146") || contains(errStr, "no such table")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mathRound arredonda um float para N casas decimais.
func mathRound(val float64, prec int) float64 {
	mult := 1.0
	for i := 0; i < prec; i++ {
		mult *= 10
	}
	return float64(int(val*mult+0.5)) / mult
}

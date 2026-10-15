// Package comercialmysql implementa as portas de comercialapp sobre MySQL
// (ARQ-04). O SQL é o dos repositórios legados, copiado sem alteração; a
// ordenação sai de switch sobre os enums do domínio (C-02); todo valor vai
// por placeholder (C-03); e o escopo de carteira só vira SQL por
// kernel.Escopo.Filtro (O1): SemAcesso e zero value voltam como erro ANTES
// de qualquer consulta.
package comercialmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

// linha é o que Scan precisa (db.Linha).
type linha = db.Linha

// execPorID roda um UPDATE/DELETE por id: nenhuma linha casada vira
// naoEncontrado (DSN com clientFoundRows=true, BUG-04). arq/rotulo vão só
// para o vlog de query (nível 4, sem valores).
func execPorID(ctx context.Context, q db.Querier, arq, rotulo, op string, naoEncontrado error, sqlTexto string, args ...any) error {
	res, err := q.ExecContext(ctx, sqlTexto, args...)
	if err != nil {
		return wrap(op, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("comercialmysql: %s rowsAffected: %w", op, err)
	}
	vlog.Printf(arq, rotulo, "afetadas=%d", n)
	if n == 0 {
		return naoEncontrado
	}
	return nil
}

// existe roda um "SELECT 1 … LIMIT 1".
func existe(ctx context.Context, q db.Querier, arq, rotulo, op, sqlTexto string, id int64) (bool, error) {
	var um int
	err := q.QueryRowContext(ctx, sqlTexto, id).Scan(&um)
	if errors.Is(err, sql.ErrNoRows) {
		vlog.Printf(arq, rotulo, "existe=false")
		return false, nil
	}
	if err != nil {
		return false, wrap(op, err)
	}
	vlog.Printf(arq, rotulo, "existe=true")
	return true, nil
}

// inserir roda um INSERT e devolve o id gerado (AUTO_INCREMENT).
func inserir(ctx context.Context, q db.Querier, op, sqlTexto string, args ...any) (int64, error) {
	res, err := q.ExecContext(ctx, sqlTexto, args...)
	if err != nil {
		return 0, wrap(op, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("comercialmysql: %s lastInsertId: %w", op, err)
	}
	return id, nil
}

// paginar roda o COUNT e o SELECT paginado de uma listagem.
func paginar[T any](ctx context.Context, q db.Querier, op, tabela, colunas, where, ordem string, args []any, p kernel.Pagina, scan func(db.Linha) (*T, error)) (kernel.Resultado[T], error) {
	var total int
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tabela+where, args...).Scan(&total); err != nil {
		return kernel.Resultado[T]{}, fmt.Errorf("comercialmysql: contar %s: %w", op, err)
	}
	sqlTexto := "SELECT " + colunas + " FROM " + tabela + where + ordem + " LIMIT ? OFFSET ?"
	itens, err := db.Consultar(ctx, q, "comercialmysql: listar "+op, sqlTexto, db.Paginada(args, p), scan)
	if err != nil {
		return kernel.Resultado[T]{}, err
	}
	return kernel.NovoResultado(itens, p, total), nil
}

// lerUma lê uma linha; sql.ErrNoRows vira naoEncontrado.
func lerUma[T any](s linha, naoEncontrado error, op string, dest func(*T) []any) (*T, error) {
	var v T
	err := s.Scan(dest(&v)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, naoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("comercialmysql: ler %s: %w", op, err)
	}
	return &v, nil
}

// vendedorDaListagem aplica o escopo à listagem de registros com coluna
// vendedor_id (visitas, oportunidades): no escopo de vendedor, o filtro de
// negócio da query é SOBRESCRITO pelo vendedor do escopo (E-03); no
// irrestrito vale o da query (0 = sem filtro, filtro de negócio). SemAcesso
// e zero value: erro, sem SQL (O1).
func vendedorDaListagem(e kernel.Escopo, daQuery int64) (int64, error) {
	vendedorID, filtra, err := e.Filtro()
	if err != nil {
		return 0, err
	}
	if filtra {
		return vendedorID, nil
	}
	return daQuery, nil
}

// ordemCliente monta " ORDER BY <coluna> <dir>" (padrão "cliente_id_origem
// ASC").
func ordemCliente(o kernel.Ordenacao[comercial.CampoCliente]) string {
	return " ORDER BY " + colunaCliente(o.Campo()) + " " + db.DirecaoSQL(o.Direcao(), "ASC")
}

// colunaCliente é a whitelist de clientes: enum → coluna.
func colunaCliente(c comercial.CampoCliente) string {
	switch c {
	case comercial.OrdemClienteRazaoSocial:
		return "razao_social"
	case comercial.OrdemClienteCNPJ:
		return "cnpj"
	case comercial.OrdemClienteSegmento:
		return "segmento"
	case comercial.OrdemClienteCidade:
		return "cidade"
	case comercial.OrdemClienteUF:
		return "uf"
	case comercial.OrdemClienteDataCadastro:
		return "data_cadastro"
	case comercial.OrdemClienteAtivo:
		return "ativo"
	case comercial.OrdemClienteCriadoEm:
		return "created_at"
	case comercial.OrdemClienteAtualizadoEm:
		return "updated_at"
	default:
		return "cliente_id_origem"
	}
}

// ordemVisita monta o ORDER BY de visitas (padrão "visita_id ASC").
func ordemVisita(o kernel.Ordenacao[comercial.CampoVisita]) string {
	return " ORDER BY " + colunaVisita(o.Campo()) + " " + db.DirecaoSQL(o.Direcao(), "ASC")
}

func colunaVisita(c comercial.CampoVisita) string {
	switch c {
	case comercial.OrdemVisitaData:
		return "data_visita"
	case comercial.OrdemVisitaDuracao:
		return "duracao_min"
	case comercial.OrdemVisitaResultado:
		return "resultado"
	case comercial.OrdemVisitaCriadoEm:
		return "created_at"
	case comercial.OrdemVisitaAtualizadoEm:
		return "updated_at"
	default: // "id" e "visita_id"
		return "visita_id"
	}
}

// ordemOportunidade monta o ORDER BY de oportunidades (padrão
// "oportunidade_id ASC").
func ordemOportunidade(o kernel.Ordenacao[comercial.CampoOportunidade]) string {
	return " ORDER BY " + colunaOportunidade(o.Campo()) + " " + db.DirecaoSQL(o.Direcao(), "ASC")
}

func colunaOportunidade(c comercial.CampoOportunidade) string {
	switch c {
	case comercial.OrdemOportunidadeDataAbertura:
		return "data_abertura"
	case comercial.OrdemOportunidadeValorEstimado:
		return "valor_estimado"
	case comercial.OrdemOportunidadeProbabilidade:
		return "probabilidade_pct"
	case comercial.OrdemOportunidadeEtapa:
		return "etapa"
	case comercial.OrdemOportunidadeOrigem:
		return "origem"
	case comercial.OrdemOportunidadeCriadoEm:
		return "created_at"
	case comercial.OrdemOportunidadeAtualizadoEm:
		return "updated_at"
	default:
		return "oportunidade_id"
	}
}

// wrap embrulha o erro de infra só com o nome da operação (O6-iv); um 1062
// perde o valor duplicado antes (N2: db.SemValorDuplicado).
func wrap(op string, err error) error {
	return fmt.Errorf("comercialmysql: %s: %w", op, db.SemValorDuplicado(err))
}

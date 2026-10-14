// Package repositories contém funções puras de acesso a dados (stateless).
// Estoque queries - cada função executa sua própria query (sem cache L1).
//
// ARQ-02: a leitura e o ajuste manual de estoque migraram para
// shared/catalogo/catalogomysql. Ficam aqui só as escritas usadas fora do
// Catálogo: a baixa por faturamento (chamada por PedidoRepository, dentro da
// tx do pedido; migra com Vendas no ARQ-05) e o upsert absoluto do ERP.
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/vlog"
)

// EstoqueRepository agrupa as escritas de estoque que ainda são legadas.
type EstoqueRepository struct{}

// NewEstoqueRepository cria um repositório stateless.
func NewEstoqueRepository() *EstoqueRepository {
	return &EstoqueRepository{}
}

// estoqueDataLayout é o formato usado para comparar/gravar data_snapshot.
const estoqueDataLayout = "2006-01-02"

// UpsertPorDataSku faz upsert do saldo/ruptura ABSOLUTO de um sku numa data:
// se já existir registro para aquele par (data_snapshot, sku) — UNIQUE KEY
// uk_estoque_data_sku —, sobrescreve saldo/ruptura; senão cria um novo
// registro.
//
// Sem chamador de produção hoje (o importador de estoque grava com SQL
// próprio); fica até o ARQ-05 junto com AjustarSaldoPorFaturamento. Grava o
// saldo absoluto (como o CSV do ERP). NÃO usar para baixa de estoque por
// faturamento de pedidos — nesse
// caso, use AjustarSaldoPorFaturamento (delta relativo, seguro sob
// concorrência e dentro da mesma transação do faturamento).
func (r *EstoqueRepository) UpsertPorDataSku(ctx context.Context, db *sql.DB, sku string, data time.Time, saldo int, ruptura bool) error {
	const q = `
		INSERT INTO estoque (data_snapshot, sku, saldo, ruptura)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			saldo = VALUES(saldo),
			ruptura = VALUES(ruptura)`
	vlog.Printf("estoque_repository.go", "EstoqueRepository.UpsertPorDataSku", "definindo _, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	_, err := db.ExecContext(ctx, q, data.Format(estoqueDataLayout), sku, saldo, ruptura)
	vlog.Printf("estoque_repository.go", "EstoqueRepository.UpsertPorDataSku", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: upsert estoque por data/sku: %w", err)
	}
	return nil
}

// AjustarSaldoPorFaturamento aplica um DELTA relativo ao saldo de um sku
// numa data, para uso EXCLUSIVO do faturamento de pedidos (Backend). Ao
// contrário de UpsertPorDataSku (que grava o saldo ABSOLUTO vindo do CSV do
// ERP e por isso não é seguro para baixa concorrente), este método faz
// `saldo = saldo - delta` atomicamente no próprio SQL, evitando race
// condition de leitura-then-escrita sob concorrência (dois pedidos
// faturando o mesmo sku ao mesmo tempo).
//
// Recebe uma *sql.Tx (não *sql.DB) porque o Backend DEVE chamar este método
// dentro da MESMA transação de PedidoRepository.UpdateComItens, garantindo
// atomicidade entre faturar o pedido e baixar o estoque (exigência do
// SecBrain 🟣).
//
// Se ainda não existir registro para (data_snapshot, sku) — ex: primeiro
// movimento do dia, antes do import do CSV rodar — o INSERT inicial parte
// de saldo = 0 e já aplica o delta (saldo final = -delta se for uma baixa;
// para dar entrada de saldo positivo, delta pode ser negativo). Chamadores
// que precisem partir do saldo do dia anterior devem resolver esse valor
// antes de chamar (ex: consultar a última posição via
// catalogomysql.Estoques.UltimaPosicao) — este método não faz esse
// carry-over sozinho, para manter a query atômica e simples.
//
// ruptura é recalculada automaticamente: true se o saldo resultante for <= 0.
//
// Usa SELECT ... FOR UPDATE para serializar concorrência entre faturamentos
// simultâneos do mesmo (data_snapshot, sku) antes do upsert, já que o
// próprio INSERT ... ON DUPLICATE KEY UPDATE sozinho não impede leituras
// fantasmas em isolamentos menos estritos.
func (r *EstoqueRepository) AjustarSaldoPorFaturamento(ctx context.Context, tx *sql.Tx, sku string, data time.Time, delta int) error {
	vlog.Printf("estoque_repository.go", "EstoqueRepository.AjustarSaldoPorFaturamento", "definindo dataStr com resultado de chamada a data.Format")
	dataStr := data.Format(estoqueDataLayout)

	// Serializa concorrência: bloqueia a linha (se existir) antes do upsert,
	// para que dois faturamentos simultâneos do mesmo sku/data não pisem um
	// no outro entre o SELECT e o INSERT/UPDATE.
	vlog.Printf("estoque_repository.go", "EstoqueRepository.AjustarSaldoPorFaturamento", "declarando variável existing")
	var existing int
	vlog.Printf("estoque_repository.go", "EstoqueRepository.AjustarSaldoPorFaturamento", "definindo err com resultado de execução SQL via tx.QueryRowContext(...).Scan (query SELECT em estoque, args omitidos) com leitura do resultado")
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM estoque WHERE data_snapshot = ? AND sku = ? FOR UPDATE`,
		dataStr, sku,
	).Scan(&existing)
	vlog.Printf("estoque_repository.go", "EstoqueRepository.AjustarSaldoPorFaturamento", "verificando se err != nil && err != sql.ErrNoRows")
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("repositories: lock estoque para ajuste de faturamento: %w", err)
	}

	// VALUES(saldo) carrega a CONTRIBUIÇÃO (não o saldo final): -delta.
	// - Caminho INSERT (linha nova): o valor literal inserido em `saldo` É
	//   a contribuição, então o saldo final já nasce correto: 0 -> -delta.
	// - Caminho UPDATE (linha existente): `saldo = saldo + VALUES(saldo)`
	//   soma a contribuição ao saldo atual: existing + (-delta) = existing - delta.
	// Isso evita o erro comum de usar "saldo - VALUES(saldo)" (que inverteria
	// o sinal no caminho UPDATE, já que VALUES(saldo) teria que representar
	// tanto o saldo final do INSERT quanto o delta do UPDATE simultaneamente
	// — impossível com uma única expressão de sinal fixo).
	const q = `
		INSERT INTO estoque (data_snapshot, sku, saldo, ruptura)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			saldo = saldo + VALUES(saldo),
			ruptura = (saldo + VALUES(saldo)) <= 0`
	vlog.Printf("estoque_repository.go", "EstoqueRepository.AjustarSaldoPorFaturamento", "definindo contribuicao = -delta")
	contribuicao := -delta
	vlog.Printf("estoque_repository.go", "EstoqueRepository.AjustarSaldoPorFaturamento", "atribuindo _, err com resultado de execução SQL via tx.ExecContext (query q, args omitidos)")
	_, err = tx.ExecContext(ctx, q, dataStr, sku, contribuicao, contribuicao <= 0)
	vlog.Printf("estoque_repository.go", "EstoqueRepository.AjustarSaldoPorFaturamento", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: ajustar saldo estoque por faturamento: %w", err)
	}
	return nil
}

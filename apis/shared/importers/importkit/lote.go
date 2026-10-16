package importkit

import (
	"database/sql"
	"fmt"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/vlog"
)

// Contagem é o resultado de um upsert em lote.
type Contagem struct {
	Inseridos   int // inseridos ou inalterados
	Atualizados int
	Falhas      int
}

// Registrar classifica o resultado de um INSERT … ON DUPLICATE KEY UPDATE.
// Com clientFoundRows=true no DSN (BUG-04), o MySQL devolve 1 = INSERT novo
// OU linha já existente sem alteração (por isso "inseridos_ou_inalterados")
// e 2 = UPDATE com alteração real. 0 não ocorre com clientFoundRows=true e,
// por robustez, conta como atualizado.
func (c *Contagem) Registrar(r sql.Result) {
	if afetadas, _ := r.RowsAffected(); afetadas == 1 {
		c.Inseridos++
		return
	}
	c.Atualizados++
}

// Upsert descreve a gravação em lote de um importador.
type Upsert[R any] struct {
	// SQL é o INSERT … ON DUPLICATE KEY UPDATE com placeholders.
	SQL string
	// ErroPrepare prefixa a falha do Prepare ("prepare falhou").
	ErroPrepare string
	// Args devolve os valores da linha; ok=false pula a linha como falha
	// (o gancho já logou o motivo, ex.: referência ausente no lookup).
	Args func(R) (args []any, ok bool)
	// LogFalha loga a falha do Exec de uma linha (sem CNPJ, M-04).
	LogFalha func(R, error)
}

// Gravar prepara o SQL uma vez e executa uma linha por vez: a falha de uma
// linha é contada e não aborta o lote. Só devolve erro se o Prepare falhar.
func (u Upsert[R]) Gravar(db cmdutil.DB, linhas []R) (Contagem, error) {
	var c Contagem
	stmt, err := db.Prepare(u.SQL)
	if err != nil {
		return c, fmt.Errorf("%s: %w", u.ErroPrepare, err)
	}
	defer stmt.Close()
	for _, linha := range linhas {
		args, ok := u.Args(linha)
		if !ok {
			c.Falhas++
			continue
		}
		r, err := stmt.Exec(args...)
		if err != nil {
			u.LogFalha(linha, err)
			c.Falhas++
			continue
		}
		c.Registrar(r)
	}
	vlog.Printf(arq, "Upsert.Gravar", "linhas=%d inseridos=%d atualizados=%d falhas=%d", len(linhas), c.Inseridos, c.Atualizados, c.Falhas)
	return c, nil
}

// IDsDeOrigem carrega o lookup de ids de origem a partir de uma consulta de
// UMA coluna (ex.: "SELECT cliente_id_origem FROM clientes"): o id de origem
// é a própria chave estrangeira, então o mapa é origem → origem.
func IDsDeOrigem(db cmdutil.DB, consulta string) (map[int64]int64, error) {
	return lookup(db, consulta, func(rows *sql.Rows, m map[int64]int64) error {
		var origem int64
		if err := rows.Scan(&origem); err != nil {
			return err
		}
		m[origem] = origem
		return nil
	})
}

// IDsPorChave carrega o lookup chave → id a partir de uma consulta de DUAS
// colunas na ordem (id, chave) (ex.: "SELECT id, sku FROM produtos").
func IDsPorChave[K comparable](db cmdutil.DB, consulta string) (map[K]int64, error) {
	return lookup(db, consulta, func(rows *sql.Rows, m map[K]int64) error {
		var id int64
		var chave K
		if err := rows.Scan(&id, &chave); err != nil {
			return err
		}
		m[chave] = id
		return nil
	})
}

func lookup[K comparable](db cmdutil.DB, consulta string, ler func(*sql.Rows, map[K]int64) error) (map[K]int64, error) {
	rows, err := db.Query(consulta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[K]int64)
	for rows.Next() {
		if err := ler(rows, m); err != nil {
			return nil, err
		}
	}
	return m, rows.Err()
}

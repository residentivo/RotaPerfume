// Command resetpassword cria ou atualiza usuários no banco, com hash bcrypt válido.
// Garante que o admin sempre existe (cria se faltar) e que todos os placeholders
// são substituídos por hashes reais.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/resetpassword -list
//	cd apis/shared && go run ./cmd/resetpassword -email=admin@rotaperfumes.com.br -password=Senha123 -role=admin
//	cd apis/shared && go run ./cmd/resetpassword -all-users -password=SenhaPadrao123
//	cd apis/shared && go run ./cmd/resetpassword -create-admin -password=Admin@123
//
// Comportamento:
//   - -list: lista usuários com status do hash (PLACEHOLDER / OK / MISSING_ADMIN).
//   - -create-admin: garante que o admin existe (INSERT se não existe, UPDATE se existe).
//   - -email=...: UPSERT do usuário (cria se não existe, atualiza se existe).
//   - -all-users: substitui todos os hashes PLACEHOLDER pela senha informada.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	// Este binário não usa config.DSN(); importa tz diretamente para fixar
	// time.Local em -03:00 antes do sql.Open (RISCO-01).
	_ "github.com/rotaperfumes/shared/tz"
)

const (
	bcryptCost = 12
)

func main() {
	email := flag.String("email", "", "email do usuário a criar/atualizar")
	password := flag.String("password", "", "senha em texto puro (vazio = gera aleatória de 16 chars)")
	role := flag.String("role", "normal", "papel (admin|normal) — usado ao criar novo usuário")
	nome := flag.String("nome", "", "nome completo — usado ao criar novo usuário (padrão: derivado do email)")
	idVendedor := flag.Int64("id-vendedor", 0, "id do vendedor vinculado (opcional, usado ao criar)")
	allUsers := flag.Bool("all-users", false, "atualiza todos os PLACEHOLDER com a mesma senha")
	createAdmin := flag.Bool("create-admin", false, "garante que o admin principal existe (cria se faltar)")
	list := flag.Bool("list", false, "lista os usuários atuais")
	flag.Parse()

	loadEnvFromCwd()

	// Mesmo DSN de config.DSN() (ver lá a regra de clientFoundRows=true:
	// condições "só se ainda não ..." vão no WHERE, nunca deduzidas de
	// RowsAffected=0). Os upserts abaixo (UPDATE e, se 0 linhas, INSERT)
	// continuam corretos: com a flag, 0 significa "e-mail não existe".
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local&clientFoundRows=true",
		getEnv("DB_USUARIO", "golang"),
		getEnv("DB_SENHA", "golang"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_NAME", "rotaperfumes"),
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("resetpassword: sql.Open: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("resetpassword: ping: %v", err)
	}

	switch {
	case *list:
		if err := listUsers(ctx, db); err != nil {
			log.Fatalf("resetpassword: list: %v", err)
		}

	case *createAdmin:
		if *password == "" {
			// Prioridade: 1) env SEED_ADMIN_PASSWORD, 2) gera aleatória.
			if envPwd := os.Getenv("SEED_ADMIN_PASSWORD"); envPwd != "" {
				*password = envPwd
				log.Printf("resetpassword: usando SEED_ADMIN_PASSWORD do .env")
			} else {
				pwd, err := generateRandomPassword(16)
				if err != nil {
					log.Fatalf("gerar senha: %v", err)
				}
				*password = pwd
				fmt.Println("=== Senha gerada (guarde!) ===")
				fmt.Printf("ADMIN_PASSWORD=%s\n", pwd)
				fmt.Println("==============================")
			}
		}
		if err := upsertAdmin(ctx, db, *password); err != nil {
			log.Fatalf("resetpassword: create-admin: %v", err)
		}
		fmt.Println()
		if err := listUsers(ctx, db); err != nil {
			log.Printf("listar final: %v", err)
		}

	case *allUsers:
		if *password == "" {
			// Prioridade: 1) env SEED_USER_PASSWORD, 2) gera aleatória.
			if envPwd := os.Getenv("SEED_USER_PASSWORD"); envPwd != "" {
				*password = envPwd
				log.Printf("resetpassword: usando SEED_USER_PASSWORD do .env")
			} else {
				pwd, err := generateRandomPassword(16)
				if err != nil {
					log.Fatalf("gerar senha: %v", err)
				}
				*password = pwd
				fmt.Println("=== Senha gerada (todos os usuários PLACEHOLDER usarão esta) ===")
				fmt.Printf("USER_PASSWORD=%s\n", pwd)
				fmt.Println("================================================================")
			}
		}
		if err := updateAllPlaceholders(ctx, db, *password); err != nil {
			log.Fatalf("resetpassword: update all: %v", err)
		}
		fmt.Println()
		if err := listUsers(ctx, db); err != nil {
			log.Printf("listar final: %v", err)
		}

	case *email != "":
		if *password == "" {
			pwd, err := generateRandomPassword(16)
			if err != nil {
				log.Fatalf("gerar senha: %v", err)
			}
			*password = pwd
			fmt.Println("=== Senha gerada ===")
			fmt.Printf("PASSWORD=%s\n", pwd)
			fmt.Println("====================")
		}
		if err := upsertByEmail(ctx, db, *email, *password, *role, *nome, *idVendedor); err != nil {
			log.Fatalf("resetpassword: upsert: %v", err)
		}
		fmt.Println()
		if err := listUsers(ctx, db); err != nil {
			log.Printf("listar final: %v", err)
		}

	default:
		log.Fatalf("informe uma ação: -list | -create-admin | -all-users | -email=...")
	}
}

// upsertAdmin garante que admin@rotaperfumes.com.br existe e tem hash válido.
func upsertAdmin(ctx context.Context, db *sql.DB, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}
	hashStr := string(hash)

	// Tenta UPDATE primeiro; se afetou 0 linhas, faz INSERT.
	res, err := db.ExecContext(ctx,
		`UPDATE usuarios SET password_hash = ?, ativo = 1 WHERE email = ?`,
		hashStr, "admin@rotaperfumes.com.br",
	)
	if err != nil {
		return fmt.Errorf("update admin: %w", err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		log.Printf("resetpassword: admin atualizado (UPDATE)")
		return nil
	}

	// Admin não existe — INSERT.
	_, err = db.ExecContext(ctx, `
		INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo)
		VALUES (?, ?, ?, 'admin', NULL, 1)`,
		"Administrador Principal", "admin@rotaperfumes.com.br", hashStr,
	)
	if err != nil {
		return fmt.Errorf("insert admin: %w", err)
	}
	log.Printf("resetpassword: admin criado (INSERT)")
	return nil
}

// upsertByEmail cria ou atualiza um usuário.
func upsertByEmail(ctx context.Context, db *sql.DB, email, password, role, nome string, idVendedor int64) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}
	hashStr := string(hash)

	// UPDATE primeiro.
	res, err := db.ExecContext(ctx,
		`UPDATE usuarios SET password_hash = ? WHERE email = ?`,
		hashStr, email,
	)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		log.Printf("resetpassword: %s atualizado (UPDATE)", email)
		return nil
	}

	// Não existe — INSERT.
	if nome == "" {
		nome = "Usuário " + email
	}
	var idVend sql.NullInt64
	if idVendedor > 0 {
		idVend = sql.NullInt64{Int64: idVendedor, Valid: true}
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo)
		VALUES (?, ?, ?, ?, ?, 1)`,
		nome, email, hashStr, role, idVend,
	)
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}
	log.Printf("resetpassword: %s criado (INSERT)", email)
	return nil
}

// updateAllPlaceholders substitui todos os hashes com placeholder.
func updateAllPlaceholders(ctx context.Context, db *sql.DB, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}
	hashStr := string(hash)

	res, err := db.ExecContext(ctx,
		`UPDATE usuarios SET password_hash = ? WHERE password_hash LIKE '%PLACEHOLDER%'`,
		hashStr,
	)
	if err != nil {
		return fmt.Errorf("update all: %w", err)
	}
	n, _ := res.RowsAffected()
	log.Printf("resetpassword: %d linha(s) com placeholder atualizada(s)", n)

	// Garante que admin existe também.
	if err := ensureAdmin(ctx, db); err != nil {
		log.Printf("resetpassword: ensure-admin: %v", err)
	}
	return nil
}

// ensureAdmin garante que o admin existe (chamado pelo -all-users como salvaguarda).
func ensureAdmin(ctx context.Context, db *sql.DB) error {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usuarios WHERE email = 'admin@rotaperfumes.com.br')`).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	log.Printf("resetpassword: admin não existe — criando")
	// Sem senha: o admin fica inativo até alguém setar uma.
	_, err = db.ExecContext(ctx, `
		INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo)
		VALUES ('Administrador Principal', 'admin@rotaperfumes.com.br',
		        '$2a$12$NEEDS_RESET_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX', 'admin', NULL, 0)`)
	return err
}

func listUsers(ctx context.Context, db *sql.DB) error {
	// Agrupa admin no topo (ORDER BY role DESC, id ASC) para fácil visualização.
	rows, err := db.QueryContext(ctx, `
		SELECT id, nome, email, role, ativo,
		       LEFT(password_hash, 25) AS preview,
		       CASE
		           WHEN password_hash LIKE '%PLACEHOLDER%' THEN 'PLACEHOLDER'
		           WHEN password_hash LIKE '$2a$12$%' OR password_hash LIKE '$2b$12$%' THEN 'OK'
		           ELSE 'UNKNOWN'
		       END AS status
		FROM usuarios
		ORDER BY (role = 'admin') DESC, id ASC`,
	)
	if err != nil {
		return fmt.Errorf("list query: %w", err)
	}
	defer rows.Close()

	fmt.Printf("%-4s %-50s %-8s %-6s %-28s %s\n", "ID", "EMAIL", "ROLE", "ATIVO", "HASH_PREVIEW", "STATUS")
	fmt.Println(strings.Repeat("-", 120))
	count := 0
	placeholderCount := 0
	adminFound := false
	okCount := 0
	for rows.Next() {
		var id int64
		var nome, email, role, preview, status string
		var ativo int
		if err := rows.Scan(&id, &nome, &email, &role, &ativo, &preview, &status); err != nil {
			return fmt.Errorf("list scan: %w", err)
		}
		fmt.Printf("%-4d %-50s %-8s %-6d %-28s %s\n", id, truncate(email, 50), role, ativo, preview, status)
		count++
		switch status {
		case "PLACEHOLDER":
			placeholderCount++
		case "OK":
			okCount++
		}
		if email == "admin@rotaperfumes.com.br" {
			adminFound = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Println(strings.Repeat("-", 120))
	fmt.Printf("Total: %d usuarios | %d OK | %d PLACEHOLDER | admin existe: %v\n", count, okCount, placeholderCount, adminFound)
	if !adminFound {
		fmt.Println("⚠️  ATENÇÃO: admin@rotaperfumes.com.br NÃO EXISTE — rode: make fix-hash")
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func generateRandomPassword(n int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadEnvFromCwd() {
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
				_ = godotenv.Load(filepath.Join(dir, ".env"))
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
}

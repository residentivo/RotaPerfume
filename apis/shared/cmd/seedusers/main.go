// Command seedusers gera hashes bcrypt para os placeholders nos SQLs de seed
// e executa os arquivos contra o MySQL.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/seedusers
//
// Faz:
//  1. Lê senha real do env (ou defaults: Admin@123, Mudar@123).
//  2. Gera 2 hashes bcrypt com cost configurado em BCRYPT_COST (default 12).
//  3. Substitui placeholders nos SQLs: 02_seed_admin.sql e 03_seed_vendedores.sql.
//  4. Opcionalmente executa os SQLs atualizados (pula com --no-exec).
//  5. Reporta contadores e códigos de saída claros.
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

const (
	defaultAdminPassword = "Admin@123"
	defaultUserPassword  = "Mudar@123"
)

// generateRandomPassword retorna uma senha aleatória de 16 caracteres (a-zA-Z0-9).
// Usada quando o seed roda em modo dev sem credenciais definidas — em produção
// SEED_ADMIN_PASSWORD e SEED_USER_PASSWORD devem ser sempre fornecidos.
func generateRandomPassword(n int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, b := range bytes {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}

// Placeholders reconhecidos nos SQLs (devem existir literalmente nos arquivos).
const (
	placeholderAdmin = "$2a$12$XXXXPLACEHOLDER_ADMIN_PRECISA_SER_GERADO_PELO_GOXXXX"
	placeholderUser  = "$2a$12$XXXXPLACEHOLDER_MUDAR123_SERA_SUBSTITUIDO_PELO_GOXXXX"
)

func main() {
	noExec := flag.Bool("no-exec", false, "apenas substitui placeholders; não executa SQL no MySQL")
	dryRun := flag.Bool("dry-run", false, "imprime hashes gerados sem modificar arquivos")
	flag.Parse()

	// Carrega .env da raiz do projeto (sobe diretórios a partir de cwd).
	loadEnvFromCwd()

	// Carrega config (lê .env via os.Getenv; o Makefile exporta antes de chamar)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("seedusers: falha ao carregar config: %v", err)
	}

	auth := services.NewAuthService()
	// Modo dev (sem SEED_*_Password definidos): gera senhas aleatórias.
	// Modo prod: exige SEED_ADMIN_PASSWORD e SEED_USER_PASSWORD no .env.
	adminPwd, err := resolveSeedPassword("SEED_ADMIN_PASSWORD", defaultAdminPassword)
	if err != nil {
		log.Fatalf("seedusers: %v", err)
	}
	userPwd, err := resolveSeedPassword("SEED_USER_PASSWORD", defaultUserPassword)
	if err != nil {
		log.Fatalf("seedusers: %v", err)
	}

	adminHash, err := auth.HashPassword(cfg, adminPwd)
	if err != nil {
		log.Fatalf("seedusers: hash admin falhou: %v", err)
	}
	userHash, err := auth.HashPassword(cfg, userPwd)
	if err != nil {
		log.Fatalf("seedusers: hash user falhou: %v", err)
	}

	log.Printf("seedusers: cost=%d, admin_hash=%s..., user_hash=%s...",
		cfg.BCryptCost, shortHash(adminHash), shortHash(userHash))

	// Imprime as credenciais geradas (sempre, mesmo no dry-run) para dev.
	fmt.Println()
	fmt.Println("=== CREDENCIAIS DE SEED ===")
	fmt.Printf("ADMIN_PASSWORD=%s\n", adminPwd)
	fmt.Printf("USER_PASSWORD =%s\n", userPwd)
	fmt.Println("============================")
	fmt.Println()

	if *dryRun {
		fmt.Println("=== DRY-RUN ===")
		fmt.Println("ADMIN:", adminHash)
		fmt.Println("USER :", userHash)
		return
	}

	// Localiza a raiz do projeto (sobe 2 níveis: cmd/seedusers → shared → apis → raiz).
	projectRoot, err := findProjectRoot()
	if err != nil {
		log.Fatalf("seedusers: %v", err)
	}

	sqlFiles := []string{
		filepath.Join(projectRoot, "sql", "02_seed_admin.sql"),
		filepath.Join(projectRoot, "sql", "03_seed_vendedores.sql"),
	}

	totalReplacements := 0
	for _, f := range sqlFiles {
		n, err := replaceInFile(f, map[string]string{
			placeholderAdmin: adminHash,
			placeholderUser:  userHash,
		})
		if err != nil {
			log.Fatalf("seedusers: erro em %s: %v", f, err)
		}
		log.Printf("seedusers: %s → %d substituições", filepath.Base(f), n)
		totalReplacements += n
	}

	log.Printf("seedusers: total de placeholders substituídos: %d", totalReplacements)

	if *noExec {
		log.Printf("seedusers: --no-exec informado, SQLs NÃO foram executados")
		return
	}

	// Executa os SQLs atualizados via mysql CLI.
	for _, f := range sqlFiles {
		log.Printf("seedusers: executando %s", filepath.Base(f))
		if err := runMySQL(cfg, f); err != nil {
			log.Fatalf("seedusers: mysql falhou em %s: %v", f, err)
		}
	}

	log.Printf("seedusers: OK — %d arquivos SQL aplicados", len(sqlFiles))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// resolveSeedPassword retorna a senha de seed. Se a env não estiver definida,
// gera uma senha aleatória (modo dev). Em produção, defina SEED_ADMIN_PASSWORD
// e SEED_USER_PASSWORD explicitamente no .env — sem elas, o dev não tem como
// logar.
func resolveSeedPassword(envKey, defaultValue string) (string, error) {
	if v := os.Getenv(envKey); v != "" {
		return v, nil
	}
	log.Printf("seedusers: %s não definido — gerando senha aleatória (modo dev)", envKey)
	return generateRandomPassword(16)
}

func shortHash(h string) string {
	if len(h) > 20 {
		return h[:20]
	}
	return h
}

// findProjectRoot sobe a árvore a partir do executável até achar go.mod de shared
// e retorna o diretório-pai (raiz do projeto SistemaCompleto).
func findProjectRoot() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	// Quando executado via `go run`, o exe vive em /tmp; nesse caso usamos cwd.
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		dir, _ = os.Getwd()
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// se houver "apis/shared/go.mod" → estamos na raiz.
			if _, err2 := os.Stat(filepath.Join(dir, "apis", "shared", "go.mod")); err2 == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fallback: assume 2 níveis acima do cwd (apis/shared/cmd/seedusers).
	cwd, _ := os.Getwd()
	return filepath.Clean(filepath.Join(cwd, "..", "..", "..")), nil
}

// replaceInFile substitui todas as ocorrências (mapa) e grava o arquivo in-place.
// Retorna o número total de substituições.
func replaceInFile(path string, repl map[string]string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	content := string(data)
	total := 0
	for old, new := range repl {
		count := strings.Count(content, old)
		if count == 0 {
			continue
		}
		content = strings.ReplaceAll(content, old, new)
		total += count
	}
	if total == 0 {
		return 0, nil
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return 0, err
	}
	return total, nil
}

// runMySQL executa um arquivo SQL via cliente mysql, lendo o conteúdo via stdin.
func runMySQL(cfg *config.Config, file string) error {
	args := []string{
		fmt.Sprintf("-u%s", cfg.DBUsuario),
		fmt.Sprintf("-p%s", cfg.DBSenha),
		fmt.Sprintf("-h%s", cfg.DBHost),
		fmt.Sprintf("-P%s", cfg.DBPort),
		"--default-character-set=utf8mb4",
		"--local-infile=1",
		cfg.DBName,
	}
	sqlBytes, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("leitura de %s: %w", file, err)
	}
	cmd := exec.Command("mysql", args...)
	cmd.Stdin = strings.NewReader(string(sqlBytes))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// loadEnvFromCwd tenta carregar o .env da raiz do projeto subindo diretórios
// a partir do working directory. Funciona tanto em `go run` quanto em binário compilado.
// Não retorna erro — se não achar, segue sem .env.
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

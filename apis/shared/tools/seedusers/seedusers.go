// Package seedusers implementa o comando cmd/seedusers: gera hashes bcrypt
// para os placeholders nos SQLs de seed e executa os arquivos contra o MySQL.
//
// Uso (via comando):
//
//	cd apis/shared && go run ./cmd/seedusers
//
// Faz:
//  1. Lê senha real do env (ou defaults: Admin@123, Mudar@123).
//  2. Gera 2 hashes bcrypt com cost configurado em BCRYPT_COST (default 12).
//  3. Substitui placeholders nos SQLs: 02_seed_admin.sql e 03_seed_vendedores.sql.
//  4. Opcionalmente executa os SQLs atualizados (pula com --no-exec).
//  5. Reporta contadores e códigos de saída claros.
package seedusers

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

// Tag prefixa os logs e as mensagens de erro do comando.
const Tag = "seedusers"

const (
	defaultAdminPassword = "Admin@123"
	defaultUserPassword  = "Mudar@123"
)

// Placeholders reconhecidos nos SQLs (devem existir literalmente nos arquivos).
const (
	PlaceholderAdmin = "$2a$12$XXXXPLACEHOLDER_ADMIN_PRECISA_SER_GERADO_PELO_GOXXXX"
	PlaceholderUser  = "$2a$12$XXXXPLACEHOLDER_MUDAR123_SERA_SUBSTITUIDO_PELO_GOXXXX"
)

// Options reúne as flags do comando.
type Options struct {
	NoExec       bool // -no-exec: só substitui placeholders
	DryRun       bool // -dry-run: só imprime os hashes
	ShowPassword bool // -show-password: imprime as senhas em claro
}

// SQLRunner executa um arquivo SQL no banco de cfg (RunMySQL no comando).
type SQLRunner func(cfg *config.Config, file string) error

// Deps agrupa as dependências externas do Run, substituíveis em testes.
type Deps struct {
	// Out recebe a saída de console (os.Stdout no comando).
	Out io.Writer
	// ProjectRoot localiza a raiz do repositório (FindProjectRoot no comando).
	ProjectRoot func() (string, error)
	// RunSQL executa cada SQL atualizado (RunMySQL no comando).
	RunSQL SQLRunner
}

// Run gera os hashes, substitui os placeholders nos SQLs de seed e (salvo
// NoExec/DryRun) executa os SQLs. Erros fatais são devolvidos sem o prefixo Tag.
func Run(cfg *config.Config, opts Options, deps Deps) error {
	out := deps.Out
	auth := services.NewAuthService()
	// Modo dev (sem SEED_*_Password definidos): gera senhas aleatórias.
	// Modo prod: exige SEED_ADMIN_PASSWORD e SEED_USER_PASSWORD no .env.
	adminPwd, err := ResolveSeedPassword("SEED_ADMIN_PASSWORD", defaultAdminPassword)
	if err != nil {
		return err
	}
	userPwd, err := ResolveSeedPassword("SEED_USER_PASSWORD", defaultUserPassword)
	if err != nil {
		return err
	}

	adminHash, err := auth.HashPassword(cfg, adminPwd)
	if err != nil {
		return fmt.Errorf("hash admin falhou: %w", err)
	}
	userHash, err := auth.HashPassword(cfg, userPwd)
	if err != nil {
		return fmt.Errorf("hash user falhou: %w", err)
	}

	log.Printf("seedusers: cost=%d, admin_hash=%s..., user_hash=%s...",
		cfg.BCryptCost, ShortHash(adminHash), ShortHash(userHash))

	// As senhas de seed só são impressas em claro no console quando
	// --show-password é passado explicitamente — evita vazamento acidental
	// em logs de CI/terminal compartilhado.
	fmt.Fprintln(out)
	if opts.ShowPassword {
		fmt.Fprintln(out, "=== CREDENCIAIS DE SEED ===")
		fmt.Fprintf(out, "ADMIN_PASSWORD=%s\n", adminPwd)
		fmt.Fprintf(out, "USER_PASSWORD =%s\n", userPwd)
		fmt.Fprintln(out, "============================")
	} else {
		fmt.Fprintln(out, "=== CREDENCIAIS DE SEED: definidas com sucesso (use --show-password para exibir) ===")
	}
	fmt.Fprintln(out)

	if opts.DryRun {
		fmt.Fprintln(out, "=== DRY-RUN ===")
		fmt.Fprintln(out, "ADMIN:", adminHash)
		fmt.Fprintln(out, "USER :", userHash)
		return nil
	}

	// Localiza a raiz do projeto (sobe 2 níveis: cmd/seedusers → shared → apis → raiz).
	projectRoot, err := deps.ProjectRoot()
	if err != nil {
		return err
	}

	sqlFiles := []string{
		filepath.Join(projectRoot, "sql", "02_seed_admin.sql"),
		filepath.Join(projectRoot, "sql", "03_seed_vendedores.sql"),
	}

	totalReplacements := 0
	for _, f := range sqlFiles {
		n, err := ReplaceInFile(f, map[string]string{
			PlaceholderAdmin: adminHash,
			PlaceholderUser:  userHash,
		})
		if err != nil {
			return fmt.Errorf("erro em %s: %w", f, err)
		}
		log.Printf("seedusers: %s → %d substituições", filepath.Base(f), n)
		totalReplacements += n
	}

	log.Printf("seedusers: total de placeholders substituídos: %d", totalReplacements)

	if opts.NoExec {
		log.Printf("seedusers: --no-exec informado, SQLs NÃO foram executados")
		return nil
	}

	// Executa os SQLs atualizados via mysql CLI.
	for _, f := range sqlFiles {
		log.Printf("seedusers: executando %s", filepath.Base(f))
		if err := deps.RunSQL(cfg, f); err != nil {
			return fmt.Errorf("mysql falhou em %s: %w", f, err)
		}
	}

	log.Printf("seedusers: OK — %d arquivos SQL aplicados", len(sqlFiles))
	return nil
}

// GenerateRandomPassword retorna uma senha aleatória de n caracteres (a-zA-Z0-9).
// Usada quando o seed roda em modo dev sem credenciais definidas — em produção
// SEED_ADMIN_PASSWORD e SEED_USER_PASSWORD devem ser sempre fornecidos.
func GenerateRandomPassword(n int) (string, error) {
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

// ResolveSeedPassword retorna a senha de seed. Se a env não estiver definida,
// gera uma senha aleatória (modo dev). Em produção, defina SEED_ADMIN_PASSWORD
// e SEED_USER_PASSWORD explicitamente no .env — sem elas, o dev não tem como
// logar.
func ResolveSeedPassword(envKey, defaultValue string) (string, error) {
	if v := os.Getenv(envKey); v != "" {
		return v, nil
	}
	log.Printf("seedusers: %s não definido — gerando senha aleatória (modo dev)", envKey)
	return GenerateRandomPassword(16)
}

// ShortHash devolve os 20 primeiros caracteres do hash (para log).
func ShortHash(h string) string {
	if len(h) > 20 {
		return h[:20]
	}
	return h
}

// FindProjectRoot sobe a árvore a partir do executável até achar go.mod de shared
// e retorna o diretório-pai (raiz do projeto SistemaCompleto).
func FindProjectRoot() (string, error) {
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

// ReplaceInFile substitui todas as ocorrências (mapa) e grava o arquivo in-place.
// Retorna o número total de substituições.
func ReplaceInFile(path string, repl map[string]string) (int, error) {
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

// RunMySQL executa um arquivo SQL via cliente mysql, lendo o conteúdo via stdin.
// A senha do banco é passada via variável de ambiente MYSQL_PWD (lida
// nativamente pelo cliente mysql) em vez de argumento de linha de comando
// (-p senha), que ficaria visível para outros processos/usuários do sistema
// via `ps`/Task Manager e no histórico de shell.
func RunMySQL(cfg *config.Config, file string) error {
	sqlBytes, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("leitura de %s: %w", file, err)
	}
	cmd := exec.Command("mysql", MySQLArgs(cfg)...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("MYSQL_PWD=%s", cfg.DBSenha))
	cmd.Stdin = strings.NewReader(string(sqlBytes))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// MySQLArgs monta os argumentos do cliente mysql (sem a senha, que vai via
// MYSQL_PWD).
func MySQLArgs(cfg *config.Config) []string {
	return []string{
		fmt.Sprintf("-u%s", cfg.DBUsuario),
		fmt.Sprintf("-h%s", cfg.DBHost),
		fmt.Sprintf("-P%s", cfg.DBPort),
		"--default-character-set=utf8mb4",
		"--local-infile=1",
		cfg.DBName,
	}
}

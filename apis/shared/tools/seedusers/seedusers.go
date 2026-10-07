// Package seedusers implementa o comando cmd/seedusers: gera hashes Argon2id (com pepper)
// para os placeholders dos SQLs de seed, grava cópias renderizadas em
// <raiz>/tmp/seed e executa essas cópias contra o MySQL. Os arquivos de sql/
// nunca são alterados (SEC-10).
//
// Uso (via comando):
//
//	cd apis/shared && go run ./cmd/seedusers               # renderiza, executa e apaga tmp/seed
//	cd apis/shared && go run ./cmd/seedusers -no-exec      # só renderiza em tmp/seed (apague depois!)
//	cd apis/shared && go run ./cmd/seedusers -dry-run      # só imprime os hashes
//
// Faz:
//  1. Lê as senhas de SEED_ADMIN_PASSWORD e SEED_USER_PASSWORD; a que não
//     estiver definida vira uma senha aleatória de 16 caracteres (modo dev).
//     Não há senha padrão fixa.
//  2. Gera 2 hashes Argon2id com o pepper PASSWORD_PEPPER e os parâmetros
//     ARGON2_* do .env (o pepper do servidor precisa ser o mesmo).
//  3. Lê sql/02_seed_admin.sql e sql/03_seed_vendedores.sql, substitui os
//     placeholders e grava as cópias em <raiz>/tmp/seed (perm 0600).
//  4. Executa as cópias (exige DB_USUARIO e DB_SENHA no .env) e as apaga ao
//     final, mesmo em erro. Com -no-exec as cópias ficam em tmp/seed.
//  5. Reporta contadores e códigos de saída claros.
package seedusers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

// Tag prefixa os logs e as mensagens de erro do comando.
const Tag = "seedusers"

// Placeholders reconhecidos nos SQLs (devem existir literalmente nos arquivos).
const (
	PlaceholderAdmin = "$2a$12$XXXXPLACEHOLDER_ADMIN_PRECISA_SER_GERADO_PELO_GOXXXX"
	PlaceholderUser  = "$2a$12$XXXXPLACEHOLDER_MUDAR123_SERA_SUBSTITUIDO_PELO_GOXXXX"
)

// ErrSemPlaceholder indica um SQL de seed sem nenhum placeholder conhecido.
var ErrSemPlaceholder = errors.New("sem placeholder (arquivo já contém hash real? restaure com git checkout)")

// ErrCredenciaisDB indica DB_USUARIO/DB_SENHA ausentes ou vazios no ambiente.
var ErrCredenciaisDB = errors.New("defina DB_USUARIO/DB_SENHA no .env")

// Options reúne as flags do comando.
type Options struct {
	NoExec       bool // -no-exec: só renderiza os SQLs em tmp/seed
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
	// RunSQL executa cada SQL renderizado (RunMySQL no comando).
	RunSQL SQLRunner
}

// Run gera os hashes, renderiza os SQLs de seed em <raiz>/tmp/seed e (salvo
// NoExec/DryRun) executa as cópias, apagando-as ao final. Erros fatais são
// devolvidos sem o prefixo Tag.
func Run(cfg *config.Config, opts Options, deps Deps) error {
	out := deps.Out
	vaiExecutar := !opts.NoExec && !opts.DryRun
	if vaiExecutar {
		// Falha cedo, antes do hash: sem credenciais explícitas o mysql
		// rodaria com um usuário/senha padrão (SEC-10).
		if err := exigirCredenciaisDB(); err != nil {
			return err
		}
	}

	adminPwd, err := ResolveSeedPassword("SEED_ADMIN_PASSWORD")
	if err != nil {
		return err
	}
	userPwd, err := ResolveSeedPassword("SEED_USER_PASSWORD")
	if err != nil {
		return err
	}

	auth := services.NewAuthService()
	adminHash, err := auth.HashPassword(cfg, adminPwd)
	if err != nil {
		return fmt.Errorf("hash admin falhou: %w", err)
	}
	userHash, err := auth.HashPassword(cfg, userPwd)
	if err != nil {
		return fmt.Errorf("hash user falhou: %w", err)
	}

	log.Printf("seedusers: argon2id m=%d t=%d p=%d, admin_hash=%s..., user_hash=%s...",
		cfg.HashSenha.MemoriaKiB, cfg.HashSenha.Iteracoes, cfg.HashSenha.Paralelismo, ShortHash(adminHash), ShortHash(userHash))

	imprimirCredenciais(out, opts.ShowPassword, adminPwd, userPwd)

	if opts.DryRun {
		fmt.Fprintln(out, "=== DRY-RUN ===")
		fmt.Fprintln(out, "ADMIN:", adminHash)
		fmt.Fprintln(out, "USER :", userHash)
		return nil
	}

	// Localiza a raiz do projeto (pasta que contém apis/shared/go.mod).
	projectRoot, err := deps.ProjectRoot()
	if err != nil {
		return err
	}

	repl := map[string]string{PlaceholderAdmin: adminHash, PlaceholderUser: userHash}
	seedDir := filepath.Join(projectRoot, "tmp", "seed")
	var renderizados []string
	if vaiExecutar {
		// As cópias contêm hashes reais: apaga sempre, inclusive em erro.
		defer func() { apagarRenderizados(renderizados) }()
	}

	total := 0
	for _, src := range SeedFiles(projectRoot) {
		dst, n, err := RenderSeedFile(src, seedDir, repl)
		if err != nil {
			return fmt.Errorf("erro em %s: %w", src, err)
		}
		renderizados = append(renderizados, dst)
		log.Printf("seedusers: %s → %d substituições (%s)", filepath.Base(src), n, dst)
		total += n
	}
	log.Printf("seedusers: total de placeholders substituídos: %d", total)

	if opts.NoExec {
		avisarNoExec(out, renderizados)
		return nil
	}

	for _, f := range renderizados {
		log.Printf("seedusers: executando %s", filepath.Base(f))
		if err := deps.RunSQL(cfg, f); err != nil {
			return fmt.Errorf("mysql falhou em %s: %w", f, err)
		}
	}

	log.Printf("seedusers: OK — %d arquivos SQL aplicados", len(renderizados))
	return nil
}

// SeedFiles devolve os SQLs de seed (em sql/) na ordem de execução.
func SeedFiles(projectRoot string) []string {
	return []string{
		filepath.Join(projectRoot, "sql", "02_seed_admin.sql"),
		filepath.Join(projectRoot, "sql", "03_seed_vendedores.sql"),
	}
}

// exigirCredenciaisDB exige DB_USUARIO e DB_SENHA presentes e não vazios.
func exigirCredenciaisDB() error {
	for _, k := range []string{"DB_USUARIO", "DB_SENHA"} {
		if v, ok := os.LookupEnv(k); !ok || v == "" {
			return ErrCredenciaisDB
		}
	}
	return nil
}

// imprimirCredenciais só mostra as senhas em claro com --show-password —
// evita vazamento acidental em logs de CI/terminal compartilhado.
func imprimirCredenciais(out io.Writer, mostrar bool, adminPwd, userPwd string) {
	fmt.Fprintln(out)
	if mostrar {
		fmt.Fprintln(out, "=== CREDENCIAIS DE SEED ===")
		fmt.Fprintf(out, "ADMIN_PASSWORD=%s\n", adminPwd)
		fmt.Fprintf(out, "USER_PASSWORD =%s\n", userPwd)
		fmt.Fprintln(out, "============================")
	} else {
		fmt.Fprintln(out, "=== CREDENCIAIS DE SEED: definidas com sucesso (use --show-password para exibir) ===")
	}
	fmt.Fprintln(out)
}

// avisarNoExec lista os SQLs renderizados mantidos em disco e pede que sejam
// apagados após o uso (contêm hashes Argon2id das senhas de seed).
func avisarNoExec(out io.Writer, arquivos []string) {
	log.Printf("seedusers: --no-exec informado, SQLs NÃO foram executados")
	fmt.Fprintln(out, "SQLs renderizados (NÃO executados):")
	for _, f := range arquivos {
		fmt.Fprintf(out, "  %s\n", f)
	}
	fmt.Fprintln(out, "ATENÇÃO: esses arquivos contêm hashes Argon2id das senhas de seed; apague-os após o uso.")
}

// apagarRenderizados remove as cópias renderizadas; falha só gera log.
func apagarRenderizados(arquivos []string) {
	for _, f := range arquivos {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("seedusers: não foi possível apagar %s: %v (apague manualmente)", f, err)
		}
	}
}

// GenerateRandomPassword retorna uma senha aleatória de n caracteres
// [a-zA-Z0-9], uniforme (delega a services.SenhaAlfanumerica). Usada quando o
// seed roda em modo dev sem credenciais definidas — em produção
// SEED_ADMIN_PASSWORD e SEED_USER_PASSWORD devem ser sempre fornecidos.
func GenerateRandomPassword(n int) (string, error) {
	return services.SenhaAlfanumerica(n)
}

// ResolveSeedPassword retorna a senha de seed da env envKey. Se ela não
// estiver definida (ou vazia), gera uma senha aleatória de 16 caracteres
// (modo dev) — não existe senha padrão fixa. Em produção, defina
// SEED_ADMIN_PASSWORD e SEED_USER_PASSWORD no .env (ou rode com
// -show-password para ver as geradas; sem isso, o dev não tem como logar).
func ResolveSeedPassword(envKey string) (string, error) {
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

// FindProjectRoot localiza a raiz do repositório (a pasta que contém
// apis/shared/go.mod) subindo a partir do cwd. Delega para
// cmdutil.FindProjectRoot: a raiz do SistemaCompleto não tem go.mod próprio,
// e a busca antiga (go.mod na raiz + fallback cwd/../../..) apontava para a
// pasta errada ao rodar de apis/shared (BUG-10). Sem raiz, devolve erro.
func FindProjectRoot() (string, error) {
	return cmdutil.FindProjectRoot()
}

// RenderSeedFile lê src, substitui todas as ocorrências das chaves de repl e
// grava o resultado em dstDir/<nome de src> com permissão 0600 (dstDir é
// criado com 0700 se faltar). src nunca é alterado. Retorna o caminho gravado
// e o total de substituições; sem nenhuma ocorrência devolve
// ErrSemPlaceholder e não grava nada.
func RenderSeedFile(src, dstDir string, repl map[string]string) (dst string, n int, err error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", 0, err
	}
	content := string(data)
	for old, novo := range repl {
		count := strings.Count(content, old)
		if count == 0 {
			continue
		}
		content = strings.ReplaceAll(content, old, novo)
		n += count
	}
	if n == 0 {
		return "", 0, ErrSemPlaceholder
	}
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return "", 0, err
	}
	dst = filepath.Join(dstDir, filepath.Base(src))
	if err := os.WriteFile(dst, []byte(content), 0o600); err != nil {
		return "", 0, err
	}
	// WriteFile não altera a permissão de um arquivo que já existia.
	if err := os.Chmod(dst, 0o600); err != nil {
		return "", 0, err
	}
	return dst, n, nil
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

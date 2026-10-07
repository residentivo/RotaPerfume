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
	"github.com/rotaperfumes/shared/vlog"
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
	vlog.Printf("seedusers.go", "Run", "declarando out com deps.Out")
	out := deps.Out
	vlog.Printf("seedusers.go", "Run", "declarando vaiExecutar com expressão !opts.NoExec && !opts.DryRun")
	vaiExecutar := !opts.NoExec && !opts.DryRun
	vlog.Printf("seedusers.go", "Run", "verificando se vaiExecutar")
	if vaiExecutar {
		// Falha cedo, antes do hash: sem credenciais explícitas o mysql
		// rodaria com um usuário/senha padrão (SEC-10).
		vlog.Printf("seedusers.go", "Run", "chamando exigirCredenciaisDB() e verificando se err != nil")
		if err := exigirCredenciaisDB(); err != nil {
			return err
		}
	}

	vlog.Printf("seedusers.go", "Run", "declarando adminPwd, err com resultado de ResolveSeedPassword()")
	adminPwd, err := ResolveSeedPassword("SEED_ADMIN_PASSWORD")
	vlog.Printf("seedusers.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}
	vlog.Printf("seedusers.go", "Run", "declarando userPwd, err com resultado de ResolveSeedPassword()")
	userPwd, err := ResolveSeedPassword("SEED_USER_PASSWORD")
	vlog.Printf("seedusers.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}

	vlog.Printf("seedusers.go", "Run", "declarando auth com resultado de services.NewAuthService()")
	auth := services.NewAuthService()
	vlog.Printf("seedusers.go", "Run", "declarando adminHash, err com resultado de auth.HashPassword()")
	adminHash, err := auth.HashPassword(cfg, adminPwd)
	vlog.Printf("seedusers.go", "Run", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("hash admin falhou: %w", err)
	}
	vlog.Printf("seedusers.go", "Run", "declarando userHash, err com resultado de auth.HashPassword()")
	userHash, err := auth.HashPassword(cfg, userPwd)
	vlog.Printf("seedusers.go", "Run", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("hash user falhou: %w", err)
	}

	log.Printf("seedusers: argon2id m=%d t=%d p=%d, admin_hash=%s..., user_hash=%s...",
		cfg.HashSenha.MemoriaKiB, cfg.HashSenha.Iteracoes, cfg.HashSenha.Paralelismo, ShortHash(adminHash), ShortHash(userHash))

	vlog.Printf("seedusers.go", "Run", "chamando imprimirCredenciais()")
	imprimirCredenciais(out, opts.ShowPassword, adminPwd, userPwd)

	vlog.Printf("seedusers.go", "Run", "verificando se opts.DryRun")
	if opts.DryRun {
		fmt.Fprintln(out, "=== DRY-RUN ===")
		fmt.Fprintln(out, "ADMIN:", adminHash)
		fmt.Fprintln(out, "USER :", userHash)
		return nil
	}

	// Localiza a raiz do projeto (pasta que contém apis/shared/go.mod).
	vlog.Printf("seedusers.go", "Run", "declarando projectRoot, err com resultado de deps.ProjectRoot()")
	projectRoot, err := deps.ProjectRoot()
	vlog.Printf("seedusers.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}

	vlog.Printf("seedusers.go", "Run", "declarando repl com literal map[string]string")
	repl := map[string]string{PlaceholderAdmin: adminHash, PlaceholderUser: userHash}
	vlog.Printf("seedusers.go", "Run", "declarando seedDir com resultado de filepath.Join()")
	seedDir := filepath.Join(projectRoot, "tmp", "seed")
	vlog.Printf("seedusers.go", "Run", "declarando variável renderizados")
	var renderizados []string
	vlog.Printf("seedusers.go", "Run", "verificando se vaiExecutar")
	if vaiExecutar {
		// As cópias contêm hashes reais: apaga sempre, inclusive em erro.
		vlog.Printf("seedusers.go", "Run", "agendando defer de função anônima()")
		defer func() { apagarRenderizados(renderizados) }()
	}

	vlog.Printf("seedusers.go", "Run", "declarando total com valor literal")
	total := 0
	vlog.Printf("seedusers.go", "Run", "iniciando loop range sobre resultado de SeedFiles()")
	for _, src := range SeedFiles(projectRoot) {
		dst, n, err := RenderSeedFile(src, seedDir, repl)
		if err != nil {
			return fmt.Errorf("erro em %s: %w", src, err)
		}
		renderizados = append(renderizados, dst)
		log.Printf("seedusers: %s → %d substituições (%s)", filepath.Base(src), n, dst)
		total += n
	}
	vlog.Printf("seedusers.go", "Run", "loop concluído; placeholders substituídos: %d", total)
	log.Printf("seedusers: total de placeholders substituídos: %d", total)

	vlog.Printf("seedusers.go", "Run", "verificando se opts.NoExec")
	if opts.NoExec {
		vlog.Printf("seedusers.go", "Run", "chamando avisarNoExec()")
		avisarNoExec(out, renderizados)
		return nil
	}

	vlog.Printf("seedusers.go", "Run", "iniciando loop range sobre renderizados")
	for _, f := range renderizados {
		log.Printf("seedusers: executando %s", filepath.Base(f))
		if err := deps.RunSQL(cfg, f); err != nil {
			return fmt.Errorf("mysql falhou em %s: %w", f, err)
		}
	}
	vlog.Printf("seedusers.go", "Run", "loop concluído; itens: %d", len(renderizados))

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
	vlog.Printf("seedusers.go", "exigirCredenciaisDB", "iniciando loop range sobre literal []string")
	for _, k := range []string{"DB_USUARIO", "DB_SENHA"} {
		if v, ok := os.LookupEnv(k); !ok || v == "" {
			return ErrCredenciaisDB
		}
	}
	vlog.Printf("seedusers.go", "exigirCredenciaisDB", "loop concluído")
	return nil
}

// imprimirCredenciais só mostra as senhas em claro com --show-password —
// evita vazamento acidental em logs de CI/terminal compartilhado.
func imprimirCredenciais(out io.Writer, mostrar bool, adminPwd, userPwd string) {
	fmt.Fprintln(out)
	vlog.Printf("seedusers.go", "imprimirCredenciais", "verificando se mostrar")
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
	vlog.Printf("seedusers.go", "avisarNoExec", "iniciando loop range sobre arquivos")
	for _, f := range arquivos {
		fmt.Fprintf(out, "  %s\n", f)
	}
	vlog.Printf("seedusers.go", "avisarNoExec", "loop concluído; itens: %d", len(arquivos))
	fmt.Fprintln(out, "ATENÇÃO: esses arquivos contêm hashes Argon2id das senhas de seed; apague-os após o uso.")
}

// apagarRenderizados remove as cópias renderizadas; falha só gera log.
func apagarRenderizados(arquivos []string) {
	vlog.Printf("seedusers.go", "apagarRenderizados", "iniciando loop range sobre arquivos")
	for _, f := range arquivos {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("seedusers: não foi possível apagar %s: %v (apague manualmente)", f, err)
		}
	}
	vlog.Printf("seedusers.go", "apagarRenderizados", "loop concluído; itens: %d", len(arquivos))
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
	vlog.Printf("seedusers.go", "ResolveSeedPassword", "chamando os.Getenv() e verificando condição do if")
	if v := os.Getenv(envKey); v != "" {
		return v, nil
	}
	log.Printf("seedusers: %s não definido — gerando senha aleatória (modo dev)", envKey)
	return GenerateRandomPassword(16)
}

// ShortHash devolve os 20 primeiros caracteres do hash (para log).
func ShortHash(h string) string {
	vlog.Printf("seedusers.go", "ShortHash", "verificando se len(h) > 20")
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
	vlog.Printf("seedusers.go", "RunMySQL", "declarando sqlBytes, err com resultado de os.ReadFile()")
	sqlBytes, err := os.ReadFile(file)
	vlog.Printf("seedusers.go", "RunMySQL", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("leitura de %s: %w", file, err)
	}
	vlog.Printf("seedusers.go", "RunMySQL", "declarando cmd com resultado de exec.Command()")
	cmd := exec.Command("mysql", MySQLArgs(cfg)...)
	vlog.Printf("seedusers.go", "RunMySQL", "atribuindo a cmd.Env o valor de resultado de append()")
	cmd.Env = append(os.Environ(), fmt.Sprintf("MYSQL_PWD=%s", cfg.DBSenha))
	vlog.Printf("seedusers.go", "RunMySQL", "atribuindo a cmd.Stdin o valor de resultado de strings.NewReader()")
	cmd.Stdin = strings.NewReader(string(sqlBytes))
	vlog.Printf("seedusers.go", "RunMySQL", "atribuindo a cmd.Stdout o valor de os.Stdout")
	cmd.Stdout = os.Stdout
	vlog.Printf("seedusers.go", "RunMySQL", "atribuindo a cmd.Stderr o valor de os.Stderr")
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

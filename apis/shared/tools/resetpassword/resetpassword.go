// Package resetpassword implementa o comando cmd/resetpassword: cria ou
// atualiza usuários no banco, com hash bcrypt válido. Garante que o admin
// sempre existe (cria se faltar) e que todos os placeholders são
// substituídos por hashes reais.
//
// Uso (via comando; exige DB_USUARIO e DB_SENHA no .env):
//
//	cd apis/shared && go run ./cmd/resetpassword -list
//	cd apis/shared && go run ./cmd/resetpassword -email=admin@rotaperfumes.com.br -password-prompt -role=admin
//	printf '%s\n' "$NOVA_SENHA" | go run ./cmd/resetpassword -all-users -password-stdin
//	cd apis/shared && go run ./cmd/resetpassword -create-admin   # SEED_ADMIN_PASSWORD do .env ou aleatória
//
// Fontes de senha (no máximo uma):
//   - -password-prompt: lê do terminal sem eco, com confirmação. No Git Bash
//     (mintty) o stdin não é um terminal: use `winpty go run ...` ou
//     -password-stdin.
//   - -password-stdin: lê a 1ª linha do stdin (sem o \r\n final).
//   - -password=...: DEPRECIADA (fica no histórico do shell); ainda aceita,
//     com aviso no stderr.
//   - nenhuma: SEED_ADMIN_PASSWORD / SEED_USER_PASSWORD do .env
//     (-create-admin / -all-users) ou senha aleatória impressa no console.
//
// Comportamento:
//   - -list: lista usuários com status do hash (PLACEHOLDER / OK / MISSING_ADMIN).
//   - -create-admin: garante que o admin existe (INSERT se não existe, UPDATE se existe).
//   - -email=...: UPSERT do usuário (cria se não existe, atualiza se existe).
//   - -all-users: substitui todos os hashes PLACEHOLDER pela senha informada.
package resetpassword

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/rotaperfumes/shared/services"
)

// BcryptCost é o custo bcrypt dos hashes gravados pelo comando.
const BcryptCost = 12

// AdminEmail é o e-mail do admin principal garantido pelo comando.
const AdminEmail = "admin@rotaperfumes.com.br"

// DB é o subconjunto de *sql.DB usado pelo comando (satisfeito por *sql.DB,
// *sql.Tx e pelo *sql.DB do sqlmock).
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Options reúne as flags do comando.
type Options struct {
	Email       string // -email: usuário a criar/atualizar
	Password    string // -password: senha em texto puro (DEPRECIADA; vazio = env/aleatória)
	Role        string // -role: papel ao criar novo usuário
	Nome        string // -nome: nome ao criar novo usuário
	IDVendedor  int64  // -id-vendedor: vendedor vinculado ao criar
	AllUsers    bool   // -all-users
	CreateAdmin bool   // -create-admin
	List        bool   // -list

	PasswordStdin  bool // -password-stdin: lê a senha da 1ª linha do stdin
	PasswordPrompt bool // -password-prompt: lê a senha do terminal, sem eco
}

// PasswordGenerator gera uma senha aleatória de n caracteres.
type PasswordGenerator func(n int) (string, error)

// ErrCredenciaisDB indica DB_USUARIO/DB_SENHA ausentes ou vazios no ambiente.
var ErrCredenciaisDB = errors.New("defina DB_USUARIO/DB_SENHA no .env")

// AvisoPasswordDepreciada é escrito no stderr quando -password é usada.
const AvisoPasswordDepreciada = "resetpassword: aviso: -password fica no histórico do shell; prefira -password-prompt ou -password-stdin (flag depreciada)"

// DSN monta o DSN a partir das variáveis de ambiente DB_*. DB_USUARIO e
// DB_SENHA são obrigatórios (sem fallback; ErrCredenciaisDB se ausentes ou
// vazios); DB_HOST, DB_PORT e DB_NAME mantêm os defaults locais.
//
// Mesmo DSN de config.DSN() (ver lá a regra de clientFoundRows=true:
// condições "só se ainda não ..." vão no WHERE, nunca deduzidas de
// RowsAffected=0). Os upserts abaixo (UPDATE e, se 0 linhas, INSERT)
// continuam corretos: com a flag, 0 significa "e-mail não existe".
func DSN() (string, error) {
	usuario, okU := os.LookupEnv("DB_USUARIO")
	senha, okS := os.LookupEnv("DB_SENHA")
	if !okU || !okS || usuario == "" || senha == "" {
		return "", ErrCredenciaisDB
	}
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local&clientFoundRows=true",
		usuario,
		senha,
		GetEnv("DB_HOST", "localhost"),
		GetEnv("DB_PORT", "3306"),
		GetEnv("DB_NAME", "rotaperfumes"),
	), nil
}

// ResolverSenha escolhe a senha a partir das flags de opts (no máximo uma
// fonte: -password, -password-stdin ou -password-prompt). Sem nenhuma fonte
// devolve "" e nil: o Run segue com env SEED_* ou senha aleatória.
//
// As dependências são injetadas (o comando passa os.Stdin, term.IsTerminal,
// term.ReadPassword e os.Stderr): stdin é lido só com -password-stdin;
// isTerminal/readNoEcho só com -password-prompt; stderr recebe o aviso de
// depreciação de -password e os rótulos do prompt.
func ResolverSenha(opts Options, stdin io.Reader, isTerminal func() bool, readNoEcho func() ([]byte, error), stderr io.Writer) (string, error) {
	fontes := 0
	for _, usada := range []bool{opts.Password != "", opts.PasswordStdin, opts.PasswordPrompt} {
		if usada {
			fontes++
		}
	}
	switch {
	case fontes > 1:
		return "", errors.New("resetpassword: uso: informe no máximo uma fonte de senha (-password, -password-stdin ou -password-prompt)")
	case opts.Password != "":
		fmt.Fprintln(stderr, AvisoPasswordDepreciada)
		return opts.Password, nil
	case opts.PasswordStdin:
		return senhaDoStdin(stdin)
	case opts.PasswordPrompt:
		return senhaDoPrompt(isTerminal, readNoEcho, stderr)
	}
	return "", nil
}

// senhaDoStdin lê a 1ª linha de stdin, sem o \r\n final; vazia é erro.
func senhaDoStdin(stdin io.Reader) (string, error) {
	linha, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("resetpassword: -password-stdin: leitura: %w", err)
	}
	senha := strings.TrimRight(linha, "\r\n")
	if senha == "" {
		return "", errors.New("resetpassword: -password-stdin: senha vazia (a 1ª linha do stdin está vazia)")
	}
	return senha, nil
}

// senhaDoPrompt lê a senha do terminal sem eco, pedindo confirmação.
func senhaDoPrompt(isTerminal func() bool, readNoEcho func() ([]byte, error), stderr io.Writer) (string, error) {
	if !isTerminal() {
		return "", errors.New("resetpassword: -password-prompt exige um terminal interativo; use -password-stdin " +
			"(ex.: printf '%s\\n' \"$SENHA\" | go run ./cmd/resetpassword ... -password-stdin) " +
			"ou, no Git Bash, rode via `winpty go run ./cmd/resetpassword ...`")
	}
	senha, err := lerSemEco(readNoEcho, stderr, "Nova senha: ")
	if err != nil {
		return "", err
	}
	// Registrado já aqui: zera a 1ª senha mesmo se a confirmação falhar.
	defer limpar(senha)
	confirmacao, err := lerSemEco(readNoEcho, stderr, "Confirme a senha: ")
	if err != nil {
		return "", err
	}
	defer limpar(confirmacao)
	if len(senha) == 0 {
		return "", errors.New("resetpassword: -password-prompt: senha vazia")
	}
	if !bytes.Equal(senha, confirmacao) {
		return "", errors.New("resetpassword: -password-prompt: as senhas não conferem")
	}
	return string(senha), nil
}

// lerSemEco escreve o rótulo em stderr e lê uma senha sem eco.
func lerSemEco(readNoEcho func() ([]byte, error), stderr io.Writer, rotulo string) ([]byte, error) {
	fmt.Fprint(stderr, rotulo)
	b, err := readNoEcho()
	fmt.Fprintln(stderr) // o terminal não ecoa o Enter
	if err != nil {
		return nil, fmt.Errorf("resetpassword: -password-prompt: leitura: %w", err)
	}
	return b, nil
}

// limpar zera o buffer da senha lida (reduz o tempo em memória).
func limpar(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// Run executa a ação escolhida em opts. A saída tabular e as senhas geradas
// vão para out (os.Stdout no comando). gen gera senhas quando nenhuma é
// informada (GenerateRandomPassword no comando). O texto do erro devolvido é
// a mensagem final completa (o comando só faz log.Fatal(err)).
func Run(ctx context.Context, db DB, opts Options, out io.Writer, gen PasswordGenerator) error {
	switch {
	case opts.List:
		if err := ListUsers(ctx, db, out); err != nil {
			return fmt.Errorf("resetpassword: list: %w", err)
		}

	case opts.CreateAdmin:
		password, err := senhaOuEnv(opts.Password, "SEED_ADMIN_PASSWORD", out, gen,
			"=== Senha gerada (guarde!) ===", "ADMIN_PASSWORD", "==============================")
		if err != nil {
			return err
		}
		if err := UpsertAdmin(ctx, db, password); err != nil {
			return fmt.Errorf("resetpassword: create-admin: %w", err)
		}
		listarFinal(ctx, db, out)

	case opts.AllUsers:
		password, err := senhaOuEnv(opts.Password, "SEED_USER_PASSWORD", out, gen,
			"=== Senha gerada (todos os usuários PLACEHOLDER usarão esta) ===", "USER_PASSWORD",
			"================================================================")
		if err != nil {
			return err
		}
		if err := UpdateAllPlaceholders(ctx, db, password); err != nil {
			return fmt.Errorf("resetpassword: update all: %w", err)
		}
		listarFinal(ctx, db, out)

	case opts.Email != "":
		password := opts.Password
		if password == "" {
			pwd, err := gerarEImprimir(out, gen, "=== Senha gerada ===", "PASSWORD", "====================")
			if err != nil {
				return err
			}
			password = pwd
		}
		if err := UpsertByEmail(ctx, db, opts.Email, password, opts.Role, opts.Nome, opts.IDVendedor); err != nil {
			return fmt.Errorf("resetpassword: upsert: %w", err)
		}
		listarFinal(ctx, db, out)

	default:
		return errors.New("informe uma ação: -list | -create-admin | -all-users | -email=...")
	}
	return nil
}

// senhaOuEnv devolve, por prioridade: a senha informada, a env envKey ou uma
// senha gerada (impressa em out com o cabeçalho/rodapé dados).
func senhaOuEnv(password, envKey string, out io.Writer, gen PasswordGenerator, cabecalho, chave, rodape string) (string, error) {
	if password != "" {
		return password, nil
	}
	if envPwd := os.Getenv(envKey); envPwd != "" {
		log.Printf("resetpassword: usando %s do .env", envKey)
		return envPwd, nil
	}
	return gerarEImprimir(out, gen, cabecalho, chave, rodape)
}

// gerarEImprimir gera uma senha de 16 caracteres e a imprime em out.
func gerarEImprimir(out io.Writer, gen PasswordGenerator, cabecalho, chave, rodape string) (string, error) {
	pwd, err := gen(16)
	if err != nil {
		return "", fmt.Errorf("gerar senha: %w", err)
	}
	fmt.Fprintln(out, cabecalho)
	fmt.Fprintf(out, "%s=%s\n", chave, pwd)
	fmt.Fprintln(out, rodape)
	return pwd, nil
}

// listarFinal imprime a lista de usuários após a ação; falha só gera log.
func listarFinal(ctx context.Context, db DB, out io.Writer) {
	fmt.Fprintln(out)
	if err := ListUsers(ctx, db, out); err != nil {
		log.Printf("listar final: %v", err)
	}
}

// UpsertAdmin garante que admin@rotaperfumes.com.br existe e tem hash válido.
func UpsertAdmin(ctx context.Context, db DB, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}
	hashStr := string(hash)

	// Tenta UPDATE primeiro; se afetou 0 linhas, faz INSERT. SEC-08: grava
	// também o corte de sessão (tokens_validos_desde), derrubando os access
	// tokens emitidos com a senha anterior.
	res, err := db.ExecContext(ctx,
		`UPDATE usuarios SET password_hash = ?, ativo = 1, tokens_validos_desde = ? WHERE email = ?`,
		hashStr, corteDeSessaoAgora(), AdminEmail,
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
		"Administrador Principal", AdminEmail, hashStr,
	)
	if err != nil {
		return fmt.Errorf("insert admin: %w", err)
	}
	log.Printf("resetpassword: admin criado (INSERT)")
	return nil
}

// UpsertByEmail cria ou atualiza um usuário.
func UpsertByEmail(ctx context.Context, db DB, email, password, role, nome string, idVendedor int64) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}
	hashStr := string(hash)

	// UPDATE primeiro (SEC-08: com corte de sessão, ver corteDeSessaoAgora).
	res, err := db.ExecContext(ctx,
		`UPDATE usuarios SET password_hash = ?, tokens_validos_desde = ? WHERE email = ?`,
		hashStr, corteDeSessaoAgora(), email,
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

// UpdateAllPlaceholders substitui todos os hashes com placeholder.
func UpdateAllPlaceholders(ctx context.Context, db DB, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
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
	if err := EnsureAdmin(ctx, db); err != nil {
		log.Printf("resetpassword: ensure-admin: %v", err)
	}
	return nil
}

// EnsureAdmin garante que o admin existe (chamado pelo -all-users como salvaguarda).
func EnsureAdmin(ctx context.Context, db DB) error {
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

// ListUsers imprime em out a lista de usuários com o status do hash.
func ListUsers(ctx context.Context, db DB, out io.Writer) error {
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

	fmt.Fprintf(out, "%-4s %-50s %-8s %-6s %-28s %s\n", "ID", "EMAIL", "ROLE", "ATIVO", "HASH_PREVIEW", "STATUS")
	fmt.Fprintln(out, strings.Repeat("-", 120))
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
		fmt.Fprintf(out, "%-4d %-50s %-8s %-6d %-28s %s\n", id, Truncate(email, 50), role, ativo, preview, status)
		count++
		switch status {
		case "PLACEHOLDER":
			placeholderCount++
		case "OK":
			okCount++
		}
		if email == AdminEmail {
			adminFound = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Fprintln(out, strings.Repeat("-", 120))
	fmt.Fprintf(out, "Total: %d usuarios | %d OK | %d PLACEHOLDER | admin existe: %v\n", count, okCount, placeholderCount, adminFound)
	if !adminFound {
		fmt.Fprintln(out, "⚠️  ATENÇÃO: admin@rotaperfumes.com.br NÃO EXISTE — rode: make fix-hash")
	}
	return nil
}

// Truncate corta s em n caracteres (bytes), terminando com "..." quando corta.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// GenerateRandomPassword devolve uma senha aleatória de n caracteres
// [a-zA-Z0-9], uniforme (delega a services.SenhaAlfanumerica).
func GenerateRandomPassword(n int) (string, error) {
	return services.SenhaAlfanumerica(n)
}

// GetEnv devolve a env key ou fallback se estiver vazia.
func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// corteDeSessaoAgora é o valor gravado em tokens_validos_desde (SEC-08) ao
// trocar a senha de um usuário existente: access tokens com iat até esse
// instante deixam de valer. Gerado pelo Go (nunca NOW() do MySQL) e
// truncado em segundos, mesma precisão do iat do JWT.
func corteDeSessaoAgora() time.Time {
	return time.Now().Truncate(time.Second)
}

// Package config carrega configurações do sistema a partir de variáveis de ambiente.
//
// O arquivo .env na raiz do projeto é carregado pelos binários (cmdutil /
// godotenv) ou pelo Makefile ("-include .env") antes de chamar o Load — aqui
// apenas consumimos via os.Getenv. DB_USUARIO/DB_SENHA são obrigatórios e
// não têm default (SEC-11): sem eles, Load devolve ErrCredenciaisDB.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	// Fixa time.Local em -03:00 (sem horário de verão) antes de qualquer
	// sql.Open — ver pacote tz (RISCO-01). Todo binário que usa config.DSN()
	// herda o fuso por este import.
	_ "github.com/rotaperfumes/shared/tz"
	"github.com/rotaperfumes/shared/vlog"
)

// Config agrega todas as configurações da aplicação.
type Config struct {
	// Banco de dados
	DBHost    string
	DBPort    string
	DBName    string
	DBUsuario string
	DBSenha   string

	// JWT
	JWTSecret string
	JWTTTL    time.Duration
	JWTIssuer string

	// HashSenha: Argon2id + pepper das senhas (SEC-13).
	HashSenha HashSenha

	// Logging / Debug
	Verbose bool

	// SMTP (envio de emails transacionais, ex: senha inicial/reset)
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	// SMTPTLSMode (SMTP_TLS_MODE): "implicit" (SMTPS, TLS desde o connect) ou
	// "starttls". Vazio = inferir pela porta (465 → implicit; demais → starttls).
	SMTPTLSMode string
	// SMTPTLSServerName (SMTP_TLS_SERVER_NAME): nome esperado no certificado
	// do servidor. Vazio = SMTP_HOST. Útil quando SMTP_HOST é um IP.
	SMTPTLSServerName string
	// SMTPTLSCAFile (SMTP_TLS_CA_FILE): PEM com CA/certificado extra a
	// confiar (além das raízes do sistema), ex.: servidor com cert próprio.
	SMTPTLSCAFile string

	// CORSAllowedOrigins é a lista explícita de origens permitidas (comparação
	// exata, sem prefix-match), configurável via CORS_ALLOWED_ORIGINS
	// (separadas por vírgula). Sempre inclui as origens padrão de dev local.
	CORSAllowedOrigins []string

	// TrustProxyHeaders indica se a aplicação está atrás de um proxy/load
	// balancer confiável que popula X-Forwarded-For/X-Real-IP corretamente.
	// Quando false (padrão), esses headers são ignorados e r.RemoteAddr é
	// sempre usado como IP do cliente — evita spoofing de IP.
	TrustProxyHeaders bool

	// CookieSecure liga o atributo Secure dos cookies de sessão (SEC-21), via
	// COOKIE_SECURE. Padrão true; só "false" explícito (dev em HTTP, o make
	// dev-api/run-api faz isso) desliga. Não depende do Host da requisição.
	CookieSecure bool

	// TurnstileSecretKey é a chave secreta do Cloudflare Turnstile usada para
	// verificar o token de CAPTCHA enviado pelo frontend em /api/auth/login e
	// /api/auth/reset-password (env TURNSTILE_SECRET_KEY). Nunca é logada nem
	// retornada em resposta JSON.
	TurnstileSecretKey string

	// TurnstileHostnames são os hostnames aceitos na resposta do siteverify
	// (SEC-19), via TURNSTILE_HOSTNAMES (separados por vírgula, só hostname,
	// em minúsculas). Ausente ou vazia: derivados de CORSAllowedOrigins.
	TurnstileHostnames []string

	// SecurityAlertEmails são os destinatários administrativos dos alertas de
	// segurança (SEC-09, ex.: reuso de refresh token), via
	// SECURITY_ALERT_EMAILS (separados por vírgula). Entradas vazias, sem "@"
	// ou com caracteres de quebra de linha são descartadas.
	SecurityAlertEmails []string

	// RefreshReuseSuppressWindow é a janela em que um novo reuso do MESMO
	// refresh token já rotacionado não derruba as sessões de novo (SEC-12),
	// via REFRESH_REUSE_SUPPRESS_WINDOW. Padrão 30m, faixa [1m, 24h].
	RefreshReuseSuppressWindow time.Duration

	// RefreshCleanupInterval é o intervalo da limpeza periódica de
	// refresh_tokens (CHORE-02), via REFRESH_CLEANUP_INTERVAL. Padrão 6h,
	// mínimo 1m; "0" desativa a limpeza.
	RefreshCleanupInterval time.Duration

	// RefreshTokenRetencao é quanto tempo um refresh token fica no banco
	// depois de expires_at antes de ser apagado (CHORE-02), via
	// REFRESH_TOKEN_RETENCAO. Padrão 720h (30 dias), faixa [24h, 8760h].
	RefreshTokenRetencao time.Duration

	// ProdutoImagensDir é o diretório das imagens de produto (IMG-01), via
	// PRODUTO_IMAGENS_DIR. Padrão ./data/produtos-imagens (relativo ao
	// diretório de execução); sempre absoluto e limpo depois do Load. Vazio,
	// "/" ou dentro de frontend/public derrubam a subida.
	ProdutoImagensDir string
}

// HashSenha são os parâmetros do hash de senha (SEC-13): Argon2id sobre o
// HMAC-SHA256(Pepper, senha). Pepper vem de PASSWORD_PEPPER e nunca é logado;
// trocá-lo invalida todas as senhas Argon2id já gravadas.
type HashSenha struct {
	Pepper      string
	MemoriaKiB  uint32
	Iteracoes   uint32
	Paralelismo uint8
}

// PepperMinBytes é o tamanho mínimo aceito para PASSWORD_PEPPER.
const PepperMinBytes = 32

// ErrPepperAusente: PASSWORD_PEPPER ausente ou curto demais (SEC-13).
var ErrPepperAusente = fmt.Errorf("config: PASSWORD_PEPPER é obrigatório (mínimo de %d bytes)", PepperMinBytes)

// JWTSecretMinBytes é o tamanho mínimo aceito para JWT_SECRET (SEC-18).
const JWTSecretMinBytes = 32

// ErrJWTSecretCurto: JWT_SECRET ausente, curto demais ou igual ao
// placeholder do .env.example (SEC-18). Nunca inclui o valor.
var ErrJWTSecretCurto = fmt.Errorf("config: JWT_SECRET é obrigatório (mínimo de %d bytes)", JWTSecretMinBytes)

// jwtSecretPlaceholder é o valor de exemplo histórico do .env.example: tem
// mais de 32 bytes, mas é público, então é recusado.
const jwtSecretPlaceholder = "troque-por-um-segredo-forte-e-aleatorio"

// Faixas e padrões do Argon2id (SEC-13). O mínimo de memória segue a
// recomendação da OWASP (19 MiB).
const (
	argon2MemoriaPadrao     = 64 * 1024
	argon2MemoriaMin        = 19 * 1024
	argon2MemoriaMax        = 1024 * 1024
	argon2IteracoesPadrao   = 3
	argon2IteracoesMax      = 10
	argon2ParalelismoPadrao = 2
	argon2ParalelismoMax    = 16
)

// ErrCredenciaisDB: DB_USUARIO/DB_SENHA ausentes ou vazios (SEC-11).
var ErrCredenciaisDB = errors.New("config: defina DB_USUARIO/DB_SENHA no .env")

// Faixas e padrões das durações de refresh token (SEC-12 / CHORE-02).
const (
	refreshReuseWindowPadrao = 30 * time.Minute
	refreshReuseWindowMin    = time.Minute
	refreshReuseWindowMax    = 24 * time.Hour

	refreshCleanupIntervalPadrao = 6 * time.Hour
	refreshCleanupIntervalMin    = time.Minute

	refreshRetencaoPadrao = 720 * time.Hour
	refreshRetencaoMin    = 24 * time.Hour
	refreshRetencaoMax    = 8760 * time.Hour
)

// Load lê as variáveis de ambiente e retorna uma Config preenchida.
// Retorna erro se variáveis obrigatórias estiverem ausentes — inclusive
// DB_USUARIO/DB_SENHA (ErrCredenciaisDB, SEC-11).
func Load() (*Config, error) {
	return load(true)
}

// LoadSemCredenciaisDB é o Load sem a exigência de DB_USUARIO/DB_SENHA, para
// ferramentas que não abrem conexão (ex.: seedusers -dry-run/-no-exec, SEC-10).
// As demais validações continuam valendo.
func LoadSemCredenciaisDB() (*Config, error) {
	return load(false)
}

func load(exigirDB bool) (*Config, error) {
	vlog.Printf("config.go", "load", "montando Config a partir das variáveis de ambiente (exigirDB=%t)", exigirDB)
	cfg := &Config{
		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    getEnv("DB_PORT", "3306"),
		DBName:    getEnv("DB_NAME", "rotaperfumes"),
		DBUsuario: os.Getenv("DB_USUARIO"),
		DBSenha:   os.Getenv("DB_SENHA"),
		JWTSecret: os.Getenv("JWT_SECRET"),
		JWTIssuer: jwtIssuer(),
	}

	vlog.Printf("config.go", "load", "verificando se JWT_SECRET tem o tamanho mínimo e não é o placeholder")
	if len(cfg.JWTSecret) < JWTSecretMinBytes || cfg.JWTSecret == jwtSecretPlaceholder {
		return nil, ErrJWTSecretCurto
	}

	vlog.Printf("config.go", "load", "lendo JWT_TTL (padrão 24h)")
	ttlStr := getEnv("JWT_TTL", "24h")
	vlog.Printf("config.go", "load", "convertendo JWT_TTL para time.Duration")
	ttl, err := time.ParseDuration(ttlStr)
	vlog.Printf("config.go", "load", "verificando se err != nil após ParseDuration de JWT_TTL")
	if err != nil {
		return nil, fmt.Errorf("config: JWT_TTL inválido (%q): %w", ttlStr, err)
	}
	vlog.Printf("config.go", "load", "verificando se JWT_TTL é positivo")
	if ttl <= 0 {
		return nil, fmt.Errorf("config: JWT_TTL inválido (%q): esperado maior que zero", ttlStr)
	}
	vlog.Printf("config.go", "load", "atribuindo JWTTTL=%s", ttl)
	cfg.JWTTTL = ttl

	vlog.Printf("config.go", "load", "carregando parâmetros de hash de senha (LoadHashSenha)")
	hs, err := LoadHashSenha()
	vlog.Printf("config.go", "load", "verificando se err != nil após LoadHashSenha")
	if err != nil {
		return nil, err
	}
	vlog.Printf("config.go", "load", "atribuindo HashSenha à Config")
	cfg.HashSenha = hs

	vlog.Printf("config.go", "load", "calculando Verbose a partir de VERBOSE/LOG_LEVEL")
	cfg.Verbose = getEnv("VERBOSE", "false") == "true" ||
		getEnv("LOG_LEVEL", "info") == "debug"

	// SMTP: sem defaults para user/password/from — quando ausentes, a
	// aplicação usa o envio Noop (notificacaosmtp, log-only). Host/porta têm
	// defaults compatíveis com Gmail.
	vlog.Printf("config.go", "load", "lendo SMTP_HOST (padrão smtp.gmail.com)")
	cfg.SMTPHost = getEnv("SMTP_HOST", "smtp.gmail.com")
	vlog.Printf("config.go", "load", "lendo SMTP_PORT (padrão 587)")
	cfg.SMTPPort = getEnv("SMTP_PORT", "587")
	vlog.Printf("config.go", "load", "lendo SMTP_USER")
	cfg.SMTPUser = os.Getenv("SMTP_USER")
	vlog.Printf("config.go", "load", "lendo SMTP_PASSWORD")
	cfg.SMTPPassword = os.Getenv("SMTP_PASSWORD")
	vlog.Printf("config.go", "load", "lendo SMTP_FROM")
	cfg.SMTPFrom = os.Getenv("SMTP_FROM")
	vlog.Printf("config.go", "load", "lendo SMTP_TLS_MODE (vazio = inferir pela porta)")
	cfg.SMTPTLSMode = os.Getenv("SMTP_TLS_MODE")
	vlog.Printf("config.go", "load", "lendo SMTP_TLS_SERVER_NAME (vazio = SMTP_HOST)")
	cfg.SMTPTLSServerName = os.Getenv("SMTP_TLS_SERVER_NAME")
	vlog.Printf("config.go", "load", "lendo SMTP_TLS_CA_FILE (vazio = só raízes do sistema)")
	cfg.SMTPTLSCAFile = os.Getenv("SMTP_TLS_CA_FILE")

	vlog.Printf("config.go", "load", "montando lista de origens CORS permitidas e verificando erro")
	if cfg.CORSAllowedOrigins, err = parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS")); err != nil {
		return nil, err
	}
	vlog.Printf("config.go", "load", "lendo TRUST_PROXY_HEADERS")
	cfg.TrustProxyHeaders = getEnv("TRUST_PROXY_HEADERS", "false") == "true"
	vlog.Printf("config.go", "load", "lendo COOKIE_SECURE (padrão true) e verificando erro")
	if cfg.CookieSecure, err = parseCookieSecure(); err != nil {
		return nil, err
	}
	vlog.Printf("config.go", "load", "lendo TURNSTILE_SECRET_KEY")
	cfg.TurnstileSecretKey = os.Getenv("TURNSTILE_SECRET_KEY")
	vlog.Printf("config.go", "load", "lendo TURNSTILE_HOSTNAMES e verificando erro")
	if cfg.TurnstileHostnames, err = parseTurnstileHostnames(os.Getenv("TURNSTILE_HOSTNAMES"), cfg.CORSAllowedOrigins); err != nil {
		return nil, err
	}
	vlog.Printf("config.go", "load", "lendo destinatários de alertas de segurança")
	cfg.SecurityAlertEmails = ParseSecurityAlertEmails(os.Getenv("SECURITY_ALERT_EMAILS"))

	vlog.Printf("config.go", "load", "carregando durações de refresh token e verificando erro")
	if err := carregarDuracoesRefresh(cfg); err != nil {
		return nil, err
	}

	vlog.Printf("config.go", "load", "lendo PRODUTO_IMAGENS_DIR e verificando erro")
	if cfg.ProdutoImagensDir, err = parseProdutoImagensDir(); err != nil {
		return nil, err
	}

	// SEC-11: checagem no fim, depois das demais validações, para que os
	// erros já existentes (JWT_SECRET, JWT_TTL, PASSWORD_PEPPER/ARGON2_*,
	// COOKIE_SECURE) tenham prioridade.
	// A mensagem nunca inclui os valores.
	vlog.Printf("config.go", "load", "verificando presença das credenciais do banco (exigirDB=%t)", exigirDB)
	if exigirDB && (strings.TrimSpace(cfg.DBUsuario) == "" || cfg.DBSenha == "") {
		return nil, ErrCredenciaisDB
	}

	return cfg, nil
}

// jwtIssuer lê JWT_ISSUER com trim; vazio (ou só espaços) vira o padrão
// "rotaperfumes" (SEC-18: issuer vazio desligaria a checagem no parser).
func jwtIssuer() string {
	vlog.Printf("config.go", "jwtIssuer", "lendo JWT_ISSUER com trim e verificando se está vazio")
	if v := strings.TrimSpace(os.Getenv("JWT_ISSUER")); v != "" {
		return v
	}
	return "rotaperfumes"
}

// produtoImagensDirPadrao é o diretório padrão das imagens de produto.
const produtoImagensDirPadrao = "./data/produtos-imagens"

// ErrProdutoImagensDir: PRODUTO_IMAGENS_DIR vazio, raiz ou dentro de
// frontend/public (IMG-01).
var ErrProdutoImagensDir = errors.New("config: PRODUTO_IMAGENS_DIR inválido (não pode ser vazio, \"/\" nem ficar dentro de frontend/public)")

// parseProdutoImagensDir lê PRODUTO_IMAGENS_DIR (ausente = padrão), aplica
// Clean+Abs e recusa a raiz e qualquer caminho dentro de frontend/public.
// Só espaços conta como vazio.
func parseProdutoImagensDir() (string, error) {
	vlog.Printf("config.go", "parseProdutoImagensDir", "lendo PRODUTO_IMAGENS_DIR (padrão %s)", produtoImagensDirPadrao)
	raw, definido := os.LookupEnv("PRODUTO_IMAGENS_DIR")
	if !definido {
		raw = produtoImagensDirPadrao
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrProdutoImagensDir
	}
	abs, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrProdutoImagensDir, err)
	}
	vlog.Printf("config.go", "parseProdutoImagensDir", "verificando se o diretório é a raiz ou fica dentro de frontend/public")
	sep := string(filepath.Separator)
	publico := sep + filepath.Join("frontend", "public")
	if abs == filepath.VolumeName(abs)+sep || strings.Contains(abs+sep, publico+sep) {
		return "", ErrProdutoImagensDir
	}
	return abs, nil
}

// parseCookieSecure lê COOKIE_SECURE: vazio → true; senão strconv.ParseBool.
// Valor inválido derruba a subida (SEC-21).
func parseCookieSecure() (bool, error) {
	vlog.Printf("config.go", "parseCookieSecure", "lendo COOKIE_SECURE com trim e verificando se está vazio")
	raw := strings.TrimSpace(os.Getenv("COOKIE_SECURE"))
	if raw == "" {
		return true, nil
	}
	vlog.Printf("config.go", "parseCookieSecure", "convertendo COOKIE_SECURE para bool")
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("config: COOKIE_SECURE inválido (%q): esperado true ou false", raw)
	}
	return v, nil
}

// LoadHashSenha lê PASSWORD_PEPPER e ARGON2_MEMORIA_KIB / ARGON2_ITERACOES /
// ARGON2_PARALELISMO. Exposto à parte para ferramentas que não carregam a
// Config inteira (ex.: cmd/resetpassword). As mensagens nunca incluem o pepper.
func LoadHashSenha() (HashSenha, error) {
	vlog.Printf("config.go", "LoadHashSenha", "lendo PASSWORD_PEPPER do ambiente")
	hs := HashSenha{Pepper: os.Getenv("PASSWORD_PEPPER")}
	vlog.Printf("config.go", "LoadHashSenha", "verificando se o pepper tem o tamanho mínimo de %d bytes", PepperMinBytes)
	if len(hs.Pepper) < PepperMinBytes {
		return HashSenha{}, ErrPepperAusente
	}
	vlog.Printf("config.go", "LoadHashSenha", "lendo ARGON2_MEMORIA_KIB na faixa permitida")
	m, err := parseUintNaFaixa("ARGON2_MEMORIA_KIB", argon2MemoriaPadrao, argon2MemoriaMin, argon2MemoriaMax)
	vlog.Printf("config.go", "LoadHashSenha", "verificando se err != nil após ARGON2_MEMORIA_KIB")
	if err != nil {
		return HashSenha{}, err
	}
	vlog.Printf("config.go", "LoadHashSenha", "lendo ARGON2_ITERACOES na faixa permitida")
	t, err := parseUintNaFaixa("ARGON2_ITERACOES", argon2IteracoesPadrao, 1, argon2IteracoesMax)
	vlog.Printf("config.go", "LoadHashSenha", "verificando se err != nil após ARGON2_ITERACOES")
	if err != nil {
		return HashSenha{}, err
	}
	vlog.Printf("config.go", "LoadHashSenha", "lendo ARGON2_PARALELISMO na faixa permitida")
	p, err := parseUintNaFaixa("ARGON2_PARALELISMO", argon2ParalelismoPadrao, 1, argon2ParalelismoMax)
	vlog.Printf("config.go", "LoadHashSenha", "verificando se err != nil após ARGON2_PARALELISMO")
	if err != nil {
		return HashSenha{}, err
	}
	vlog.Printf("config.go", "LoadHashSenha", "atribuindo parâmetros Argon2id (memoria=%d KiB, iteracoes=%d, paralelismo=%d)", m, t, p)
	hs.MemoriaKiB, hs.Iteracoes, hs.Paralelismo = uint32(m), uint32(t), uint8(p)
	return hs, nil
}

// parseUintNaFaixa lê a env chave como inteiro (padrão se vazia) e exige
// minimo <= valor <= maximo.
func parseUintNaFaixa(chave string, padrao, minimo, maximo uint64) (uint64, error) {
	vlog.Printf("config.go", "parseUintNaFaixa", "lendo env %s com trim", chave)
	raw := strings.TrimSpace(os.Getenv(chave))
	vlog.Printf("config.go", "parseUintNaFaixa", "verificando se %s está vazia (usa padrão)", chave)
	if raw == "" {
		return padrao, nil
	}
	vlog.Printf("config.go", "parseUintNaFaixa", "convertendo %s para inteiro sem sinal", chave)
	v, err := strconv.ParseUint(raw, 10, 32)
	vlog.Printf("config.go", "parseUintNaFaixa", "verificando erro de conversão ou valor fora da faixa [%d, %d]", minimo, maximo)
	if err != nil || v < minimo || v > maximo {
		return 0, fmt.Errorf("config: %s inválido (%q): esperado inteiro entre %d e %d", chave, raw, minimo, maximo)
	}
	return v, nil
}

// carregarDuracoesRefresh lê as durações de refresh token (SEC-12 e CHORE-02)
// e valida as faixas; valor inválido vira erro do Load (estilo JWT_TTL).
func carregarDuracoesRefresh(cfg *Config) error {
	vlog.Printf("config.go", "carregarDuracoesRefresh", "lendo REFRESH_REUSE_SUPPRESS_WINDOW na faixa permitida")
	janela, err := parseDuracaoNaFaixa("REFRESH_REUSE_SUPPRESS_WINDOW", refreshReuseWindowPadrao, refreshReuseWindowMin, refreshReuseWindowMax)
	vlog.Printf("config.go", "carregarDuracoesRefresh", "verificando se err != nil após REFRESH_REUSE_SUPPRESS_WINDOW")
	if err != nil {
		return err
	}
	vlog.Printf("config.go", "carregarDuracoesRefresh", "atribuindo RefreshReuseSuppressWindow=%s", janela)
	cfg.RefreshReuseSuppressWindow = janela

	vlog.Printf("config.go", "carregarDuracoesRefresh", "lendo intervalo de limpeza de refresh tokens")
	intervalo, err := parseIntervaloLimpeza()
	vlog.Printf("config.go", "carregarDuracoesRefresh", "verificando se err != nil após parseIntervaloLimpeza")
	if err != nil {
		return err
	}
	vlog.Printf("config.go", "carregarDuracoesRefresh", "atribuindo RefreshCleanupInterval=%s", intervalo)
	cfg.RefreshCleanupInterval = intervalo

	vlog.Printf("config.go", "carregarDuracoesRefresh", "lendo REFRESH_TOKEN_RETENCAO na faixa permitida")
	retencao, err := parseDuracaoNaFaixa("REFRESH_TOKEN_RETENCAO", refreshRetencaoPadrao, refreshRetencaoMin, refreshRetencaoMax)
	vlog.Printf("config.go", "carregarDuracoesRefresh", "verificando se err != nil após REFRESH_TOKEN_RETENCAO")
	if err != nil {
		return err
	}
	vlog.Printf("config.go", "carregarDuracoesRefresh", "atribuindo RefreshTokenRetencao=%s", retencao)
	cfg.RefreshTokenRetencao = retencao
	return nil
}

// parseIntervaloLimpeza lê REFRESH_CLEANUP_INTERVAL: "0" (ou "0s") desativa a
// limpeza; qualquer outro valor precisa ser >= 1m.
func parseIntervaloLimpeza() (time.Duration, error) {
	const chave = "REFRESH_CLEANUP_INTERVAL"
	vlog.Printf("config.go", "parseIntervaloLimpeza", "lendo env %s com trim", chave)
	raw := strings.TrimSpace(os.Getenv(chave))
	vlog.Printf("config.go", "parseIntervaloLimpeza", "verificando se %s está vazia (usa padrão)", chave)
	if raw == "" {
		return refreshCleanupIntervalPadrao, nil
	}
	vlog.Printf("config.go", "parseIntervaloLimpeza", "verificando se %s é zero literal (limpeza desativada)", chave)
	if raw == "0" {
		return 0, nil
	}
	vlog.Printf("config.go", "parseIntervaloLimpeza", "convertendo %s para time.Duration", chave)
	d, err := time.ParseDuration(raw)
	vlog.Printf("config.go", "parseIntervaloLimpeza", "verificando se err != nil após ParseDuration")
	if err != nil {
		return 0, fmt.Errorf("config: %s inválido (%q): %w", chave, raw, err)
	}
	vlog.Printf("config.go", "parseIntervaloLimpeza", "verificando se a duração é zero (limpeza desativada)")
	if d == 0 {
		return 0, nil
	}
	vlog.Printf("config.go", "parseIntervaloLimpeza", "verificando se a duração é menor que o mínimo %s", refreshCleanupIntervalMin)
	if d < refreshCleanupIntervalMin {
		return 0, fmt.Errorf("config: %s inválido (%q): esperado 0 (desativa) ou >= %s", chave, raw, refreshCleanupIntervalMin)
	}
	return d, nil
}

// parseDuracaoNaFaixa lê a env chave como time.Duration (padrão se vazia) e
// exige minimo <= valor <= maximo.
func parseDuracaoNaFaixa(chave string, padrao, minimo, maximo time.Duration) (time.Duration, error) {
	vlog.Printf("config.go", "parseDuracaoNaFaixa", "lendo env %s com trim", chave)
	raw := strings.TrimSpace(os.Getenv(chave))
	vlog.Printf("config.go", "parseDuracaoNaFaixa", "verificando se %s está vazia (usa padrão %s)", chave, padrao)
	if raw == "" {
		return padrao, nil
	}
	vlog.Printf("config.go", "parseDuracaoNaFaixa", "convertendo %s para time.Duration", chave)
	d, err := time.ParseDuration(raw)
	vlog.Printf("config.go", "parseDuracaoNaFaixa", "verificando se err != nil após ParseDuration")
	if err != nil {
		return 0, fmt.Errorf("config: %s inválido (%q): %w", chave, raw, err)
	}
	vlog.Printf("config.go", "parseDuracaoNaFaixa", "verificando se a duração está fora da faixa [%s, %s]", minimo, maximo)
	if d < minimo || d > maximo {
		return 0, fmt.Errorf("config: %s inválido (%q): esperado entre %s e %s", chave, raw, minimo, maximo)
	}
	return d, nil
}

// ParseSecurityAlertEmails interpreta SECURITY_ALERT_EMAILS: separa por
// vírgula, aplica trim e descarta entradas vazias, sem "@" ou contendo CR/LF
// (evita injeção de cabeçalho no envio SMTP).
func ParseSecurityAlertEmails(raw string) []string {
	vlog.Printf("config.go", "ParseSecurityAlertEmails", "declarando lista de e-mails de alerta")
	var emails []string
	vlog.Printf("config.go", "ParseSecurityAlertEmails", "iterando entradas separadas por vírgula e filtrando inválidas")
	for _, e := range strings.Split(raw, ",") {
		e = strings.TrimSpace(e)
		if e == "" || !strings.Contains(e, "@") || strings.ContainsAny(e, "\r\n") {
			continue
		}
		emails = append(emails, e)
	}
	vlog.Printf("config.go", "ParseSecurityAlertEmails", "e-mails de alerta válidos: %d", len(emails))
	return emails
}

// defaultDevOrigins são as origens de desenvolvimento local sempre permitidas,
// independente de CORS_ALLOWED_ORIGINS — cobre o frontend Next.js em dev.
var defaultDevOrigins = []string{
	"http://localhost:3000",
	"http://127.0.0.1:3000",
}

// ErrCORSOrigemInvalida: CORS_ALLOWED_ORIGINS com "*" ou "null" (SEC-17).
// O CORS da API responde com Allow-Credentials; um curinga liberaria os
// cookies de sessão para qualquer site.
var ErrCORSOrigemInvalida = errors.New("config: CORS_ALLOWED_ORIGINS não aceita \"*\" nem \"null\" (os cookies de sessão exigem origens explícitas)")

// parseAllowedOrigins monta a lista final de origens permitidas: as origens de
// dev local padrão mais o que vier em CORS_ALLOWED_ORIGINS (separadas por
// vírgula). Comparação é sempre exata — sem prefix-match. Uma entrada "*" ou
// "null" (sem diferenciar maiúsculas) é ErrCORSOrigemInvalida (SEC-17).
func parseAllowedOrigins(raw string) ([]string, error) {
	vlog.Printf("config.go", "parseAllowedOrigins", "alocando slice de origens com capacidade inicial")
	origins := make([]string, 0, len(defaultDevOrigins)+2)
	vlog.Printf("config.go", "parseAllowedOrigins", "adicionando %d origens padrão de dev", len(defaultDevOrigins))
	origins = append(origins, defaultDevOrigins...)

	vlog.Printf("config.go", "parseAllowedOrigins", "iterando origens de CORS_ALLOWED_ORIGINS")
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if origemCuringa(o) {
			vlog.Printf("config.go", "parseAllowedOrigins", "decisão origem_curinga_recusada")
			return nil, ErrCORSOrigemInvalida
		}
		origins = append(origins, o)
	}
	vlog.Printf("config.go", "parseAllowedOrigins", "total de origens permitidas: %d", len(origins))
	return origins, nil
}

// origemCuringa: "*" e "null" nunca entram na allowlist com credentials.
func origemCuringa(o string) bool {
	return o == "*" || strings.EqualFold(o, "null")
}

// ErrTurnstileHostnames: TURNSTILE_HOSTNAMES com item vazio, esquema, porta,
// caminho, curinga ou espaço interno (SEC-19). Nunca inclui o valor.
var ErrTurnstileHostnames = errors.New("config: TURNSTILE_HOSTNAMES inválida (lista de hostnames separados por vírgula, sem esquema, porta, caminho nem curinga)")

// parseTurnstileHostnames monta os hostnames aceitos pelo Turnstile (SEC-19).
// raw vazio (ou só espaços) deriva a lista das origens CORS; senão cada item
// passa por trim e minúsculas e um item inválido é ErrTurnstileHostnames.
// A lista final não tem repetição.
func parseTurnstileHostnames(raw string, origens []string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		hosts := hostnamesDasOrigens(origens)
		vlog.Printf("config.go", "parseTurnstileHostnames", "decisão hostnames_derivados_do_cors total=%d", len(hosts))
		return hosts, nil
	}
	var hosts []string
	for _, h := range strings.Split(raw, ",") {
		h = strings.ToLower(strings.TrimSpace(h))
		if !hostnameValido(h) {
			vlog.Printf("config.go", "parseTurnstileHostnames", "decisão hostname_invalido_recusado")
			return nil, ErrTurnstileHostnames
		}
		hosts = adicionarSemRepetir(hosts, h)
	}
	vlog.Printf("config.go", "parseTurnstileHostnames", "hostnames de TURNSTILE_HOSTNAMES total=%d", len(hosts))
	return hosts, nil
}

// hostnameValido recusa vazio, esquema, porta, caminho, curinga e espaço.
func hostnameValido(h string) bool {
	return h != "" && !strings.ContainsAny(h, ":/* \t\r\n")
}

// hostnamesDasOrigens extrai o hostname de cada origem (minúsculas, sem
// repetição); origem sem hostname é ignorada.
func hostnamesDasOrigens(origens []string) []string {
	var hosts []string
	for _, o := range origens {
		u, err := url.Parse(o)
		if err != nil {
			continue
		}
		if h := strings.ToLower(u.Hostname()); h != "" {
			hosts = adicionarSemRepetir(hosts, h)
		}
	}
	return hosts
}

func adicionarSemRepetir(lista []string, v string) []string {
	if slices.Contains(lista, v) {
		return lista
	}
	return append(lista, v)
}

// DSN monta a connection string para o driver go-sql-driver/mysql, que
// conecta ao MariaDB (protocolo MySQL).
// Inclui parseTime=true para que colunas TIMESTAMP sejam tipadas como time.Time.
//
// loc=Local: time.Local é fixado em -03:00 pelo pacote tz (import em branco
// acima). Não adicionar time_zone ao DSN.
//
// clientFoundRows=true (BUG-04): RowsAffected passa a contar as linhas
// ENCONTRADAS pelo WHERE, não só as alteradas. Assim, um UPDATE que não muda
// nenhum valor devolve 1 (sucesso) em vez de 0 (que os repositórios traduzem
// para ErrNotFound/404).
//
// REGRA DO PROJETO: operações do tipo "só se ainda não usado/revogado/
// desligado" DEVEM colocar a condição no WHERE (ex.: "... WHERE id = ? AND
// revogado_em IS NULL") ou usar COALESCE no SET. Nunca deduzir "não mudou"
// a partir de RowsAffected=0 — com clientFoundRows=true isso não acontece
// mais quando a linha existe. Manter este DSN idêntico ao de
// cmd/resetpassword.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local&clientFoundRows=true",
		c.DBUsuario, c.DBSenha, c.DBHost, c.DBPort, c.DBName,
	)
}

func getEnv(key, fallback string) string {
	vlog.Printf("config.go", "getEnv", "lendo env %s e verificando se está preenchida", key)
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

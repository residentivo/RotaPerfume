package services_test

// ARQ-03: AuthService, password_hash, password_policy, password_generator e
// turnstile_service saíram de shared/services. Os testes (mesmos nomes)
// exercitam os substitutos: identidadecrypto (Hasher, JWT, geradores),
// identidademysql (Usuarios) e adapters/turnstile. authCrypto só repassa
// cada chamada antiga ao método novo, sem lógica própria.

import (
	"context"
	"database/sql"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/kernel"
)

type authCrypto struct{}

func novoAuth() authCrypto { return authCrypto{} }

func (authCrypto) HashPassword(cfg *config.Config, senha string) (string, error) {
	h, err := identidadecrypto.NovoHasher(cfg.HashSenha).Gerar(identidade.NovaSenha(senha))
	return h.Revelar(), err
}

func (authCrypto) VerifyPassword(cfg *config.Config, hash, senha string) bool {
	return identidadecrypto.NovoHasher(cfg.HashSenha).Verificar(identidade.NovoHashSenha(hash), identidade.NovaSenha(senha))
}

func (authCrypto) NeedsRehash(cfg *config.Config, hash string) bool {
	return identidadecrypto.NovoHasher(cfg.HashSenha).PrecisaRehash(identidade.NovoHashSenha(hash))
}

func (authCrypto) VerifyDummyPassword(cfg *config.Config, senha string) {
	identidadecrypto.NovoHasher(cfg.HashSenha).ExecutarDescartavel(identidade.NovaSenha(senha))
}

func (authCrypto) GenerateJWT(cfg *config.Config, uid int64, role string) (string, error) {
	return identidadecrypto.NovoJWT(cfg).Emitir(uid, kernel.RoleDoBanco(role))
}

func (authCrypto) ValidateJWT(token, segredo string) (*identidadecrypto.Claims, error) {
	return identidadecrypto.NovoJWT(&config.Config{JWTSecret: segredo}).Ler(token)
}

func (authCrypto) GetUsuarioByEmail(ctx context.Context, db *sql.DB, email string) (*identidade.Usuario, error) {
	return identidademysql.NovoUsuarios(db).PorEmail(ctx, email)
}

func (authCrypto) ResetPassword(ctx context.Context, db *sql.DB, id int64, hash string, deveTrocar bool) error {
	return identidademysql.NovoUsuarios(db).AtualizarSenha(ctx, id, identidade.NovoHashSenha(hash), deveTrocar)
}

func (authCrypto) RehashPassword(ctx context.Context, db *sql.DB, id int64, antigo, novo string) error {
	return identidademysql.NovoUsuarios(db).RegravarHash(ctx, id, identidade.NovoHashSenha(antigo), identidade.NovoHashSenha(novo))
}

// Funções de pacote do legado → identidadecrypto/domínio.
func gerarHashSenha(hs config.HashSenha, senha string) (string, error) {
	return authCrypto{}.HashPassword(&config.Config{HashSenha: hs}, senha)
}

func verificarSenha(hs config.HashSenha, hash, senha string) bool {
	return authCrypto{}.VerifyPassword(&config.Config{HashSenha: hs}, hash, senha)
}

func precisaRehash(hs config.HashSenha, hash string) bool {
	return authCrypto{}.NeedsRehash(&config.Config{HashSenha: hs}, hash)
}

func validarForcaSenha(s string) error { return identidade.ValidarForca(identidade.NovaSenha(s)) }

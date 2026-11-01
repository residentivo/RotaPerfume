package identidade

import "errors"

// ErrSerializarSegredo é devolvido por MarshalJSON/MarshalText dos VOs de
// segredo: Senha, HashSenha e TokenOpaco nunca viram JSON (I-12).
var ErrSerializarSegredo = errors.New("identidade: segredo não é serializável")

// mascara é a forma de log/print de todo segredo.
const mascara = "***"

// Senha é uma senha em texto claro. String/GoString devolvem "***" e a
// serialização falha: o valor só sai por Revelar, chamado pelo hasher e
// pelo envio do e-mail de senha inicial.
type Senha struct{ valor string }

// NovaSenha embrulha o texto digitado (sem normalizar: espaços contam).
func NovaSenha(s string) Senha { return Senha{valor: s} }

// Vazia informa se nada foi digitado.
func (s Senha) Vazia() bool { return s.valor == "" }

// Igual compara com outra senha (nova == atual, I-12).
func (s Senha) Igual(outra Senha) bool { return s.valor == outra.valor }

// Revelar devolve o texto claro. Uso restrito: hasher e envio de e-mail.
func (s Senha) Revelar() string { return s.valor }

func (Senha) String() string               { return mascara }
func (Senha) GoString() string             { return mascara }
func (Senha) MarshalJSON() ([]byte, error) { return nil, ErrSerializarSegredo }
func (Senha) MarshalText() ([]byte, error) { return nil, ErrSerializarSegredo }

// HashSenha é o hash gravado em usuarios.password_hash (Argon2id com pepper
// ou bcrypt legado). Opaco: só o adapter MariaDB e o hasher leem o valor.
type HashSenha struct{ valor string }

// NovoHashSenha embrulha o valor lido do banco ou gerado pelo hasher.
func NovoHashSenha(s string) HashSenha { return HashSenha{valor: s} }

// Revelar devolve o valor para gravar no banco ou verificar.
func (h HashSenha) Revelar() string { return h.valor }

func (HashSenha) String() string               { return mascara }
func (HashSenha) GoString() string             { return mascara }
func (HashSenha) MarshalJSON() ([]byte, error) { return nil, ErrSerializarSegredo }
func (HashSenha) MarshalText() ([]byte, error) { return nil, ErrSerializarSegredo }

// TokenOpaco é o refresh token em texto claro (32 bytes aleatórios em hex).
// Só o hash vai para o banco; o valor só sai por Revelar, para o cookie.
type TokenOpaco struct{ valor string }

// NovoTokenOpaco embrulha o token recebido (body/cookie) ou gerado.
func NovoTokenOpaco(s string) TokenOpaco { return TokenOpaco{valor: s} }

// Vazio informa se nenhum token foi enviado.
func (t TokenOpaco) Vazio() bool { return t.valor == "" }

// Revelar devolve o valor para o Set-Cookie e para o hash.
func (t TokenOpaco) Revelar() string { return t.valor }

func (TokenOpaco) String() string               { return mascara }
func (TokenOpaco) GoString() string             { return mascara }
func (TokenOpaco) MarshalJSON() ([]byte, error) { return nil, ErrSerializarSegredo }
func (TokenOpaco) MarshalText() ([]byte, error) { return nil, ErrSerializarSegredo }

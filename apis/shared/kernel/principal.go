package kernel

// Role é o papel do usuário, com o valor BRUTO vindo do banco (o /me continua
// devolvendo a string do banco).
//
// R-a: um valor desconhecido não é erro. Ele é tratado como restrito, como o
// `role != "admin"` de hoje; Conhecido permite ao chamador logar o caso.
type Role string

// Papéis conhecidos.
const (
	RoleAdmin  Role = "admin"
	RoleNormal Role = "normal"
)

// RoleDoBanco embrulha o valor da coluna usuarios.role sem normalizar.
func RoleDoBanco(s string) Role { return Role(s) }

// String devolve o valor bruto.
func (r Role) String() string { return string(r) }

// EhAdmin é verdadeiro só para "admin" exato (I-05).
func (r Role) EhAdmin() bool { return r == RoleAdmin }

// Conhecido informa se o valor é um dos papéis do sistema.
func (r Role) Conhecido() bool { return r == RoleAdmin || r == RoleNormal }

// Principal é o usuário autenticado de uma requisição, com o role CONFERIDO
// no banco.
//
// O zero value é inválido: todo caso de uso que recebe Principal chama
// Valido() na primeira linha e falha fechado com ErrPrincipalInvalido. Não
// existe "Principal de sistema": CLIs não recebem Principal (O5).
type Principal struct {
	userID int64
	role   Role
}

// ErrPrincipalInvalido sinaliza Principal ausente ou zero value (500).
var ErrPrincipalInvalido = NovoErro(KindInterno, "erro interno")

// NovoPrincipal cria um Principal a partir de dados JÁ CONFERIDOS no banco.
//
// Chamador permitido: somente rotaperfumes-api/internal/httpx
// (TestARQ_NovoPrincipal_SoHttpx).
func NovoPrincipal(userID int64, role Role) (Principal, error) {
	if userID <= 0 || role.String() == "" {
		return Principal{}, ErrPrincipalInvalido
	}
	return Principal{userID: userID, role: role}, nil
}

// Valido é falso para o zero value.
func (p Principal) Valido() bool { return p.userID > 0 && p.role.String() != "" }

// EhAdmin exige Principal válido e role "admin" exato.
func (p Principal) EhAdmin() bool { return p.Valido() && p.role.EhAdmin() }

// UserID devolve o id do usuário (0 no zero value).
func (p Principal) UserID() int64 { return p.userID }

// Role devolve o role do banco.
func (p Principal) Role() Role { return p.role }

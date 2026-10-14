package identidade_test

// Fakes em memória das portas de identidadeapp: os testes dos casos de uso
// controlam cada resposta (inclusive os erros de infraestrutura) sem SQL.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

var errInfra = errors.New("infra fora do ar")

type usuariosFake struct {
	porID      map[int64]*identidade.Usuario
	porEmail   map[string]*identidade.Usuario
	errPorID   []error // consumido a cada PorID (nil = segue o mapa)
	errEmail   error
	errInserir error
	errAtualiz error
	errAtivo   error
	errSenha   error
	errRehash  error
	errInvalid error
	ativos     []identidade.Usuario
	senhas     []identidade.HashSenha
	invalidou  int
	rehash     int
	proximoID  int64
}

func novoUsuariosFake(us ...*identidade.Usuario) *usuariosFake {
	f := &usuariosFake{porID: map[int64]*identidade.Usuario{}, porEmail: map[string]*identidade.Usuario{}, proximoID: 100}
	for _, u := range us {
		f.porID[u.ID] = u
		f.porEmail[u.Email] = u
	}
	return f
}

func (f *usuariosFake) PorEmail(_ context.Context, email string) (*identidade.Usuario, error) {
	if f.errEmail != nil {
		return nil, f.errEmail
	}
	if u, ok := f.porEmail[email]; ok {
		c := *u
		return &c, nil
	}
	return nil, identidade.ErrUsuarioNaoEncontrado
}

func (f *usuariosFake) PorID(_ context.Context, id int64) (*identidade.Usuario, error) {
	if len(f.errPorID) > 0 {
		err := f.errPorID[0]
		f.errPorID = f.errPorID[1:]
		if err != nil {
			return nil, err
		}
	}
	if u, ok := f.porID[id]; ok {
		c := *u
		return &c, nil
	}
	return nil, identidade.ErrUsuarioNaoEncontrado
}

func (f *usuariosFake) StatusParaAutenticacao(_ context.Context, id int64) (identidade.StatusAcesso, error) {
	if u, ok := f.porID[id]; ok {
		return identidade.StatusAcesso{Ativo: u.Ativo, Role: u.Role}, nil
	}
	return identidade.StatusAcesso{}, identidade.ErrUsuarioNaoEncontrado
}

func (f *usuariosFake) Listar(_ context.Context, _ kernel.Ordenacao[identidade.CampoUsuario], p kernel.Pagina) (kernel.Resultado[identidade.Usuario], error) {
	return kernel.NovoResultado(f.ativos, p, len(f.ativos)), nil
}

func (f *usuariosFake) ListarAtivos(context.Context) ([]identidade.Usuario, error) {
	return f.ativos, nil
}

func (f *usuariosFake) Inserir(_ context.Context, u *identidade.Usuario) error {
	if f.errInserir != nil {
		return f.errInserir
	}
	f.proximoID++
	u.ID = f.proximoID
	return nil
}

func (f *usuariosFake) Atualizar(context.Context, int64, identidade.UsuarioValidado) error {
	return f.errAtualiz
}

func (f *usuariosFake) DefinirAtivo(context.Context, int64, bool) error { return f.errAtivo }

func (f *usuariosFake) AtualizarSenha(_ context.Context, _ int64, h identidade.HashSenha, _ bool) error {
	if f.errSenha != nil {
		return f.errSenha
	}
	f.senhas = append(f.senhas, h)
	return nil
}

func (f *usuariosFake) RegravarHash(context.Context, int64, identidade.HashSenha, identidade.HashSenha) error {
	f.rehash++
	return f.errRehash
}

func (f *usuariosFake) RegistrarLogin(context.Context, int64, time.Time) error { return nil }

func (f *usuariosFake) InvalidarSessoes(context.Context, int64, time.Time) error {
	f.invalidou++
	return f.errInvalid
}

type tokensFake struct {
	mu         sync.Mutex
	porHash    map[string]*identidade.RefreshToken
	errHash    error
	errInserir error
	errRevogar error
	errTodos   error
	revogados  []identidade.MotivoRevogacao
	inseridos  int
}

func novoTokensFake(rts ...*identidade.RefreshToken) *tokensFake {
	f := &tokensFake{porHash: map[string]*identidade.RefreshToken{}}
	for _, rt := range rts {
		f.porHash[rt.HashToken] = rt
	}
	return f
}

func (f *tokensFake) PorHash(_ context.Context, h string) (*identidade.RefreshToken, error) {
	if f.errHash != nil {
		return nil, f.errHash
	}
	if rt, ok := f.porHash[h]; ok {
		return rt, nil
	}
	return nil, identidade.ErrRefreshInvalido
}

func (f *tokensFake) Inserir(_ context.Context, rt *identidade.RefreshToken) error {
	if f.errInserir != nil {
		return f.errInserir
	}
	f.inseridos++
	rt.ID = int64(1000 + f.inseridos)
	return nil
}

func (f *tokensFake) Revogar(_ context.Context, _ int64, m identidade.MotivoRevogacao) error {
	if f.errRevogar != nil {
		return f.errRevogar
	}
	f.revogados = append(f.revogados, m)
	return nil
}

func (f *tokensFake) RevogarTodosDoUsuario(_ context.Context, _ int64, m identidade.MotivoRevogacao) error {
	if f.errTodos != nil {
		return f.errTodos
	}
	f.revogados = append(f.revogados, m)
	return nil
}

func (f *tokensFake) MarcarReuso(context.Context, int64, time.Time, time.Time) (bool, error) {
	return true, nil
}

func (f *tokensFake) DesfazerReuso(context.Context, int64, time.Time) error { return nil }

func (f *tokensFake) ApagarExpirados(context.Context, time.Time, int) (int64, error) { return 0, nil }

type historicoFake struct {
	itens       []identidade.RegistroSenha
	errListar   error
	errRegistro error
	registros   int
}

func (f *historicoFake) Registrar(context.Context, *identidade.RegistroSenha) error {
	f.registros++
	return f.errRegistro
}

func (f *historicoFake) Todos(_ context.Context, _ kernel.Ordenacao[identidade.CampoHistorico], p kernel.Pagina) (kernel.Resultado[identidade.RegistroSenha], error) {
	return kernel.NovoResultado(f.itens, p, len(f.itens)), f.errListar
}

func (f *historicoFake) DoUsuario(_ context.Context, _ int64, _ kernel.Ordenacao[identidade.CampoHistorico], p kernel.Pagina) (kernel.Resultado[identidade.RegistroSenha], error) {
	return kernel.NovoResultado(f.itens, p, len(f.itens)), f.errListar
}

// txFake roda a fn com os repositórios do próprio fake (sem SQL).
type txFake struct{ tokens *tokensFake }

func (t txFake) WithTx(_ context.Context, fn func(identidadeapp.TxRotacao) error) error {
	return fn(identidadeapp.TxRotacao{Tokens: t.tokens})
}

// hasherFake: o "hash" de s é "h:"+s.
type hasherFake struct {
	errGerar error
	rehash   bool
}

func (h *hasherFake) Gerar(s identidade.Senha) (identidade.HashSenha, error) {
	if h.errGerar != nil {
		return identidade.HashSenha{}, h.errGerar
	}
	return identidade.NovoHashSenha("h:" + s.Revelar()), nil
}

func (h *hasherFake) Verificar(hash identidade.HashSenha, s identidade.Senha) bool {
	return hash.Revelar() == "h:"+s.Revelar()
}

func (h *hasherFake) PrecisaRehash(identidade.HashSenha) bool { return h.rehash }
func (h *hasherFake) ExecutarDescartavel(identidade.Senha)    {}

type emissorFake struct{ err error }

func (e emissorFake) Emitir(int64, kernel.Role) (string, error) { return "jwt", e.err }

type geradorRefreshFake struct{}

func (geradorRefreshFake) Novo() identidade.TokenOpaco         { return identidade.NovoTokenOpaco("novo") }
func (geradorRefreshFake) Hash(t identidade.TokenOpaco) string { return "hash:" + t.Revelar() }

// geradorSenhaFake devolve as senhas em sequência (a última se repete).
type geradorSenhaFake struct {
	senhas []string
	err    error
	n      int
}

func (g *geradorSenhaFake) Gerar() (identidade.Senha, error) {
	if g.err != nil {
		return identidade.Senha{}, g.err
	}
	i := min(g.n, len(g.senhas)-1)
	g.n++
	return identidade.NovaSenha(g.senhas[i]), nil
}

type captchaFake struct{}

func (captchaFake) Verificar(_ context.Context, token, _ string) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("ausente")
	}
	return nil
}

// limitadorFake bloqueia as chaves com o prefixo informado.
type limitadorFake struct{ bloquear string }

func (l limitadorFake) Bloqueado(chave string) (bool, time.Duration) {
	return l.bloquear != "" && strings.HasPrefix(chave, l.bloquear), 42 * time.Second
}
func (limitadorFake) RegistrarFalha(string)   {}
func (limitadorFake) RegistrarSucesso(string) {}

type emailFake struct{ err error }

func (e emailFake) EnviarSenhaInicial(context.Context, string, string, string) error { return e.err }

type vendedoresFake struct {
	existe    bool
	errExiste error
	desligado bool
	errDeslig error
}

func (v vendedoresFake) Existe(context.Context, int64) (bool, error) { return v.existe, v.errExiste }
func (v vendedoresFake) Desligado(context.Context, int64) (bool, error) {
	return v.desligado, v.errDeslig
}

// ambiente é o conjunto de fakes de um teste.
type ambiente struct {
	usuarios   *usuariosFake
	tokens     *tokensFake
	historico  *historicoFake
	hasher     *hasherFake
	emissor    emissorFake
	senhas     *geradorSenhaFake
	limite     limitadorFake
	email      emailFake
	vendedores vendedoresFake
	agora      time.Time
}

func novoAmbiente(us ...*identidade.Usuario) *ambiente {
	return &ambiente{
		usuarios: novoUsuariosFake(us...), tokens: novoTokensFake(), historico: &historicoFake{},
		hasher: &hasherFake{}, senhas: &geradorSenhaFake{senhas: []string{"Gerada#1234567"}},
		agora: time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local),
	}
}

func (a *ambiente) deps() identidadeapp.Deps {
	l := a.limite
	return identidadeapp.Deps{
		Usuarios: a.usuarios, Tokens: a.tokens, Historico: a.historico, Rotacao: txFake{a.tokens},
		Hasher: a.hasher, Emissor: a.emissor, Refresh: geradorRefreshFake{}, Senhas: a.senhas,
		Captcha: captchaFake{}, Limites: identidadeapp.Limites{Login: l, Refresh: l, Reset: l},
		Publicador: identidadeapp.SemPublicador{}, Email: a.email, Vendedores: a.vendedores,
		Relogio: func() time.Time { return a.agora },
	}
}

func (a *ambiente) casos() *identidadeapp.Casos {
	return identidadeapp.Novo(a.deps(), identidadeapp.Opcoes{})
}

func principal(id int64, role kernel.Role) kernel.Principal {
	p, _ := kernel.NovoPrincipal(id, role)
	return p
}

func usuarioAtivo(id int64, email, senha string, role kernel.Role) *identidade.Usuario {
	return &identidade.Usuario{ID: id, Nome: "U", Email: email, Role: role, Ativo: true, Hash: identidade.NovoHashSenha("h:" + senha)}
}

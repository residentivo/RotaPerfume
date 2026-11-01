package routes_test

// SEC-23 (Lote 28): matriz da troca de senha pendente. Só as rotas com
// liberaTroca na matriz (POST /api/auth/reset-password e GET /api/auth/me)
// atendem o usuário com deve_trocar_senha; toda outra rota protegida
// responde 403 "troca de senha obrigatória" sem chegar ao handler, para
// qualquer papel. As públicas não passam pelo Autenticador.

import (
	"net/http"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

const msgTrocaSenhaObrigatoria = "troca de senha obrigatória"

func TestSEC23_Matriz_LiberacaoRegistradaIgualAMatriz(t *testing.T) {
	naMatriz := make(map[string]bool, len(matrizRotas))
	for _, r := range matrizRotas {
		naMatriz[r.padrao] = r.liberaTroca
	}
	var liberadas, divergentes []string
	for _, rt := range arq00RotasRegistradas(t) {
		pd := rt.Metodo + " " + rt.Padrao
		if rt.LiberaTrocaSenhaPendente {
			liberadas = append(liberadas, pd)
		}
		if rt.LiberaTrocaSenhaPendente != naMatriz[pd] {
			divergentes = append(divergentes, pd)
		}
	}
	sort.Strings(liberadas)
	assert.Empty(t, divergentes, "LiberaTrocaSenhaPendente registrada diverge da matriz")
	assert.Equal(t, []string{"GET /api/auth/me", "POST /api/auth/reset-password"}, liberadas,
		"só a troca de senha e o /me atendem quem tem troca pendente (revisão do SecBrain para mudar)")
}

func TestSEC23_Matriz_TrocaPendenteBloqueiaRotasProtegidas(t *testing.T) {
	for _, papel := range []struct {
		uid  int64
		role string
	}{{1, "admin"}, {2, "normal"}} {
		checker := fakeUserStatusChecker{status: map[int64]identidade.StatusAcesso{
			papel.uid: {Ativo: true, Role: kernel.RoleDoBanco(papel.role), DeveTrocarSenha: true},
		}}
		for _, r := range matrizRotas {
			if !protegida(r.politica) {
				continue
			}
			for _, m := range r.metodos() {
				for _, pth := range r.paths {
					t.Run(papel.role+" | "+r.padrao+" | "+pth, func(t *testing.T) {
						h, mock := arq00Router(t, checker)
						w := arq00Serve(h, m, pth, token(t, papel.uid, papel.role), "")
						if r.liberaTroca {
							assert.NotContains(t, w.Body.String(), msgTrocaSenhaObrigatoria, "rota liberada passa pelo middleware")
							return
						}
						assert.Equal(t, http.StatusForbidden, w.Code)
						assert.JSONEq(t, `{"success":false,"error":"`+msgTrocaSenhaObrigatoria+`"}`, w.Body.String())
						assertSecurityHeaders(t, w)
						assert.NoError(t, mock.ExpectationsWereMet(), "barrado antes do handler, sem tocar o banco")
					})
				}
			}
		}
	}
}

func TestSEC23_Matriz_RotasPublicasNaoConsultamATroca(t *testing.T) {
	for _, c := range casosDaMatriz(func(p politicaRota) bool { return p == politicaPublic }) {
		t.Run(c.nome, func(t *testing.T) {
			calls := 0
			checker := fakeUserStatusChecker{calls: &calls, status: map[int64]identidade.StatusAcesso{
				2: {Ativo: true, Role: "normal", DeveTrocarSenha: true},
			}}
			h, _ := arq00Router(t, checker)
			w := arq00Serve(h, c.method, c.path, token(t, 2, "normal"), "")
			assert.NotContains(t, w.Body.String(), msgTrocaSenhaObrigatoria)
			assert.Equal(t, 0, calls, "rota pública não consulta o checker")
		})
	}
}

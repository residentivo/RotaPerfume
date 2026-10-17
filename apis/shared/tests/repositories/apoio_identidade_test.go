package repositories_test

// ARQ-07: os testes de Identidade chamam os adapters de identidademysql
// diretamente (os repassadores com a assinatura legada saíram). Aqui ficam só
// o parse de order_by/order_dir que o identidadehttp faz antes do caso de uso
// e o desembrulho de kernel.Resultado para as asserções de itens e total.

import (
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

// itensETotal devolve itens, total e erro de uma listagem paginada.
func itensETotal[T any](r kernel.Resultado[T], err error) ([]T, int, error) {
	return r.Itens, r.Total, err
}

// ordemUsuario é o parse de order_by/order_dir da listagem de usuários.
func ordemUsuario(campo, dir string) kernel.Ordenacao[identidade.CampoUsuario] {
	return kernel.ParseOrdenacao(campo, dir, identidade.CamposOrdenacaoUsuario(), identidade.OrdenacaoUsuarioPadrao())
}

// ordemHistorico é o parse de order_by/order_dir do histórico de senhas.
func ordemHistorico(campo, dir string) kernel.Ordenacao[identidade.CampoHistorico] {
	return kernel.ParseOrdenacao(campo, dir, identidade.CamposOrdenacaoHistorico(), identidade.OrdenacaoHistoricoPadrao())
}

// ptrInt64 devolve o endereço de v (campos opcionais do domínio).
func ptrInt64(v int64) *int64 { return &v }

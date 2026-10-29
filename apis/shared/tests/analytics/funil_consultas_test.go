package analytics_test

// DASH-01 (caso de uso): Consultas.Funil decide o acesso pelo autorizador
// ANTES do repositório (D-01/O1) e repassa escopo e janela sem alteração.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsapp"
	"github.com/rotaperfumes/shared/kernel"
)

// funilFake grava a chamada (escopo e janela) e devolve as contagens ou o
// erro configurados.
type funilFake struct {
	contagens []analytics.ContagemEtapa
	err       error
	chamadas  int
	escopo    kernel.Escopo
	janela    analytics.Janela
}

func (f *funilFake) Funil(_ context.Context, e kernel.Escopo, j analytics.Janela) ([]analytics.ContagemEtapa, error) {
	f.chamadas++
	f.escopo, f.janela = e, j
	return f.contagens, f.err
}

func consultasFunil(f *funilFake, e escopoFake) *analyticsapp.Consultas {
	return analyticsapp.Novo(analyticsapp.Deps{Vendas: &vendasFake{}, Clientes: &clientesFake{}, Funil: f, Escopo: e,
		Relogio: func() time.Time { return hojeFixo }})
}

func TestDASH01_Consultas_Funil_Liberado(t *testing.T) {
	doVendedor, err := kernel.EscopoDoVendedor(7)
	require.NoError(t, err)
	contagens := []analytics.ContagemEtapa{
		{Etapa: analytics.EtapaProspeccao, Quantidade: 2, Valor: 20},
		{Etapa: analytics.EtapaGanho, Quantidade: 1, Valor: 10},
		{Etapa: analytics.EtapaPerdido, Quantidade: 1, Valor: 5},
	}
	casos := []struct {
		nome   string
		escopo kernel.Escopo
		janela analytics.Janela
	}{
		{"admin all", irrestrito(t), analytics.JanelaTudo},
		{"admin 30d", irrestrito(t), analytics.Janela30d},
		{"vendedor 90d", doVendedor, analytics.Janela90d},
		{"vendedor 365d", doVendedor, analytics.Janela365d},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			repo := &funilFake{contagens: contagens}
			f, err := consultasFunil(repo, escopoFake{e: c.escopo}).Funil(context.Background(), analyticsapp.ConsultaFunil{Janela: c.janela})
			require.NoError(t, err)
			assert.Equal(t, 1, repo.chamadas)
			assert.Equal(t, c.escopo, repo.escopo, "o repositório recebe o escopo resolvido")
			assert.Equal(t, c.janela, repo.janela)
			assert.Equal(t, analytics.NovoFunil(c.janela, contagens), f)
			assert.False(t, f.VendedorDesligado)
		})
	}
}

// D-01/O1: desligado, SemAcesso e zero value respondem o funil zerado SEM
// chamar o repositório; só o desligado leva o aviso.
func TestDASH01_Consultas_Funil_SemAcessoZeradoSemRepositorio(t *testing.T) {
	casos := []struct {
		nome      string
		esc       escopoFake
		desligado bool
	}{
		{"desligado", escopoFake{e: kernel.EscopoSemAcesso(), desligado: true}, true},
		{"desligado com escopo irrestrito (defesa)", escopoFake{e: irrestrito(t), desligado: true}, true},
		{"sem acesso", escopoFake{e: kernel.EscopoSemAcesso()}, false},
		{"zero value", escopoFake{e: kernel.Escopo{}}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			repo := &funilFake{err: errInfra}
			f, err := consultasFunil(repo, c.esc).Funil(context.Background(), analyticsapp.ConsultaFunil{Janela: analytics.Janela90d})
			require.NoError(t, err)
			assert.Zero(t, repo.chamadas, "sem acesso não chega ao repositório")
			assert.Equal(t, analytics.FunilZerado(analytics.Janela90d, c.desligado), f)
			assert.Len(t, f.Etapas, 5)
		})
	}
}

func TestDASH01_Consultas_Funil_Erros(t *testing.T) {
	t.Run("erro do autorizador é erro (R3), sem repositório", func(t *testing.T) {
		repo := &funilFake{}
		f, err := consultasFunil(repo, escopoFake{err: errInfra}).Funil(context.Background(), analyticsapp.ConsultaFunil{})
		assert.ErrorIs(t, err, errInfra)
		assert.Equal(t, analytics.Funil{}, f)
		assert.Zero(t, repo.chamadas)
	})
	t.Run("erro do repositório é erro, sem resposta parcial", func(t *testing.T) {
		repo := &funilFake{err: errInfra, contagens: []analytics.ContagemEtapa{{Etapa: analytics.EtapaGanho, Quantidade: 1}}}
		f, err := consultasFunil(repo, escopoFake{e: irrestrito(t)}).Funil(context.Background(), analyticsapp.ConsultaFunil{Janela: analytics.Janela30d})
		assert.ErrorIs(t, err, errInfra)
		assert.Equal(t, analytics.Funil{}, f)
		assert.Equal(t, 1, repo.chamadas)
	})
}

func TestDASH01_Novo_ExigeFunil(t *testing.T) {
	assert.PanicsWithValue(t, "analyticsapp: Novo exige todas as dependências", func() {
		analyticsapp.Novo(analyticsapp.Deps{Vendas: &vendasFake{}, Clientes: &clientesFake{}, Escopo: escopoFake{},
			Relogio: func() time.Time { return hojeFixo }})
	})
}

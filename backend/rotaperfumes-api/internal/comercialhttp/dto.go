// Package comercialhttp é a camada HTTP do contexto Comercial (ARQ-04):
// handlers finos (parse/DTO → caso de uso → envelope httpx). As rotas e as
// políticas ficam na tabela do bootstrap; o escopo de carteira é decidido
// pelos casos de uso (comercialapp), nunca aqui.
//
// Os DTOs têm as mesmas chaves, ordem e tipos dos models legados (contrato
// JSON inalterado). Como as entidades do domínio têm os mesmos campos, a
// conversão é a conversão de tipo do Go (as tags não contam).
package comercialhttp

import (
	"net/http"
	"strings"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialapp"
	"github.com/rotaperfumes/shared/kernel"
)

// ---------------------------------------------------------------------------
// Respostas
// ---------------------------------------------------------------------------

type vendedorJSON struct {
	ID               int64      `json:"id"`
	Nome             string     `json:"nome"`
	Regiao           string     `json:"regiao"`
	UF               string     `json:"uf"`
	DataAdmissao     time.Time  `json:"data_admissao"`
	DataDesligamento *time.Time `json:"data_desligamento"`
	MetaMensal       float64    `json:"meta_mensal"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func novoVendedorJSON(v comercial.Vendedor) vendedorJSON { return vendedorJSON(v) }

type vendedorResumoJSON struct {
	ID               int64      `json:"id"`
	Nome             string     `json:"nome"`
	Regiao           string     `json:"regiao"`
	UF               string     `json:"uf"`
	DataDesligamento *time.Time `json:"data_desligamento"`
}

func novoVendedorResumoJSON(v comercial.VendedorResumo) vendedorResumoJSON {
	return vendedorResumoJSON(v)
}

// detalheVendedorJSON é o vendedor (campos "achatados") + "clientes".
type detalheVendedorJSON struct {
	vendedorJSON
	Clientes []clienteResumoJSON `json:"clientes"`
}

func novoDetalheVendedorJSON(d comercial.DetalheVendedor) detalheVendedorJSON {
	return detalheVendedorJSON{vendedorJSON: novoVendedorJSON(d.Vendedor), Clientes: httpx.Lista(d.Clientes, novoClienteResumoJSON)}
}

type clienteResumoJSON struct {
	ID          int64      `json:"id"`
	CNPJ        string     `json:"cnpj"`
	RazaoSocial string     `json:"razao_social"`
	Segmento    string     `json:"segmento"`
	Cidade      string     `json:"cidade"`
	UF          string     `json:"uf"`
	CarteiraID  int64      `json:"carteira_id"`
	DataInicio  time.Time  `json:"data_inicio"`
	DataFim     *time.Time `json:"data_fim"`
}

func novoClienteResumoJSON(c comercial.ClienteResumo) clienteResumoJSON { return clienteResumoJSON(c) }

type clienteJSON struct {
	ClienteIDOrigem int64     `json:"cliente_id_origem"`
	CNPJ            string    `json:"cnpj"`
	RazaoSocial     string    `json:"razao_social"`
	Segmento        string    `json:"segmento"`
	Cidade          string    `json:"cidade"`
	UF              string    `json:"uf"`
	Bairro          string    `json:"bairro"`
	DataCadastro    time.Time `json:"data_cadastro"`
	Ativo           bool      `json:"ativo"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func novoClienteJSON(c comercial.Cliente) clienteJSON { return clienteJSON(c) }

type visitaJSON struct {
	VisitaID   int64     `json:"visita_id"`
	ClienteID  int64     `json:"cliente_id"`
	VendedorID int64     `json:"vendedor_id"`
	DataVisita time.Time `json:"data_visita"`
	Resultado  string    `json:"resultado"`
	DuracaoMin int       `json:"duracao_min"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func novoVisitaJSON(v comercial.Visita) visitaJSON { return visitaJSON(v) }

type oportunidadeJSON struct {
	OportunidadeID   int64      `json:"oportunidade_id"`
	ClienteID        int64      `json:"cliente_id"`
	VendedorID       int64      `json:"vendedor_id"`
	Origem           string     `json:"origem"`
	DataAbertura     time.Time  `json:"data_abertura"`
	Etapa            string     `json:"etapa"`
	ProbabilidadePct float64    `json:"probabilidade_pct"`
	ValorEstimado    float64    `json:"valor_estimado"`
	DataFechamento   *time.Time `json:"data_fechamento"`
	CicloDias        *int       `json:"ciclo_dias"`
	MotivoPerda      *string    `json:"motivo_perda"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func novoOportunidadeJSON(o comercial.Oportunidade) oportunidadeJSON { return oportunidadeJSON(o) }

// ---------------------------------------------------------------------------
// Requisições (um DTO por forma de body; create e update têm a mesma forma
// em cada entidade, como no legado)
// ---------------------------------------------------------------------------

type vendedorRequest struct {
	Nome         string  `json:"nome"`
	Regiao       string  `json:"regiao"`
	UF           string  `json:"uf"`
	DataAdmissao string  `json:"data_admissao"` // AAAA-MM-DD (opcional na criação)
	MetaMensal   float64 `json:"meta_mensal"`
}

type vincularClienteRequest struct {
	ClienteID int64 `json:"cliente_id"`
}

type clienteRequest struct {
	CNPJ         string `json:"cnpj"`
	RazaoSocial  string `json:"razao_social"`
	Segmento     string `json:"segmento"`
	Cidade       string `json:"cidade"`
	UF           string `json:"uf"`
	Bairro       string `json:"bairro"`
	DataCadastro string `json:"data_cadastro"` // AAAA-MM-DD (opcional na criação)
}

type visitaRequest struct {
	ClienteID  int64  `json:"cliente_id"`
	VendedorID int64  `json:"vendedor_id"` // ignorado para usuário restrito (E-05)
	DataVisita string `json:"data_visita"`
	Resultado  string `json:"resultado"`
	DuracaoMin int    `json:"duracao_min"`
}

type oportunidadeRequest struct {
	ClienteID        int64   `json:"cliente_id"`
	VendedorID       int64   `json:"vendedor_id"` // ignorado para usuário restrito (E-05)
	Origem           string  `json:"origem"`
	DataAbertura     string  `json:"data_abertura"`
	Etapa            string  `json:"etapa"`
	ProbabilidadePct float64 `json:"probabilidade_pct"`
	ValorEstimado    float64 `json:"valor_estimado"`
	DataFechamento   string  `json:"data_fechamento"`
	CicloDias        *int    `json:"ciclo_dias"`
	MotivoPerda      string  `json:"motivo_perda"`
}

// leitor adia o decode do body para depois da autorização do caso de uso
// ("escopo antes do decode", E-05). Body inválido → 400 "body JSON inválido".
func leitor[Req, D any](r *http.Request, conv func(Req) D) comercialapp.Leitor[D] {
	return httpx.LeitorJSON(r, conv)
}

func dadosVendedor(q vendedorRequest) comercial.DadosVendedor { return comercial.DadosVendedor(q) }
func dadosCliente(q clienteRequest) comercial.DadosCliente    { return comercial.DadosCliente(q) }
func dadosVisita(q visitaRequest) comercial.DadosVisita       { return comercial.DadosVisita(q) }
func dadosOportunidade(q oportunidadeRequest) comercial.DadosOportunidade {
	return comercial.DadosOportunidade(q)
}

// ---------------------------------------------------------------------------
// Query strings
// ---------------------------------------------------------------------------

func texto(r *http.Request, nome string) string { return strings.TrimSpace(r.URL.Query().Get(nome)) }

// Os filtros devolvem httpx.ErrBuscaMuitoLonga para q acima de 100
// caracteres (SEC-25); o handler responde 400 antes do caso de uso.

func filtroClienteDaQuery(r *http.Request) (comercial.FiltroCliente, error) {
	busca, err := httpx.BuscaDaQuery(r)
	if err != nil {
		return comercial.FiltroCliente{}, err
	}
	return comercial.FiltroCliente{
		UF: texto(r, "uf"), Segmento: texto(r, "segmento"), Ativo: httpx.BoolDaQuery(r, "ativo"), Busca: busca,
		Ordenacao: httpx.OrdenacaoDaQuery(r, comercial.CamposOrdenacaoCliente(), comercial.OrdenacaoClientePadrao()),
	}, nil
}

func filtroVisitaDaQuery(r *http.Request) (comercial.FiltroVisita, error) {
	busca, err := httpx.BuscaDaQuery(r)
	if err != nil {
		return comercial.FiltroVisita{}, err
	}
	return comercial.FiltroVisita{
		ClienteID: httpx.Int64DaQuery(r, "cliente_id"), VendedorID: httpx.Int64DaQuery(r, "vendedor_id"),
		Resultado: texto(r, "resultado"), De: texto(r, "data_visita_de"), Ate: texto(r, "data_visita_ate"), Busca: busca,
		Ordenacao: httpx.OrdenacaoDaQuery(r, comercial.CamposOrdenacaoVisita(), comercial.OrdenacaoVisitaPadrao()),
	}, nil
}

func filtroOportunidadeDaQuery(r *http.Request) (comercial.FiltroOportunidade, error) {
	busca, err := httpx.BuscaDaQuery(r)
	if err != nil {
		return comercial.FiltroOportunidade{}, err
	}
	return comercial.FiltroOportunidade{
		ClienteID: httpx.Int64DaQuery(r, "cliente_id"), VendedorID: httpx.Int64DaQuery(r, "vendedor_id"),
		Etapa: texto(r, "etapa"), Origem: texto(r, "origem"), De: texto(r, "data_abertura_de"), Ate: texto(r, "data_abertura_ate"),
		Busca:     busca,
		Ordenacao: httpx.OrdenacaoDaQuery(r, comercial.CamposOrdenacaoOportunidade(), comercial.OrdenacaoOportunidadePadrao()),
	}, nil
}

// ---------------------------------------------------------------------------
// Apoio dos handlers
// ---------------------------------------------------------------------------

// principal devolve o Principal do request autenticado; se faltar (rota mal
// montada), 500 "erro interno" sem chamar o caso de uso (§9).
func principal(w http.ResponseWriter, r *http.Request, tag string) (kernel.Principal, bool) {
	p, ok := httpx.PrincipalFrom(r)
	if !ok {
		httpx.ResponderErro(w, tag, kernel.ErrPrincipalInvalido)
	}
	return p, ok
}

// principalEID junta principal (rotas de gestão, AdminOnly) e o {id}.
func principalEID(w http.ResponseWriter, r *http.Request, tag string) (kernel.Principal, int64, bool) {
	p, ok := principal(w, r, tag)
	if !ok {
		return p, 0, false
	}
	id, ok := idDoPath(w, r, tag)
	return p, id, ok
}

// principalDoEscopo é o Principal das rotas com escopo de carteira. A falta
// dele (rota mal montada) NÃO vira 500 aqui: o AutorizadorEscopo trata o
// Principal inválido como SemAcesso, o escopo mais restritivo, sem consulta
// (contrato legado, TestEscopo_SemUsuarioNoContexto).
func principalDoEscopo(r *http.Request) kernel.Principal {
	p, _ := httpx.PrincipalFrom(r)
	return p
}

// idDoPath lê o {id} com o parse permissivo do legado (-1 e 0 seguem e dão
// 404); não numérico → 400 "id inválido".
func idDoPath(w http.ResponseWriter, r *http.Request, tag string) (int64, bool) {
	return httpx.IDOuErro(w, r, tag, "id")
}

// responder escreve o erro pelo mapper ou o objeto convertido no status.
func responder[T, D any](w http.ResponseWriter, tag string, status int, v *T, err error, conv func(T) D) {
	httpx.Responder(w, tag, status, v, err, conv)
}

// responderLista escreve a lista (nil → null; vazia → []).
func responderLista[T, D any](w http.ResponseWriter, tag string, vs []T, err error, conv func(T) D) {
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.Lista(vs, conv), "")
}

// responderPaginado escreve a listagem paginada.
func responderPaginado[T, D any](w http.ResponseWriter, tag string, res kernel.Resultado[T], err error, conv func(T) D) {
	httpx.ResponderPaginado(w, tag, res, err, conv)
}

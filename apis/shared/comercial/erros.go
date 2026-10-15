// Package comercial é o domínio do contexto Comercial (ARQ-04): Vendedor,
// Carteira (vínculo cliente↔vendedor), Cliente com o VO CNPJ, Visita e
// Oportunidade, as ordenações tipadas (C-02) e a regra pura de escrita sob
// escopo de carteira (E-05).
//
// O pacote é puro (docs/arquitetura-ddd.md §4.1): importa só a stdlib, o
// kernel e shared/cnpj. Não loga, não conhece SQL nem HTTP e não tem tags
// JSON (o DTO HTTP é que serializa). As entidades têm campos exportados com
// os mesmos nomes e tipos dos models legados (registro ARQ-02, divergência b).
package comercial

import "github.com/rotaperfumes/shared/kernel"

// Erros de escopo de carteira (E-01, E-05). "Não é seu" nunca é
// KindProibido: fora da carteira responde com o mesmo NaoEncontrado de
// "não existe" (E-04).
var (
	ErrVendedorDesligado     = kernel.NovoErro(kernel.KindProibido, "acesso bloqueado: vendedor desligado")
	ErrSemVendedor           = kernel.NovoErro(kernel.KindProibido, "usuário sem vendedor vinculado")
	ErrClienteForaDaCarteira = kernel.NovoErro(kernel.KindInvalido, "cliente não pertence à carteira deste vendedor")
)

// Erros de Vendedor e Carteira (mensagens do vendedor_service.go legado).
var (
	ErrVendedorNaoEncontrado = kernel.NovoErro(kernel.KindNaoEncontrado, "vendedor não encontrado")
	ErrNomeObrigatorio       = kernel.NovoErro(kernel.KindInvalido, "nome é obrigatório")
	ErrRegiaoObrigatoria     = kernel.NovoErro(kernel.KindInvalido, "regiao é obrigatória")
	ErrUFInvalida            = kernel.NovoErro(kernel.KindInvalido, "uf deve ter 2 letras")
	ErrDataAdmissaoInvalida  = kernel.NovoErro(kernel.KindInvalido, "data_admissao inválida (use o formato AAAA-MM-DD)")
	ErrMetaMensalInvalida    = kernel.NovoErro(kernel.KindInvalido, "meta_mensal deve ser maior ou igual a zero")
	ErrVinculoNaoEncontrado  = kernel.NovoErro(kernel.KindNaoEncontrado, "vínculo entre cliente e vendedor não encontrado")
	ErrClienteIDObrigatorio  = kernel.NovoErro(kernel.KindInvalido, "cliente_id é obrigatório")
	// ErrVinculoDuplicado: 1062 em carteiras (corrida no vínculo do dia).
	// Continua 500 como no legado, mas sem o valor do driver no log (N-8).
	ErrVinculoDuplicado = kernel.NovoErro(kernel.KindInterno, "erro interno")
)

// Erros de Cliente (mensagens do cliente_service.go legado).
var (
	ErrClienteNaoEncontrado   = kernel.NovoErro(kernel.KindNaoEncontrado, "cliente não encontrado")
	ErrRazaoSocialObrigatoria = kernel.NovoErro(kernel.KindInvalido, "razão social é obrigatória")
	ErrCNPJObrigatorio        = kernel.NovoErro(kernel.KindInvalido, "cnpj é obrigatório")
	ErrCNPJInvalido           = kernel.NovoErro(kernel.KindInvalido, "cnpj inválido")
	// ErrCNPJDuplicado é genérico de propósito (E-07, NEG-01): não revela id,
	// vendedor nem razão social do cliente que já usa o CNPJ.
	ErrCNPJDuplicado        = kernel.NovoErro(kernel.KindConflito, "cnpj já cadastrado")
	ErrSegmentoObrigatorio  = kernel.NovoErro(kernel.KindInvalido, "segmento é obrigatório")
	ErrCidadeObrigatoria    = kernel.NovoErro(kernel.KindInvalido, "cidade é obrigatória")
	ErrDataCadastroInvalida = kernel.NovoErro(kernel.KindInvalido, "data_cadastro inválida (use o formato AAAA-MM-DD)")
)

// Erros comuns a Visita e Oportunidade (mesmas mensagens nos dois legados).
var (
	ErrClienteInvalido  = kernel.NovoErro(kernel.KindInvalido, "cliente_id é obrigatório e deve existir")
	ErrVendedorInvalido = kernel.NovoErro(kernel.KindInvalido, "vendedor_id é obrigatório e deve existir")
)

// Erros de Visita (visita_service.go legado).
var (
	ErrVisitaNaoEncontrada  = kernel.NovoErro(kernel.KindNaoEncontrado, "visita não encontrada")
	ErrDataVisitaInvalida   = kernel.NovoErro(kernel.KindInvalido, "data_visita inválida (use o formato AAAA-MM-DD)")
	ErrResultadoObrigatorio = kernel.NovoErro(kernel.KindInvalido, "resultado é obrigatório")
	ErrDuracaoInvalida      = kernel.NovoErro(kernel.KindInvalido, "duracao_min deve ser maior ou igual a zero")
)

// Erros de Oportunidade (oportunidade_service.go legado).
var (
	ErrOportunidadeNaoEncontrada = kernel.NovoErro(kernel.KindNaoEncontrado, "oportunidade não encontrada")
	ErrOrigemObrigatoria         = kernel.NovoErro(kernel.KindInvalido, "origem é obrigatória")
	ErrEtapaObrigatoria          = kernel.NovoErro(kernel.KindInvalido, "etapa é obrigatória")
	ErrProbabilidadeInvalida     = kernel.NovoErro(kernel.KindInvalido, "probabilidade_pct deve estar entre 0 e 100")
	ErrValorEstimadoInvalido     = kernel.NovoErro(kernel.KindInvalido, "valor_estimado deve ser maior ou igual a zero")
	ErrDataAberturaInvalida      = kernel.NovoErro(kernel.KindInvalido, "data_abertura inválida (use o formato AAAA-MM-DD)")
	ErrDataFechamentoInvalida    = kernel.NovoErro(kernel.KindInvalido, "data_fechamento inválida (use o formato AAAA-MM-DD)")
	ErrMotivoPerdaObrigatorio    = kernel.NovoErro(kernel.KindInvalido, "motivo_perda é obrigatório quando etapa = \"Fechado perdido\"")
)

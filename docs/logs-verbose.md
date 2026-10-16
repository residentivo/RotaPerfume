# Logs verbose de rastreamento (`VERBOSE`)

> Card **LOG-02** (Lote 17, 2026-10-07), revisto no **ARQ-08** (Lote 21, 2026-10-16) para o modelo de **4 níveis** aprovado pelo usuário (decisão **P1** do ARQ-00; detalhe em `docs/arquitetura-ddd.md` §6). Arquivos envolvidos: `apis/shared/vlog/vlog.go` (formato, `MaskEmail`, `OcultarEmail` e a regra resumida no comentário do pacote), `apis/shared/vlog/trace.go` (`vlog.Trace`, nível 2), `apis/rotaperfumes-api/internal/httpx/logging.go` (`httpx.Trace`, nível 1, e o log de acesso `[http]`), `apis/shared/config/config.go` (`cfg.Verbose`), `frontend/src/lib/vlog.ts`, `frontend/.env.local.example` e os testes em `apis/shared/tests/vlog/` e `apis/rotaperfumes-api/tests/arquitetura/vlog_test.go`.
>
> **O que mudou no Lote 21:** a regra antiga ("linha a linha": um log antes de cada passo) foi substituída por 4 níveis no backend (rota, caso de uso, decisão e query), de cerca de 3.700 para cerca de 600 chamadas. O formato da linha e as proibições de conteúdo **não mudaram**, e o TST-05 continua verde. O frontend não mudou.

## 1. O que é

Um log de rastreamento para desenvolvimento. Quando está ligado, a API, os importadores, os tools e o frontend emitem linhas que dizem **o que** o código está fazendo, sem expor **o valor** dos dados. Desligado (o padrão), não custa nada: a flag é checada antes de qualquer formatação.

No backend, os logs seguem 4 níveis (seção 2). No frontend continua valendo o rastreamento por passo relevante (seção 7).

## 2. Formato e níveis (backend)

```
[arquivo] [Funcao] descrição
```

| Nível | Quem emite | Exemplo |
|---|---|---|
| 1. Rota | automático: o `httpx.Router` envolve todo handler com `httpx.Trace` (`internal/httpx/logging.go`). Fica **dentro** da autenticação, então 401/403 não emitem o rótulo do handler. | `[produto_handler.go] [ProdutoHandler.ListProdutos] início GET` / `fim status=200` |
| 2. Caso de uso | `defer vlog.Trace(...)(&err)` na 1ª linha de cada método dos pacotes `<contexto>app` (e do `importkit.Executar`). O fim registra a duração e, em erro, só o `Kind` (`nao_encontrado`, `invalido`, `interno`...), nunca `err.Error()`. | `[produtos.go] [Catalogo.ListarProdutos] início page=1 limit=20 busca=false ordem=sku` / `fim ok dur=3ms` |
| 3. Decisão | uma linha `vlog.Printf` em cada ramo com significado de negócio ou de segurança. | `[produtos.go] [Catalogo.AlternarAtivoProduto] decisão ativo=false id=7` |
| 4. Query | uma linha por operação de repositório (adapters `<contexto>mysql`): nome da operação e contagens. | `[produtos.go] [Produtos.Listar] SELECT produtos linhas=20 total=37` |

Fora do `vlog`, continuam **sempre ligados** (não dependem de `VERBOSE`) os logs operacionais com `log.Printf`: `[http] MÉTODO caminho -> status (duração)`, `[auth] acesso negado ...`, `[escopo] bloqueado vendedor_desligado ... rota=<path>`, `[server] ...`. Neles o caminho passa por `kernel.SanearRota` (`apis/shared/kernel/rota.go`): caracteres de controle e separadores de linha vêm escapados (`\n`), para um path decodificado não forjar linhas no log (ARQ-07, NB-C).

## 3. Como ligar

### 3.1 Backend (API, CLIs de import e tools)

1. No `.env` da raiz, defina `VERBOSE=true`. `LOG_LEVEL=debug` também liga.
2. Reinicie o processo (`make dev-api`).
3. No startup, o server emite um **WARN** avisando que o modo verbose está ativo (`apis/rotaperfumes-api/cmd/server/main.go`).
4. Os CLIs (`apis/shared/cmd/*`, `apis/rotaperfumes-api/cmd/resetsenhas`) também respeitam a flag: o `main` de cada um é fino e chama `cmdutil.Executar`, e a lógica do comando carrega a config por `cmdutil.Configurar` (`apis/shared/cmdutil/cli.go`), que chama `vlog.SetEnabled(cfg.Verbose)`.

Para desligar, remova a variável (ou use `VERBOSE=false`) e reinicie.

### 3.2 Frontend (Next.js)

1. No `frontend/.env.local`, defina `NEXT_PUBLIC_VERBOSE=true` (veja `frontend/.env.local.example`).
2. A variável é **embutida no bundle em tempo de build**. Em `make dev-frontend` basta reiniciar. Em build de produção é preciso **rebuildar** para ligar ou desligar.
3. As linhas aparecem no console do navegador (DevTools).

## 4. Aviso: NÃO usar em produção

- **Volume e disco:** mesmo com os 4 níveis, cada requisição gera várias linhas (rota, caso de uso, queries), o que enche o log e o disco.
- **Amplificação em ataque:** num flood de tentativas de login, cada requisição gera várias linhas, multiplicando o custo do ataque para o servidor.
- **Frontend:** com `NEXT_PUBLIC_VERBOSE=true` no build, o rastreamento fica visível no DevTools de qualquer usuário. Builds de produção devem sair **sem** a variável ou com `false`.

Use só em desenvolvimento ou numa investigação pontual, e desligue em seguida.

## 5. O que pode e o que nunca é logado

**Regra geral:** descreva o que o código faz, não o valor dos dados.

### 5.1 Por nível (backend)

| Nível | Pode | Nunca |
|---|---|---|
| 1. Rota | método HTTP, status, rótulo `[arquivo] [Funcao]` | path com query string, headers, cookies, corpo |
| 2. Caso de uso | IDs numéricos, contagens, `page`/`limit`, enums (`periodo=mes`, `ordem=sku`), booleanos, `Kind` do erro | `%v`/`%+v`/`%#v` de comando, entidade, DTO ou filtro; textos digitados (nome, busca, razão social) no máximo como `len()` ou `busca=sim/não` |
| 3. Decisão | IDs numéricos, papel, booleanos, `retry` em segundos, `token_id` (id da linha, não o token), e-mail **só** via `vlog.MaskEmail`/`Email.Mascarado()` | a **chave** do rate limit; token Turnstile, refresh token, hash, JWT, senha (**nem mascarados**); CNPJ; IP e User-Agent |
| 4. Query | **só** o nome da operação (`SELECT produtos`, `UPDATE refresh_tokens revogar`) e contagens (`linhas=20 total=37 afetadas=1`) | texto do SQL, args, placeholders, valores de filtro |

Uma quantidade numérica não sensível digitada pelo usuário (ex.: `saldo=%d` no ajuste de estoque) conta como contagem e pode ir para o nível 2. No log operacional (`log.Printf`), texto livre (ex.: SKU) vai com `%q`, para que um CR/LF não forje linhas.

### 5.2 Lista geral

**Permitido:** IDs numéricos (`usuario_id`, `cliente_id`, `pedido_id`...), contagens/`len()`, booleanos, status HTTP, papel (`admin`/`normal`), durações e e-mail **mascarado** com `vlog.MaskEmail` (ex.: `a***@empresa.com`).

**Nunca logado (backend):**
- `%v`/`%+v`/`%#v` de struct, map ou slice de entidade, comando, DTO ou filtro. Tag `json:"-"` **não** protege.
- `r.Body`, request decodificado, `r.Header`, `r.Cookies()`, query string inteira.
- Configuração sensível: senha e usuário do banco, `cfg.DSN()` (só via `maskDSN` em `apis/shared/db/db.go`), `JWT_SECRET`, pepper do hash, usuário e senha SMTP, secret do Turnstile, e-mails de alerta de segurança, o `*Config` inteiro.
- Senhas em claro (login, reset, senha inicial, senhas geradas) e hashes Argon2id (atuais e do histórico). O `seedusers` só imprime o hash completo com `-show-password`, e o `resetsenhas` mascara os e-mails na listagem (ARQ-07, NB-7).
- Tokens: JWT, refresh token (em claro e hash), token do captcha, resposta crua da Cloudflare, cookies `access_token`/`refresh_token`, header `Authorization`. Das claims JWT, só `usuario_id` e `papel`.
- PII: CNPJ nunca; IP e User-Agent nunca em verbose; e-mail só via `vlog.MaskEmail`; strings digitadas pelo usuário (nome, razão social, busca, filtros) no máximo como `len()`; conteúdo de planilhas dos importers/exportação nunca.
- SQL: só o nome da operação (ex.: "UPDATE usuarios por id"), nunca a query com valores nem os args.
- Erros: no nível 2 só o `Kind`. Erro do driver SMTP pode repetir o endereço do destinatário: antes de logar, passe por `vlog.OcultarEmail(err, emails...)`, que troca cada e-mail por `[email-usuario]` (ignora maiúsculas e espaços nas pontas; ARQ-07, NB-1/NB-A/NB-B).

**Nunca logado (frontend):** senha, senha atual/nova, token do captcha, `access_token`/`refresh_token`, o objeto `User` inteiro, e-mail ou CNPJ de formulários, corpo de request/response no `apiClient` (só método, rota sem query e status), estado de formulário inteiro.

## 6. Proibido e exceções

- **Proibido no backend:** vlog "de sintaxe" (uma linha por `if`, atribuição ou declaração, como no modelo antigo), `if cfg.Verbose { log.Printf(...) }` (vira nível 3) e vlog no `kernel` ou no domínio (`apis/shared/<contexto>`, que são puros; quem loga é a aplicação e a infraestrutura).
- **Interior de loops** (`for rows.Next()`, linhas de planilha nos importers, lotes do cleanup de `refresh_tokens`): loga-se **antes e depois**, com a contagem. O `importkit` não loga por linha.
- `return` e respostas da API não têm log próprio (o nível 1 registra o status).
- No frontend: formatadores chamados por linha não logam.

## 7. Como adicionar log em código novo

**Go (caso de uso, nível 2), com retorno nomeado `err`:**

```go
// apis/shared/catalogo/catalogoapp/produtos.go
func (c *Casos) ObterProduto(ctx context.Context, id int64) (p *catalogo.Produto, err error) {
	defer vlog.Trace(arqProdutos, "Catalogo.ObterProduto", "id=%d", id)(&err)
	return c.produtos.PorID(ctx, id)
}
```

**Go (decisão, nível 3, e query, nível 4):**

```go
// decisão (catalogoapp/produtos.go)
vlog.Printf(arqProdutos, "Catalogo.AlternarAtivoProduto", "decisão ativo=%t id=%d", p.Ativo, id)
// query (catalogomysql/produtos.go)
vlog.Printf(arqProdutos, "Produtos.Listar", "SELECT produtos linhas=%d total=%d", len(itens), total)
// e-mail, quando indispensável, só mascarado
vlog.Printf(arq, "Funcao", "decisão ... email=%s", vlog.MaskEmail(email))
```

- 1º argumento: nome do arquivo; 2º: `Tipo.Metodo` (ou só `Funcao`); depois o formato e os args.
- Handlers HTTP **não** precisam de vlog: o nível 1 é automático pelo router.
- CLIs novos: use `cmdutil.Executar` no `main` e `cmdutil.Configurar` na lógica, que já liga o vlog.

**Frontend:**

```ts
import { vlog } from "@/lib/vlog";

const F = "MeuComponente.tsx";
vlog(F, "MeuComponente.handleSalvar", "salvando pedido itens=" + itens.length);
```

Antes de abrir PR, confira as chamadas novas contra a seção 5. Em caso de dúvida, logue só a contagem ou o booleano.

## 8. Testes

- `apis/shared/tests/vlog/vlog_test.go`: liga/desliga e formato da linha.
- `apis/shared/tests/vlog/trace_test.go`: `vlog.Trace` (nível 2).
- `apis/shared/tests/vlog/vlog_runtime_test.go`: comportamento em runtime com a flag ligada e `MaskEmail`.
- `apis/shared/tests/vlog/arq07_ocultar_email_test.go`: `OcultarEmail` (`[email-usuario]`, caixa e espaços).
- `apis/rotaperfumes-api/tests/arquitetura/vlog_test.go` (`TestARQ_VlogSemConteudoSensivel`): teste de AST nos dois módulos; rejeita `%v`/`%+v`/`%#v` no formato e argumentos com nome sensível (`senha`, `token`, `cnpj`, `email`, `hash`...) sem `MaskEmail`/`Mascarado()`. Tem controle negativo (`_DetectaViolacoes`).
- `apis/rotaperfumes-api/tests/handlers/tst05_vlog_runtime_auth_test.go` (TST-05, Lote 20): com `VERBOSE` ligado, login e reset de senha não vazam senha, hash, token, cookie, captcha, User-Agent nem e-mail sem máscara.
- Rodar: `make test` (ou só a arquitetura com `make arch-check`).

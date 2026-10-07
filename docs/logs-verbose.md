# Logs verbose de rastreamento (`VERBOSE`)

> Card **LOG-02** (Lote 17, 2026-10-07). Arquivos envolvidos: `apis/shared/vlog/vlog.go`, `frontend/src/lib/vlog.ts`, `apis/shared/config/config.go` (`cfg.Verbose`), `frontend/.env.local.example` e os testes em `apis/shared/tests/vlog/`.

## 1. O que é

Um log de rastreamento "linha a linha". Quando está ligado, cada passo relevante do código (API, importers, tools e frontend) emite uma linha que diz **o que** o código está fazendo, sem expor **o valor** dos dados. Desligado (o padrão), não custa nada: a flag é checada antes de qualquer formatação.

## 2. Formato

```
[arquivo] [Funcao] descrição
```

Exemplos:

```
[usuario_service.go] [UsuarioService.ResetSenha] buscando usuário por id=42
[auth_handler.go] [AuthHandler.Login] login ok usuario_id=42 papel=vendedor
[apiClient.ts] [apiFetch] GET /api/clientes status=200
```

## 3. Como ligar

### 3.1 Backend (API, CLIs de import e tools)

1. No `.env` da API, defina `VERBOSE=true`. `LOG_LEVEL=debug` também liga.
2. Reinicie o processo (`make dev-api`).
3. No startup, o server emite um **WARN** avisando que o modo verbose está ativo.
4. Os CLIs de import (`apis/shared/importers/*`, `apis/shared/cmd/*`) e os tools (`apis/shared/tools/*`) também respeitam a flag, porque todos os mains chamam `vlog.SetEnabled`.

Para desligar, remova a variável (ou use `VERBOSE=false`) e reinicie.

### 3.2 Frontend (Next.js)

1. No `frontend/.env.local`, defina `NEXT_PUBLIC_VERBOSE=true` (veja `frontend/.env.local.example`).
2. A variável é **embutida no bundle em tempo de build**. Em `make dev-frontend` basta reiniciar. Em build de produção é preciso **rebuildar** para ligar ou desligar.
3. As linhas aparecem no console do navegador (DevTools).

## 4. Aviso: NÃO usar em produção

- **Volume e disco:** milhares de linhas por requisição enchem o log e o disco.
- **Amplificação em ataque:** num flood de tentativas de login, cada requisição gera várias linhas, multiplicando o custo do ataque para o servidor.
- **Frontend:** com `NEXT_PUBLIC_VERBOSE=true` no build, o rastreamento fica visível no DevTools de qualquer usuário. Builds de produção devem sair **sem** a variável ou com `false`.

Use só em desenvolvimento ou numa investigação pontual, e desligue em seguida.

## 5. O que pode e o que nunca é logado

**Regra geral:** descreva o que o código faz, não o valor dos dados.

**Permitido:** IDs numéricos (`usuario_id`, `cliente_id`, `pedido_id`...), contagens/`len()`, booleanos, status HTTP, papel (`admin`/`vendedor`), durações e e-mail **mascarado** com `vlog.MaskEmail` (ex.: `a***@empresa.com`).

**Nunca logado (backend):**
- `%v`/`%+v`/`%#v` de struct, map ou slice de modelo. `json:"-"` **não** protege (ex.: `models.Usuario.PasswordHash`).
- `r.Body`, request decodificado, `r.Header`, `r.Cookies()`, query string inteira.
- Configuração sensível: senha e usuário do banco, `cfg.DSN()` (só via `maskDSN` em `shared/db/db.go`), `JWT_SECRET`, pepper do hash, usuário e senha SMTP, secret do Turnstile, e-mails de alerta de segurança, o `*Config` inteiro.
- Senhas em claro (login, reset, senha inicial, senhas geradas) e hashes Argon2id (atuais e do histórico).
- Tokens: JWT, refresh token (em claro e hash), token do captcha, resposta crua da Cloudflare, cookies `access_token`/`refresh_token`, header `Authorization`. Das claims JWT, só `usuario_id` e `papel`.
- PII: CNPJ nunca; IP e User-Agent nunca em verbose; e-mail só via `vlog.MaskEmail`; strings digitadas pelo usuário (nome, razão social, busca, filtros) no máximo como `len()`; conteúdo de planilhas dos importers/exportação nunca.
- SQL: só o nome da operação (ex.: "UPDATE usuarios por id"), nunca a query com valores nem os args.
- Erros: só `err.Error()` de erros próprios ou do driver, nunca de erro que embrulhe input do usuário.

**Nunca logado (frontend):** senha, senha atual/nova, token do captcha, `access_token`/`refresh_token`, o objeto `User` inteiro, e-mail ou CNPJ de formulários, corpo de request/response no `apiClient` (só método, rota sem query e status), estado de formulário inteiro.

## 6. Exceções (linhas sem log)

- `return` e respostas da API.
- Linhas que já são log.
- Interior de loops (`for rows.Next()`, loops de importers/planilhas, cleanup de `refresh_tokens`): loga-se **antes e depois** do loop, com a contagem.
- Funções chamadas por linha nos importers (pacote `cnpj`, `ParseRow` etc.).
- Formatadores chamados por linha no frontend.

## 7. Como adicionar log em código novo

**Go:**

```go
import "github.com/rotaperfumes/shared/vlog"

vlog.Printf("pedido_service.go", "PedidoService.Criar", "pedido criado id=%d itens=%d", pedido.ID, len(itens))
vlog.Printf("auth_handler.go", "AuthHandler.Login", "tentativa de login email=%s", vlog.MaskEmail(req.Email))
```

- 1º argumento: nome do arquivo; 2º: `Tipo.Metodo` (ou só `Funcao`); depois o formato e os args.
- Em mains novos, chame `vlog.SetEnabled(cfg.Verbose)` no startup.

**Frontend:**

```ts
import { vlog } from "@/lib/vlog";

const F = "MeuComponente.tsx";
vlog(F, "MeuComponente.handleSalvar", "salvando pedido itens=" + itens.length);
```

Antes de abrir PR, confira as chamadas novas contra a seção 5. Em caso de dúvida, logue só a contagem ou o booleano.

## 8. Testes

- `apis/shared/tests/vlog/vlog_test.go`: liga/desliga, formato da linha e `MaskEmail`.
- `apis/shared/tests/vlog/vlog_runtime_test.go`: comportamento em runtime com a flag ligada.
- Rodar: `make test`.
- Pendência: o teste de runtime com `VERBOSE` ligado nos handlers de login/reset depende de mock HTTP/DB (card **TST-05** em `tarefas/afazer.md`).

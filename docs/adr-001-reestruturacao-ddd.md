# ADR-001: reestruturação da API em DDD + Clean Code + Design Patterns

- **Status:** aceito e implementado.
- **Data da decisão:** 2026-10-13 (ARQ-00). **Implementação:** ARQ-01 a ARQ-07, de 2026-10-13 a 2026-10-16. **Registro:** ARQ-08 (🔵 SubBrain, 2026-10-16).
- **Decisores:** usuário (pedido, estratégia incremental, P1 e X1); 🤍 MegaBrain (P3, P4, R-a, portões de cobertura); 🟡 BackBrain (desenho); 🟣 SecBrain (invariantes O1 a O9, R-a a R-g); 🔴 TestBrain (matriz de rotas e linha de base).
- **Branch:** `refactor/ddd`, um commit por card: ARQ-00 `cd12d47`, ARQ-01 `0e7e757`, ARQ-02 `17502da`, ARQ-03 `c412d10`, ARQ-04 `fdaa7d9`, ARQ-05 `3e33f6f`, ARQ-06 `adce2b8`, ARQ-07 `719393c`, ARQ-08 `5228747`.
- **Documentos relacionados:** `docs/arquitetura-ddd.md` (blueprint e, na seção 10, o estado final), `docs/arq-00-invariantes-seguranca.md` (invariantes de segurança), `docs/logs-verbose.md` (regra de logs em 4 níveis).

## Contexto

Pedido do usuário: "revise todo o código da API e adicione Design Patterns em todos os locais possíveis para simplificar o código. Use o padrão DDD e Clean Code para reestruturar o código da API".

O diagnóstico do ARQ-00 (`docs/arquitetura-ddd.md` §1) encontrou, entre outros:

- `*sql.DB` passado em todo método; repositórios como `struct{}` vazias; construtores que ignoravam dependências; handlers montando os próprios services; injeção por setter;
- regra de escopo por carteira (segurança) espalhada em handlers e baseada numa sentinela numérica (`vendedorID <= 0` = acesso global);
- filtros, paginação e mapeamento de erro→HTTP duplicados por contexto; `models` anêmicos usados ao mesmo tempo como entidade, DTO e linha de banco;
- rotas protegidas por booleanos posicionais (`jwt(true, false)`) e 13 padrões aceitando qualquer método;
- cerca de 3.700 chamadas de `vlog` "linha a linha", quase metade das linhas de produção;
- transações abertas dentro de repositórios e `main` com toda a montagem.

## Decisão

1. **Bounded contexts** em `apis/shared/<contexto>/`: Catálogo, Identidade e Acesso, Comercial, Vendas, Analytics e Notificação. Cada contexto tem três camadas em pacotes de nome único: domínio (`<ctx>`), aplicação (`<ctx>app`, casos de uso + portas) e adapters (`<ctx>mysql`; também `identidadecrypto` e `notificacaosmtp`).
2. **Shared kernel** `apis/shared/kernel` (puro, só stdlib): `Principal`, `Role`, `Escopo` (zero value inválido; tradução para SQL só por `Filtro()`), `Pagina`/`Resultado[T]`, `Ordenacao[C]`, `Data`, `Email`, `Dinheiro`, erros tipados por `Kind` e a rota do request para log (`rota.go`, com `SanearRota`).
3. **Camada HTTP só na API**, em `apis/rotaperfumes-api/internal/`: `httpx` (kit transversal: router com política tipada e método obrigatório, cadeia de middlewares, autenticação, envelope, mapper de erros), `<ctx>http` (handlers finos) e `bootstrap` (composition root).
4. **Dois módulos Go mantidos** (`shared` e `rotaperfumes-api`, com `replace ../shared`); Dockerfile, Makefile e Jenkinsfile sem mudança de comando.
5. **Migração incremental (strangler fig)** por contexto, um commit por card, cada um com build, vet e testes verdes, sem perda de testes (R1) e com a matriz de 60 rotas verde (R2). O legado (`middleware`, `services`, `routes`, `handlers`, `repositories`, `models`) foi removido por completo no ARQ-07.
6. **Design Patterns aplicados onde simplificam**, com a lista do que não aplicar (`docs/arquitetura-ddd.md` §5.19). A tabela "pattern → onde no código" está em `docs/arquitetura-ddd.md` §10.10.
7. **Logs `vlog` em 4 níveis** (rota automática, caso de uso por `defer vlog.Trace`, decisão e uma linha por query), substituindo a regra "linha a linha" do LOG-02 (decisão P1 do usuário). Formato da linha e proibições de conteúdo mantidos.
8. **Regras de camada verificadas por teste** (`apis/shared/tests/arquitetura`, `apis/rotaperfumes-api/tests/arquitetura`): dependências entre camadas e contextos, allowlists de chamadores (`NovoPrincipal`, `Escopo*`), AST de erros, de `vlog` e de SQL montado.

## Alternativas consideradas e rejeitadas

| Alternativa | Por que não |
|---|---|
| Reescrita única (big bang) | Risco alto num sistema com regras de segurança finas (escopo, sessão, rate limit). O strangler permitiu validar cada contexto com os testes existentes. |
| Aplicação dentro do módulo da API | Os CLIs de `shared` (`resetsenhas`, `seedusers`, `resetpassword`) precisam das mesmas regras de Identidade; `shared` não pode importar a API. |
| `shared/internal/...` | O Go não deixa o módulo da API importar `internal` de outro módulo. A disciplina de camadas fica com os testes de arquitetura. |
| Fundir os dois módulos | Ortogonal ao DDD; mexeria em Dockerfile, Makefile, Jenkinsfile e na contagem de testes por suíte. Fica como opção futura, com card próprio. |
| Framework de DI ou de logging | Nenhuma dependência nova; construtores explícitos e `bootstrap` bastam. |
| Decorator de interface por caso de uso (para log) | Exigiria cerca de 70 interfaces e wrappers; `defer vlog.Trace` tem o mesmo efeito numa linha. |
| `Dinheiro` em centavos já no Lote 21 | Risco de divergência de arredondamento nos totais gravados. Ficou `float64` encapsulado (P3); a troca é o card ARQ-09. |

## Consequências

**Positivas**

- Dependências explícitas e testáveis; o `main` da API e os 13 `main` dos CLIs viraram composition roots finos.
- A regra de escopo por carteira ficou num ponto só (`comercialapp.AutorizadorEscopo` + `kernel.Escopo.Filtro()`), fechando os riscos críticos E-02 (sentinela 0 = global) e O1 (SemAcesso virando consulta global).
- Rotas declaradas numa tabela única (`internal/bootstrap/rotas.go`) com método e política obrigatórios; a matriz de rotas compara contra o registro em runtime.
- Cobertura combinada API+shared de 88,48% (linha de base) para 98,06% no ARQ-07; statements descobertos de cerca de 1.300 para 133.
- Testes de arquitetura impedem regressão ao legado e o uso fora das allowlists.

**Negativas / custos**

- Mais pacotes e arquivos (três por contexto, mais o HTTP). A navegação depende da convenção `<ctx>`, `<ctx>app`, `<ctx>mysql`, `<ctx>http`.
- Única mudança de contrato HTTP: a **X1** (aprovada pelo usuário): método fixo nas 13 rotas que não tinham (outro método → 405) e `GET /api/senha-historico/{user_id...}` com parse estrito (`/2/extra`, `/0` e `/-1` → 400). Detalhe em `postman/README.md`, seção "Mudanças do Lote 21".
- Os testes mantiveram pastas e nomes antigos (`tests/services`, `tests/repositories`, `TestProdutoService_*`) por causa da regra R1; os nomes não refletem mais os pacotes de produção.
- Ressalva N-7 (aceita, só documentada): `Novo`/construtores que conferem `dep == nil` não detectam uma interface não nula guardando ponteiro nil tipado. A proteção é o `bootstrap` sempre montar adapters reais.

## Pendências derivadas (fora do Lote 21, em `tarefas/afazer.md`)

ARQ-09 (Dinheiro em centavos), BUG-14 (`VincularCliente` sem transação), BUG-15 (troca de senha sem transação), BUG-16, BUG-17, BUG-18, SEC-14 a SEC-28, TST-06, TST-07, CHORE-03 a CHORE-06.

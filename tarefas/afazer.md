# A Fazer

> Atualizado em 2026-10-07. O 🔴 TestBrain executou `docs/roteiro-teste-manual-lote12.md` (24 aprovados, 1 falhou, 13 não verificáveis de 38; ver a seção "Execução (2026-10-07, 🔴 TestBrain)" no fim do roteiro). Daí saíram os cards BUG-13, LOG-01, DOC-05 e TST-04. BUG-13, LOG-01 e DOC-05 foram concluídos no Lote 16 (2026-10-07; ver `feito.md`). O TST-04 segue **bloqueado** até o usuário liberar `make db-*` ou rodar os alvos. O DEPLOY-01 (Lote 13) segue em `fazendo.md`, aguardando o usuário. O LOG-02 (Lote 17, 2026-10-07) foi aberto direto em `fazendo.md` e concluído no mesmo dia (ver `feito.md` e `docs/logs-verbose.md`); dele saiu o card TST-05 (prioridade BAIXA). Em 2026-10-11 o item 13 do `FalhasEncontradas.md` foi fechado pelo card DEP-00 (Lote 19, ver `feito.md`); da primeira execução de `make deps-audit` saíram os cards DEP-01 e DEP-02 (prioridade BAIXA).

---

## TST-04: itens do roteiro do Lote 12 não verificados (`make db-*` bloqueado) — prioridade MÉDIA

**Status:** pendente, **bloqueado** (depende do usuário)
**Camada:** Testes
**Responsável:** 🔴 TestBrain com o usuário
**Origem:** 🔴 TestBrain, roteiro manual do Lote 12 (2026-10-07).

**Descrição:** o controle de permissões do agente bloqueou a execução de `make db-*`, então estes itens não foram verificados: 1.1, 1.2 e 1.7 (SEC-11, `db-check-env` e precedência), 3.1 a 3.7 (DB-01, `estoque` no `db-up` e migração 18 com reversão) e 4.1 a 4.3 (migração 23 idempotente). Houve só uma revisão estática (ordem 16 → 17 no `db-up`, `17` sem `origem`, `06` com `reuso_detectado_em`, `18`/`23` com `information_schema` + `PREPARE`), que não substitui a execução. O banco descartável `rotaperfumes_l12` nunca foi criado.

**Ação esperada:** o usuário roda os alvos ou libera a permissão para o 🔴 TestBrain. Regras:
- usar **sempre** `DB_NAME=rotaperfumes_l12`, conferindo antes com `make -n` que o banco alvo é o descartável;
- **nunca** rodar no banco `rotaperfumes`;
- apagar o `rotaperfumes_l12` ao final.

Registrar o resultado na seção "Execução" do roteiro.

---

## TST-05: teste de runtime do vlog com `VERBOSE` ligado nos handlers de login/reset — prioridade BAIXA

**Status:** pendente
**Camada:** Testes
**Responsável:** 🔴 TestBrain
**Origem:** pendência do LOG-02 (Lote 17, 2026-10-07; ver `feito.md`).

**Descrição:** a spec de segurança do LOG-02 pede um teste com `VERBOSE=true` nos fluxos de login e reset de senha que confirme que nenhum valor sensível (senha, hash, token, captcha, cookie, e-mail sem máscara) vai ao log. A auditoria AST e os testes em `apis/shared/tests/vlog/` cobrem o pacote e as chamadas estaticamente, mas o teste de runtime nos handlers depende de mock HTTP/DB.

**Ação esperada:** montar o mock HTTP/DB, ligar `vlog.SetEnabled(true)`, capturar a saída do `log` durante login e reset (sucesso e falha) e verificar que os valores sensíveis usados no teste não aparecem e que o e-mail sai só como `vlog.MaskEmail`. Referência: `docs/logs-verbose.md`, seção 5.

---

## DEP-01: migrar Tailwind 3→4 (elimina vulns de dev braces/postcss-selector-parser) — prioridade BAIXA

**Status:** pendente
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain
**Origem:** residual do item 13 do `FalhasEncontradas.md` (Lote 19, 2026-10-11; ver `feito.md`).

**Descrição:** após o `npm audit fix` do Lote 19, restam 9 vulnerabilidades só em dependências de desenvolvimento: 7 altas (`braces`, sem versão corrigida, e dependentes, incluindo tailwindcss 3) e 2 moderadas (`postcss-selector-parser`/`postcss-nested` via tailwind 3). Produção está limpa (`npm audit --omit=dev` = 0), mas essas vulns só saem com o upgrade major para Tailwind 4, que não pode ser feito via `npm audit fix --force` sem card próprio (política do `CLAUDE.md`).

**Ação esperada:** migrar para Tailwind 4 (nova configuração/plugin PostCSS, revisão de classes e do `tailwind.config`), conferir visualmente as telas (incluindo o menu mobile do UX-01) e validar com `tsc`, `eslint`, vitest, `next build` e `npm audit` (dev) sem as vulns listadas.

---

## DEP-02: atualizar módulos Go desatualizados — prioridade BAIXA

**Status:** pendente
**Camada:** Backend
**Responsável:** 🟡 BackBrain
**Origem:** saída do `go list -m -u all` no `make deps-audit-go` (Lote 19, 2026-10-11; ver `feito.md`).

**Descrição:** módulos com versões novas disponíveis, sem CVE apontado pelo govulncheck: `golang.org/x/crypto` 0.24 → 0.58, `golang.org/x/net`, `golang.org/x/sys`, `golang.org/x/text`, `github.com/go-sql-driver/mysql` 1.8.1 → 1.10.1 e `github.com/stretchr/testify`.

**Ação esperada:** atualizar nos dois módulos (`apis/shared`, `apis/rotaperfumes-api`), mantendo as mesmas versões entre eles; revisar changelogs (atenção ao driver MySQL e ao `x/crypto`, usado no hash de senha); rodar `go build`, `go vet`, `go test ./...` e `make deps-audit-go`.

# A Fazer

> Atualizado em 2026-10-11. O 🔴 TestBrain executou `docs/roteiro-teste-manual-lote12.md` (24 aprovados, 1 falhou, 13 não verificáveis de 38; ver a seção "Execução (2026-10-07, 🔴 TestBrain)" no fim do roteiro). Daí saíram os cards BUG-13, LOG-01, DOC-05 e TST-04. BUG-13, LOG-01 e DOC-05 foram concluídos no Lote 16 (2026-10-07; ver `feito.md`). O LOG-02 (Lote 17) gerou o TST-05, e o DEP-00 (Lote 19) gerou o DEP-01 e o DEP-02. No Lote 20 (2026-10-11; ver `feito.md`) foram concluídos DEP-01, DEP-02 e TST-05; o TST-04 avançou parcialmente (só faltam 1.1, 1.2 e 1.7, que dependem do usuário rodar o `make`) e do DEP-01 saiu o card DEP-03 (BAIXA). O MAIL-01 e o DEPLOY-01 seguem em `fazendo.md`, aguardando o usuário.

---

## TST-04: itens do roteiro do Lote 12 que testam o próprio `make` (1.1, 1.2 e 1.7) — prioridade MÉDIA

**Status:** pendente, **bloqueado** (depende do usuário); avançou parcialmente no Lote 20 (2026-10-11)
**Camada:** Testes
**Responsável:** 🔴 TestBrain com o usuário
**Origem:** 🔴 TestBrain, roteiro manual do Lote 12 (2026-10-07).

**Já feito (Lote 20, 2026-10-11):** execução equivalente sem `make` no banco descartável `rotaperfumes_l12` (criado e depois apagado; o banco `rotaperfumes` ficou intacto, com o mesmo MD5 de schema e `estoque` com 8401 linhas). Aprovados: 3.1, 3.2, 3.3, 3.5, 3.6, 3.7, 4.1, 4.2 e 4.3. O 3.4 não se aplica (só com `db-rebuild`). Resultado em `docs/roteiro-teste-manual-lote12.md`, seção "Execução complementar do TST-04".

**Falta:** 1.1, 1.2 e 1.7 (SEC-11: `db-check-env` e precedência de variáveis). Eles testam o próprio `make`, então não há execução equivalente: continuam bloqueados até o usuário rodá-los com o `make` (ou liberar a permissão de `make db-*` para o 🔴 TestBrain).

**Regras:**
- usar **sempre** `DB_NAME=rotaperfumes_l12`, conferindo antes com `make -n` que o banco alvo é o descartável;
- **nunca** rodar no banco `rotaperfumes`;
- apagar o `rotaperfumes_l12` ao final.

Registrar o resultado na seção "Execução" do roteiro.

---

## DEP-03: vulnerabilidades altas de dev restantes (`braces` via `eslint-config-next`) — prioridade BAIXA

**Status:** pendente (acompanhar o upstream)
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain
**Origem:** residual do DEP-01 (Lote 20, 2026-10-11; ver `feito.md`).

**Descrição:** após a migração para Tailwind 4, o `npm audit` completo caiu de 9 para 5 vulnerabilidades (5 altas); `npm audit --omit=dev` = 0. As 5 vêm da cadeia `braces` → `micromatch` → `fast-glob@3.3.1` → `@next/eslint-plugin-next` → `eslint-config-next@16.4.0`, que só é usada pela ferramenta de lint (dev). O `braces` não tem versão corrigida, e o `npm audit fix --force` rebaixaria o `eslint-config-next` para 14.2.35 (rejeitado).

**Ação esperada:** acompanhar o upstream (`eslint-config-next`/`@next/eslint-plugin-next`, `fast-glob`, `micromatch`, `braces`). Quando houver correção, atualizar ou usar `overrides` de `fast-glob`/`micromatch` no `package.json` e validar com `tsc`, `eslint`, vitest, `next build` e `npm audit`. Não usar `npm audit fix --force`.

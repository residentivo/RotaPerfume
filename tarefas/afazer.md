# A Fazer

> Atualizado em 2026-10-07. O 🔴 TestBrain executou `docs/roteiro-teste-manual-lote12.md` (24 aprovados, 1 falhou, 13 não verificáveis de 38; ver a seção "Execução (2026-10-07, 🔴 TestBrain)" no fim do roteiro). Daí saíram os cards BUG-13, LOG-01, DOC-05 e TST-04. BUG-13, LOG-01 e DOC-05 foram concluídos no Lote 16 (2026-10-07; ver `feito.md`). Resta só o TST-04, **bloqueado** até o usuário liberar `make db-*` ou rodar os alvos. O DEPLOY-01 (Lote 13) segue em `fazendo.md`, aguardando o usuário.

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

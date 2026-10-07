# A Fazer

> Atualizado em 2026-10-07. O 🔴 TestBrain executou `docs/roteiro-teste-manual-lote12.md` (24 aprovados, 1 falhou, 13 não verificáveis de 38; ver a seção "Execução (2026-10-07, 🔴 TestBrain)" no fim do roteiro). Daí saíram os cards BUG-13, LOG-01, DOC-05 e TST-04. BUG-13, LOG-01 e DOC-05 foram concluídos no Lote 16 (2026-10-07; ver `feito.md`). O TST-04 segue **bloqueado** até o usuário liberar `make db-*` ou rodar os alvos. O DEPLOY-01 (Lote 13) segue em `fazendo.md`, aguardando o usuário. O LOG-02 (Lote 17, 2026-10-07) foi aberto direto em `fazendo.md` e concluído no mesmo dia (ver `feito.md` e `docs/logs-verbose.md`); dele saiu o card TST-05 (prioridade BAIXA).

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

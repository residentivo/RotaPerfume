# A Fazer

> Os cards CHORE-01, BUG-11 e FE-13 foram concluídos no Lote 9 (2026-09-26; ver `feito.md`). SEC-10, SEC-09 e DOC-04 aguardam a priorização do usuário; INFO-01 é só informativo. O card BUG-12 foi registrado no fechamento do Lote 9.

---

## BUG-12: `scanSenhaHistorico` lê `ip_origem` e `user_agent` como string, mas as colunas aceitam NULL — prioridade BAIXA

**Status:** não iniciado
**Camada:** Backend shared (ou Database)
**Responsável:** 🟡 BackBrain **ou** 🌸 DataBrain (testes: 🔴 TestBrain)
**Origem:** 🔴 TestBrain, Lote 9 (2026-09-26).

**Descrição:**
- Em `apis/shared/repositories/senha_historico_repository.go`, o `scanSenhaHistorico` lê `ip_origem` e `user_agent` como `string`, mas as colunas aceitam NULL no banco. Um registro com NULL derruba a listagem com `500` ("converting NULL to string is unsupported").
- Com a ordenação por `ip_origem asc` (FE-13), esse registro cairia na primeira página.
- Hoje há 0 linhas com NULL e o `Create` sempre grava string: risco latente.

**Ação esperada (escolher uma):**
- 🟡 BackBrain: ler com `sql.NullString` ou `COALESCE`.
- 🌸 DataBrain: tornar as colunas `NOT NULL` (com migração e revert).
- 🔴 TestBrain: cobrir registro com NULL na listagem (e na ordenação por `ip_origem`).

---

## SEC-10: endurecimentos sugeridos na revisão do Lote 7 — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Segurança (+ Backend shared)
**Responsável:** 🟣 SecBrain (avaliação) → 🟡 BackBrain (aplicação) → 🔴 TestBrain
**Origem:** 🟣 SecBrain, revisão dos construtores novos do Lote 7 (2026-09-26). Os três construtores foram aprovados; os itens abaixo são melhorias, não bloqueios. Item 6 acrescentado no fechamento do Lote 8.

**Descrição e ação esperada:**
1. **`NewSMTPEmailServiceWithTLSConfig`:** forçar `InsecureSkipVerify = false` e `MinVersion = tls.VersionTLS12` na config clonada, para que nenhum chamador desligue a validação por engano.
2. **`GenerateRandomPassword`:** o `alphabet[b%62]` tem viés de módulo. Trocar por `crypto/rand.Int` (ou descarte por rejeição).
3. **CLI do `resetpassword`:** a senha passada em `-password=` fica no histórico do shell. Avaliar leitura por prompt sem eco, stdin ou variável de ambiente, e ajustar os roteiros `docs/roteiro-teste-manual-lote6.md` e `docs/roteiro-teste-manual-lote8.md` (que usam `-password`).
4. **`seedusers`:** remover as constantes mortas de senha padrão e os defaults `golang/golang` do DSN.
5. **`seedusers.ReplaceInFile`:** grava hashes reais em `sql/0*_seed*.sql`, com risco de commit acidental. Avaliar gravar em arquivo temporário ou ignorado pelo git.
6. **`InvalidarSessoes` (SEC-08, Lote 8):** grava o `t` recebido sem truncar, e a coluna `DATETIME` arredonda fração ≥ .5 para o segundo seguinte. Hoje o único chamador já trunca; truncar dentro do repositório por defesa.

> **Nota (SEC-08, Lote 8):** pela regra `iat <= corte`, um login no mesmo segundo de um corte gera token recusado. **Aceito pelo 🤍 MegaBrain:** a troca de senha já redireciona para o login, e os outros casos exigem ação humana posterior. Não é item a corrigir.

---

## INFO-01: linhas não alcançáveis aceitas como fora da meta de cobertura — informativo

**Status:** registrado; nenhuma ação pendente
**Camada:** Testes
**Origem:** 🔴 TestBrain, TST-02 e TST-03 (Lote 7, 2026-09-26). Aceito pelo 🤍 MegaBrain.

**Descrição:** As linhas que restam sem cobertura no Go só são alcançáveis mudando o código de produção: erros de `crypto/rand` (`password_generator`), `os.Getwd`/`os.Executable`, `rows.Columns()`, a escrita no `DATA` do SMTP, a escrita do cabeçalho num `csv.Writer` com buffer e os ramos de `maskDSN` sem `@`/`:`. Também ficam fora da meta os `cmd/*/main.go` finos e o `cmd/server`. Não há ação a tomar, salvo se o usuário pedir 100%.

---

## SEC-09: integrar o alerta `[auth][seguranca]` a monitoramento ou notificação — prioridade BAIXA (opcional)

**Status:** não iniciado; opcional, aguarda a priorização do usuário
**Camada:** Segurança (+ Backend)
**Origem:** 🟡 BackBrain, SEC-07 (Lote 6, 2026-09-26).

**Descrição:** Com o SEC-07, o reuso de refresh token já rotacionado gera o alerta `[auth][seguranca]` (user_id, token_id, IP e UA) e revoga todas as sessões do usuário (e, desde o SEC-08, grava o corte de sessão). Hoje o alerta só vai para o log da API: ninguém é avisado.

**Ação esperada:**
- 🟣 SecBrain: definir o destino (monitoramento de logs, e-mail ao admin e/ou ao usuário) e o volume aceitável.
- 🟡 BackBrain: aplicar.

---

## DOC-04: manual completo da base de dados — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Documentação
**Origem:** 🔵 SubBrain, DOC-02 (2026-09-25).

**Descrição:** O `docs/manual-base-de-dados.md` detalha a tabela `clientes` (DOC-02, migração 19), a `refresh_tokens` (migração 21, seção 7) e a coluna `usuarios.tokens_validos_desde` (migração 22, seção 8). As demais tabelas aparecem só no índice.

**Ação esperada:**
- 🔵 SubBrain: detalhar as demais tabelas, se o usuário quiser o manual completo.

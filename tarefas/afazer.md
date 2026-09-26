# A Fazer

> Os cards BUG-10, BUG-09, SEC-08, FE-11, FE-12 e FE-10 foram concluídos no Lote 8 (2026-09-26; ver `feito.md`). SEC-10, SEC-09 e DOC-04 aguardam a priorização do usuário; INFO-01 é só informativo. Os cards BUG-11, FE-13 e CHORE-01 foram registrados no fechamento do Lote 8, que também acrescentou o item 6 ao SEC-10.

---

## BUG-11: `UsuarioRepository.Create` devolve erro com o INSERT já gravado se só a releitura falhar — prioridade BAIXA

**Status:** não iniciado
**Camada:** Backend shared
**Responsável:** 🟡 BackBrain (testes: 🔴 TestBrain)
**Origem:** 🟡 BackBrain, BUG-09 (Lote 8, 2026-09-26).

**Descrição:**
- O `UsuarioRepository.Create` (`apis/shared/repositories/usuario_repository.go`) relê o usuário com `GetByID` depois do INSERT. Se só a releitura falhar, ele devolve erro, mas o usuário já foi gravado.
- Efeito: o service responde erro, **não** envia o e-mail com a senha inicial, e uma nova tentativa bate em e-mail duplicado.

**Ação esperada:**
- 🟡 BackBrain: tirar a releitura do repositório e fazer no service, no padrão `relerXCriado` do BUG-08/BUG-09 (falha na releitura → log e objeto em memória).
- 🔴 TestBrain: cobrir a falha só da releitura (usuário criado, e-mail enviado, resposta de sucesso).

---

## FE-13: colunas de `admin/senha-historico` parecem ordenáveis, mas não reordenam — prioridade BAIXA

**Status:** não iniciado; aguarda decisão (Front ou Back)
**Camada:** Frontend + Backend
**Responsável:** 🟡 BackBrain **ou** 🟢 FrontBrain (testes: 🔴 TestBrain)
**Origem:** 🟢 FrontBrain, FE-12 (Lote 8, 2026-09-26).

**Descrição:** Em `admin/senha-historico`, as colunas Usuario, Resetado Por e IP Origem aparecem como ordenáveis, mas o `ORDER_BY_MAP` não as envia, porque estão fora da whitelist de `GET /api/senha-historico` (`id, usuario_id, tipo_reset, created_at`). O clique muda a seta, mas não a ordem.

**Ação esperada (escolher uma):**
- 🟡 BackBrain: incluir `usuario_nome`, `resetado_por_nome` e `ip_origem` na whitelist (e 🟢 FrontBrain mapeá-las no `ORDER_BY_MAP`); 🔵 SubBrain atualiza o Postman.
- 🟢 FrontBrain: tornar as três colunas não ordenáveis.
- 🔴 TestBrain: cobrir a opção escolhida.

---

## CHORE-01: `frontend/tsconfig.tsbuildinfo` rastreado pelo git — prioridade BAIXA

**Status:** não iniciado; aguarda o usuário
**Camada:** Repositório
**Origem:** 🔴 TestBrain (Lote 8, 2026-09-26).

**Descrição:** O `frontend/tsconfig.tsbuildinfo` é rastreado pelo git e é regravado a cada `tsc`, sujando o `git status`.

**Ação esperada:** `git rm --cached frontend/tsconfig.tsbuildinfo` e `*.tsbuildinfo` no `.gitignore`, se o usuário aprovar.

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

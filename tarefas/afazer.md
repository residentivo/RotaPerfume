# A Fazer

> O card BUG-12 foi concluído no Lote 10 (2026-09-26; ver `feito.md`). SEC-10, SEC-09, DOC-04 e FE-14 aguardam a priorização do usuário; INFO-01 é só informativo. O card FE-14 foi registrado no fechamento do Lote 10.

---

## FE-14: formatador de data único que trate a data zero do Go — prioridade BAIXA

**Status:** não iniciado; aguarda a priorização do usuário
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain (testes: 🔴 TestBrain)
**Origem:** 🟢 FrontBrain, BUG-12 (Lote 10, 2026-09-26).

**Descrição:** Os formatadores de data locais só tratam string vazia; a data zero do Go (`"0001-01-01T00:00:00Z"`) aparece como "31/12/1, ...". Estão em: `src/app/pagamentos/page.tsx`, `src/components/PagamentoModal.tsx`, `src/app/admin/{estoque,clientes,pedidos,visitas,oportunidades}/page.tsx` e `src/components/admin/VendedorModal.tsx` (o de `admin/senha-historico` já trata, desde o BUG-12). Risco hoje baixo: após o BUG-09 os creates devolvem timestamps reais.

**Ação esperada:**
- 🟢 FrontBrain: criar um formatador único em `src/lib` que devolva `-` para vazio e para a data zero do Go, e substituir os formatadores locais (incluindo o de `senha-historico`).
- 🔴 TestBrain: cobrir em `tests/lib`.

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

> **Nota (BUG-12, Lote 10):** em `scanSenhaHistorico` (92.3%), o ramo `sql.ErrNoRows` → `ErrNotFound` é inalcançável, porque a função só é chamada dentro de `rows.Next`.

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

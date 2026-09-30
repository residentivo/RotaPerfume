# A Fazer

> Os cards FE-14, SEC-10, SEC-09 e DOC-04 foram concluídos no Lote 11 (2026-09-26; ver `feito.md`). SEC-11, DB-01, SEC-12 e CHORE-02 foram para `fazendo.md` no Lote 12 (2026-09-27). O DEPLOY-01 foi para `fazendo.md` no Lote 13 (2026-09-30). Resta só o INFO-01, que é informativo.

---

## INFO-01: linhas não alcançáveis aceitas como fora da meta de cobertura — informativo

**Status:** registrado; nenhuma ação pendente
**Camada:** Testes
**Origem:** 🔴 TestBrain, TST-02 e TST-03 (Lote 7, 2026-09-26). Aceito pelo 🤍 MegaBrain.

**Descrição:** As linhas que restam sem cobertura no Go só são alcançáveis mudando o código de produção: erros de `crypto/rand` (`password_generator`), `os.Getwd`/`os.Executable`, `rows.Columns()`, a escrita no `DATA` do SMTP, a escrita do cabeçalho num `csv.Writer` com buffer e os ramos de `maskDSN` sem `@`/`:`. Também ficam fora da meta os `cmd/*/main.go` finos e o `cmd/server`. Não há ação a tomar, salvo se o usuário pedir 100%.

> **Nota (BUG-12, Lote 10):** em `scanSenhaHistorico` (92.3%), o ramo `sql.ErrNoRows` → `ErrNotFound` é inalcançável, porque a função só é chamada dentro de `rows.Next`.

> **Nota (SEC-10 e SEC-09, Lote 11):** ramos inalcançáveis do SEC-10: erros de `crypto/rand` em `indiceAleatorio`/`SenhaAlfanumerica`, `indiceAleatorio` com `n <= 0` e a falha de `os.Chmod` em `RenderSeedFile`. `newEmailService` fica em `package main` do `cmd/server` (0%).

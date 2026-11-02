# SistemaCompleto

Sistema com API Go (rotaperfumes-api) e frontend Next.js. Este projeto é gerenciado por uma **equipe de agentes** que recebem qualquer nova execução e a processam da melhor forma possível.

## Equipe de Agentes (em `.claude/agents/`)

| 🤍 Agente     | Papel                  | Arquivo            | Cor  | Tag Visual      |
| ------------- | ---------------------- | ------------------ | ---- | -------------- |
| **MegaBrain** | Líder                  | `lider.md`         | ⚪️ Branco | `🤍 MegaBrain` |
| **SubBrain**  | Analista / Coordenador | `analista.md`       | 🔵 Azul  | `🔵 SubBrain` |
| **DataBrain** | Dev Database           | `database.md`       | 🌸 Rosa  | `🌸 DataBrain` |
| **BackBrain** | Dev Backend            | `backend.md`        | 🟡 Amarelo | `🟡 BackBrain` |
| **FrontBrain**| Dev Frontend           | `frontend.md`      | 🟢 Verde | `🟢 FrontBrain` |
| **TestBrain** | Dev Testes / QA       | `tester.md`        | 🔴 Vermelho | `🔴 TestBrain` |
| **SecBrain**  | Dev Segurança          | `security.md`       | 🟣 Roxo  | `🟣 SecBrain` |

> **Regra de cor:** Toda resposta de um agente deve iniciar com sua **tag visual** (ex: `🤍 MegaBrain`, `🔵 SubBrain`, etc.) e usar a cor correspondente como borda/destaque.

## Fluxo de Execução (via MegaBrain)

1. **Usuário** envia o pedido.
2. **🤍 MegaBrain** carrega a skill `megabrain-router` (`.claude/skills/megabrain-router/SKILL.md`), decompõe em tarefas e prioriza.
3. **🔵 SubBrain** atualiza o Kanban e planeja a divisão por camada.
4. **🟣 SecBrain** avalia riscos de segurança da feature e indica correções necessárias às demais camadas.
5. **🌸 DataBrain** (se houver mudança de schema) → cria tabelas e chaves.
6. **🟡 BackBrain** implementa APIs (paga o server com logs).
7. **🟢 FrontBrain** implementa telas consumindo as APIs.
8. **🔴 TestBrain** escreve e executa testes com cobertura.
9. **🔵 SubBrain** fecha documentação, Postman, manuais e move a tarefa para `feito.md`.

## Skill de Roteamento

A skill `megabrain-router` é o **roteador padrão** — toda execução nova passa por ela. Está em:
- `.claude/skills/megabrain-router/SKILL.md`

## Kanban (em `tarefas/`)

- `afazer.md` — backlog (pendentes)
- `fazendo.md` — em execução (com passo atual)
- `feito.md` — concluídas (histórico)

Toda mudança de estado é refletida movendo o card entre os três arquivos.

## Como Usar

1. Envie seu pedido normalmente.
2. O **MegaBrain (🤍)** recebe, decompoe e distribui para o agente correto.
3. Cada agente responde com sua **tag visual e cor** para identificação clara.
4. O **SubBrain (🔵)** mantém o Kanban atualizado ao longo do processo.

## Comandos úteis

```bash
make help          # ver todos os comandos
make setup-check   # conferir as ferramentas do ambiente (só verifica)
make setup         # instalar/ajustar as ferramentas (Go, Node, govulncheck, mariadb, Postman CLI)
make db-reset      # recriar banco
make dev-api       # subir API Go
make dev-frontend  # subir frontend Next.js
make test          # rodar testes Go
make test-frontend # rodar testes frontend
make deps-audit    # auditar dependências Go + frontend (vulns e pacotes desatualizados)
```

Banco: **MariaDB 11.4 LTS**. Os alvos `make db-*` (inclusive `db-seed`) exigem o cliente `mariadb` no `PATH`; o driver Go é o `go-sql-driver/mysql` (compatível).

## Instalação do ambiente

- **Scripts (SETUP-01):** `scripts/setup-linux.sh` (Debian/Ubuntu e derivados, via apt; em outras distros só confere) e `scripts/setup-windows.ps1` (PowerShell 5.1+/7, via winget). Pelo make: `make setup-check` (só verifica, sai com erro se faltar algo) e `make setup` (instala o que falta; idempotente). Opções: `--check`/`-Check`, `--deps`/`-Deps` (`go mod download` + `make frontend-deps`), `--go-dir`/`-GoDir`, `--go-sistema` (Linux, `/usr/local/go` com sudo), `--sem-sudo`; ver `--help`. Pelo make: `make setup SETUP_ARGS="--deps"`.
- **Versões (fonte da verdade):** Go = diretiva `go` dos `go.mod` (hoje 1.26.0); Node = `frontend/.nvmrc` (mínimo em `engines` do `frontend/package.json`); demais mínimos (cliente MariaDB 11.4, Postman CLI, git, make, openssl, versão do nvm) em `scripts/versoes.conf`. Ao subir a versão do Go ou do Node, mude só esses arquivos (e o `GO_IMAGE` do Jenkinsfile / `NODE_VERSION` do `frontend/Dockerfile`).
- **Pré-requisito do `make test` (TST-08):** Go **instalado localmente** na versão do `go.mod` (tarball/MSI/zip oficial de go.dev), primeiro no `PATH`, e `GOTOOLCHAIN=local` (`go env -w GOTOOLCHAIN=local`; o `make setup` grava). Sem isso, um `go` mais antigo (ex.: o `golang-go` do apt) troca sozinho para um toolchain baixado do módulo `golang.org/toolchain`, e o TST-08 registrou `go: no such tool "covdata"` nesse cenário com `-coverpkg`. O `make setup-check` confere `go tool -n covdata` e roda um `go test -coverpkg` mínimo.
- **Postman CLI:** o script instala o pacote npm oficial `postman-cli` (ou o instalador oficial com `--postman-script`). Se o app desktop do Postman estiver no `PATH` antes do CLI, `postman --version` falha e o script avisa.
- **Docker:** opcional (deploy); só é conferido, nunca instalado.

## Auditoria de dependências

- **Quando:** rodar `make deps-audit` **mensalmente** e **antes de cada deploy**.
- **Pré-requisito (Go):** `go install golang.org/x/vuln/cmd/govulncheck@latest` (o `make setup` faz) e garantir que `$(go env GOPATH)/bin` está no `PATH`. Sem o govulncheck, `make deps-audit-go` imprime um aviso e falha.
- **Alvos:** `deps-audit-go` (`go list -m -u all` + `govulncheck ./...`) e `deps-audit-frontend` (`npm outdated` informativo + `npm audit --omit=dev --audit-level=high`, falha em high ou acima).
- **Rodar ambos mesmo se um falhar:** `make -k deps-audit`.
- **Achados:** abrir um card no Kanban (`tarefas/afazer.md`) para cada correção necessária. **Não** usar `npm audit fix --force` (upgrades major) sem um card próprio; `npm audit fix` sem `--force` é permitido se build, lint e testes continuarem passando.

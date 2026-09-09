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

> **Regra de cor:** Toda resposta de um agente deve iniciar com sua **tag visual** (ex: `🤍 MegaBrain`, `🔵 SubBrain`, etc.) e usar a cor correspondente como borda/destaque.

## Fluxo de Execução (via MegaBrain)

1. **Usuário** envia o pedido.
2. **🤍 MegaBrain** carrega a skill `megabrain-router` (`.claude/skills/megabrain-router/SKILL.md`), decompõe em tarefas e prioriza.
3. **🔵 SubBrain** atualiza o Kanban e planeja a divisão por camada.
4. **🌸 DataBrain** (se houver mudança de schema) → cria tabelas e chaves.
5. **🟡 BackBrain** implementa APIs (paga o server com logs).
6. **🟢 FrontBrain** implementa telas consumindo as APIs.
7. **🔴 TestBrain** escreve e executa testes com cobertura.
8. **🔵 SubBrain** fecha documentação, Postman, manuais e move a tarefa para `feito.md`.

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
make db-reset      # recriar banco
make dev-api       # subir API Go
make dev-frontend  # subir frontend Next.js
make test          # rodar testes Go
make test-frontend # rodar testes frontend
```

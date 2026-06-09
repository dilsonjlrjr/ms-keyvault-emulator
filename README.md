# kvemu — Azure Key Vault Emulator (backend)

Emulador local do data-plane do Azure Key Vault (Secrets, Keys, Certificates) em
**Go 1.22+ + SQLite**, com foco em compatibilidade total com **Spring Boot 2.7+**
(`spring-cloud-azure-starter-keyvault`).

A especificação e a arquitetura completas estão em [`../arquitetura`](../arquitetura).

## Premissas de execução (valem para todas as etapas)

- Usar **context-mode**, **rtk-ia** e **caveman** no fluxo de trabalho.
- **Não** adicionar autoria de assistente em commits, código ou docs (sem `Co-Authored-By`).
- Este diretório `backend/` é o projeto do emulador, com **repositório Git próprio**.
- **Sempre** commitar com **mensagens semânticas** (Conventional Commits) — ver abaixo.

## Convenção de commits (semânticos)

Formato: `<tipo>(<escopo opcional>): <descrição no imperativo>`

| Tipo | Uso |
|------|-----|
| `feat` | nova funcionalidade |
| `fix` | correção de bug |
| `docs` | documentação |
| `test` | testes |
| `refactor` | refatoração sem mudança de comportamento |
| `chore` | tarefa de manutenção (deps, config) |
| `build` | build/empacotamento (Docker, goreleaser) |
| `ci` | pipeline |
| `perf` | performance |

Exemplos: `feat(challenge): emite WWW-Authenticate canônico`,
`fix(auth): remove header Bearer duplicado`, `build(docker): adiciona compose do emulador`.

## Status

Fase 0 (fundação). Ver roadmap em [`../arquitetura/08-roadmap.md`](../arquitetura/08-roadmap.md).

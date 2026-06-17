# CLAUDE.md — rail0-cli

Working instructions for Claude Code in this repository. This file carries the
project-wide rules (shared across the whole rail0 project) followed by a
CLI-specific section. The same project-wide rules are duplicated in every per-repo CLAUDE.md; keep them consistent.

## Project structure

rail0 is a multi-repo project. All repositories prefixed with `rail0-` are part of the same project, as is `rail0` itself (the smart contract). All repos are located under the same parent directory (`/Users/pierlu/Documents/GitHub/`).

| Repo | Role |
| --- | --- |
| `rail0` | EVM smart contract (Solidity) |
| `rail0-gateway` | Backend API (Ruby/Grape) |
| `rail0-indexer` | On-chain event indexer (TypeScript/Envio) |
| `rail0-admin` | Admin UI |
| `rail0-cli` | CLI tool |
| `rail0-ruby` | Ruby SDK |
| `rail0-go` | Go SDK |
| `rail0-test` | Integration and cross-SDK tests |

> Note: `rail0-api`, `rail0-ts`, `rail0-py`, and `rail0-rust` are temporarily out of scope.

When a change in one repo affects the contract, the indexer, or any SDK, flag it explicitly and propose coordinated changes across the relevant repos.

## Rules

1. **Always propose before implementing.** For any non-trivial change, present a plan of action and wait for explicit confirmation before writing any code.

2. **Follow language and framework conventions.** Respect the idioms and conventions of the language and framework used in this repo. Match the style of surrounding code.

3. **Do not make structural changes without consent.** The architecture of each repo is intentional. Do not reorganise layers, introduce new abstractions, or change project layout without explicit approval.

4. **Avoid duplication — favour reuse and centralisation.** Before adding code, check whether the functionality already exists. Prefer extending existing helpers or packages over creating parallel implementations.

5. **Always work on a branch.** Never commit directly to `main`. If no branch exists for the current task, create one before making any changes using the naming convention `feature/short-desc` for new functionality or `fix/short-desc` for bug fixes.

6. **Use Conventional Commits format.** Every commit message must follow the [Conventional Commits](https://www.conventionalcommits.org/) specification: `type(scope): description`, where type is one of `feat`, `fix`, `refactor`, `docs`, `test`, `chore`.

7. **Always open a draft PR.** After the first push to a branch, open a pull request in draft status if one does not already exist. The PR title must also follow Conventional Commits format.

8. **Never log sensitive data.** Do not log private keys, signatures, raw transaction payloads, HMAC secrets, JWT tokens, or any user-identifying data. When logging errors or request context, include only non-sensitive identifiers (e.g. `payment_id`, `chain_id`, `operation`).

9. **Comment non-obvious functions.** Add a detailed comment to any function whose logic is not immediately clear from its name alone — explaining what it does, why it works that way, and any non-obvious invariants or edge cases. Simple accessors need no comment; cryptographic operations, signing, and multi-step workflows do.

10. **Keep documentation and tests in sync.** After every change, update the documentation and tests present in the repo (README, command help text, unit tests). Do not consider a task complete until all are consistent with the code.

11. **Keep all SDKs aligned when asked.** When asked to update the SDKs, check every SDK repo (`rail0-ruby`, `rail0-go`, `rail0-cli`) for alignment with the current gateway API surface. For each SDK: update client methods, README, and unit tests. Flag any SDK where alignment requires a breaking change.

12. **Align all tests when asked.** When asked to align or update tests, cover both layers: unit tests in every affected repo (gateway, indexer, all SDKs), and integration tests in `rail0-test` (API tests, flow tests for each SDK language, and cross-SDK tests). Verify that test fixtures, helper methods, and expected response shapes are consistent with the current gateway behaviour.

## CLI-specific conventions

`rail0-cli` is the command-line client for rail0-gateway, written in Go on top of the `rail0-go` SDK. Beyond the rules above, the following conventions are specific to this repo.

- **Idiomatic Go.** `gofmt`-clean, errors wrapped with context, `context.Context` threaded through commands. Match the surrounding package style.
- **The CLI consumes the SDK — it does not re-implement it.** All gateway calls go through `rail0-go`; do not hand-roll HTTP or signing in `cmd/`. When the gateway surface changes, update `rail0-go` first, then surface it here.
- **Commands live under `cmd/`,** one concern per file; shared helpers in `internal/`. Keep new commands consistent with the existing command/flag style rather than introducing a parallel structure.
- **No `/sync` surface.** The HMAC-protected indexer endpoints are not exposed by the CLI.
- **Validation before commit.** `go build ./...`, `go vet ./...`, and `go test ./...` must pass.

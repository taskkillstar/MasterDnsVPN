# AGENTS.md — conventions for agents and contributors

MasterDnsVPN is a high-performance DNS tunneling system carrying TCP and SOCKS traffic over DNS query-response exchanges.

## Repository layout

| Path | Role |
|------|------|
| `cmd/client/` | Client CLI application entry point. |
| `cmd/server/` | Authoritative DNS server application entry point. |
| `internal/client/` | Client tunnel path lifecycle, resolver dispatch, and tunnel orchestration. |
| `internal/vpnproto/` | Wire protocol packet framing, serialization, and parsing. |
| `internal/arq/` | Selective-repeat ARQ reliability layer over DNS. |
| `internal/mlq/` | Multi-Level Queue priority scheduling. |
| `internal/fragmentstore/` | Packet fragmentation and reassembly. |
| `internal/basecodec/` | DNS payload encodings (Base32, Base36, Base64, Hex). |
| `internal/security/` | Encryption and cryptographic ciphers (ChaCha20, AES, XOR). |
| `internal/logger/` | Centralized structured logging. |
| `internal/config/` | Client and server TOML configuration parsers. |

## Build & test commands

```bash
# Run the entire test suite
go test ./...

# Fast scoped tests during development (internal/client tests take ~60s; test individual packages for tight feedback loops)
go test ./internal/<package>

# Build binaries
go build ./cmd/client
go build ./cmd/server
```

## Conventions

- **Domain language**: Use terms defined in [`CONTEXT.md`](./CONTEXT.md). Respect `_Avoid_` aliases.
- **Logging**: Use `internal/logger` instances scoped to subsystems. Avoid raw `fmt.Println` or stdlib `log`.
- **Error handling**: Return wrapped errors using `fmt.Errorf("...: %w", err)` for actionable call chains.
- **Testing**: Place unit tests alongside code in `*_test.go`. Target deterministic tests; isolate network dependencies with mocks or loopbacks.
- **Commits**: Conventional commits in lowercase (`feat(arq):`, `fix(client):`, `chore:`, `docs:`).

## Agent skills

### Issue tracker

Local markdown files in `.scratch/`. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical five-role vocabulary (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context (`CONTEXT.md` and `docs/adr/` at repo root). See `docs/agents/domain.md`.

## Git Remotes & Upstream PR Protocol

- **Remotes**:
  - `origin` (`https://github.com/taskkillstar/MasterDnsVPN.git`): Contributor fork. Fork `main` is downstream-only and retains personal agent tooling, `AGENTS.md`, `CONTEXT.md`, and `docs/agents/`.
  - `upstream` (`https://github.com/masterking32/MasterDnsVPN.git`): Official upstream maintainer repository.
- **Upstream PR boundaries**:
  - NEVER include personal agent workflows (`AGENTS.md`), redundant context (`CONTEXT.md`), or private planning docs (`docs/agents/`) in PR branches targeting `upstream`.
  - Upstream PRs must strictly contain only feature code, tests, and agreed-upon public documentation.
  - Architectural guides (such as `docs/resolver-manager.md`) should be submitted in a dedicated docs PR or explicitly coordinated with the maintainer.
- **Syncing fork `main` from upstream**:
  - Always synchronize fork `main` using `git merge upstream/main` (never rebase `main` onto upstream) so fork-only personal files remain preserved.
  - Never open a PR directly from fork `main` to `upstream/main`.

# AGENTS.md — conventions for agents and contributors

This repository (`DNStunnel` / `MasterDnsVPN`) is the **downstream Go core fork**
of `masterking32/MasterDnsVPN`. It is also the upstream core provider for the
Android client repository (`taskkillstar/MasterDnsVPN-AndroidClient`).

Plan executors, coding agents, and human contributors must follow the conventions
below to move fast on customized features while maintaining clean compatibility
with upstream changes.

---

## 1. Repository Identity & Roles

| Remote / Branch | URL / Target | Role | Policy |
| :--- | :--- | :--- | :--- |
| `origin` | `https://github.com/taskkillstar/MasterDnsVPN.git` | Your downstream fork. | Default push/pull target. |
| `upstream` | `https://github.com/masterking32/MasterDnsVPN.git` | Upstream project. | Read-only. Never push directly. |
| `main` | Tracks `origin/main`. | Downstream production trunk. | **Never force-push**. Must remain buildable & tested. |

The Android client (`DNStunnel-mobile`) vendors `cmd/` and `internal/` directly
from `origin/main` via its `scripts/sync_core.sh`. Stability on `origin/main` is
critical.

---

## 2. Two-Track Development Strategy

Upstream maintainer review is slow or reluctant. **Never block on upstream approval.**

### Track 1: Downstream Features & Optimizations (Move Fast)
- Branch from `origin/main` (e.g. `feat/<name>`, `perf/<name>`).
- Implement features in a modular, decoupled manner.
- Merge into `origin/main` as soon as unit tests pass (`go test ./...`).
- Do not wait for or request upstream review for opinionated fork features.

### Track 2: Upstream Bugfixes (PR Candidates)
- When fixing an upstream bug (e.g. `fix/strategy6-probation-starvation`), branch
  strictly from `upstream/main`:
  ```bash
  git checkout -b fix/<bug-name> upstream/main
  ```
- Keep the commit atomic, minimal, and free of any downstream-only dependencies.
- Push to `origin/fix/<bug-name>` and open an upstream PR to `masterking32/MasterDnsVPN`.
- Cherry-pick or merge this branch into your `origin/main` immediately. Whether
  upstream merges it today, in 6 months, or never, your fork continues moving forward.

---

## 3. Commit Conventions

Follow Conventional Commits: `<type>(<scope>): <short summary>` in lowercase.

- **Types**:
  - `feat`: New feature or user-facing enhancement.
  - `fix`: Bug fix.
  - `perf`: Performance optimization.
  - `refactor`: Code change that neither fixes a bug nor adds a feature.
  - `test`: Adding or correcting tests.
  - `chore`: Maintenance, dependencies, or upstream sync.
- **Scopes**:
  - `client`: Client logic, runtime, or connection management.
  - `resolver`: Resolver probing, scoring, or selection.
  - `balancer`: Strategy algorithms or pool balancing.
  - `config`: TOML configuration parsing and defaults.
  - `server`: Server/relay logic.
  - `upstream`: Upstream synchronization commits.

Examples:
- `feat(resolver): implement dynamic MAX_ACTIVE_RESOLVERS pool bounding`
- `fix(balancer): eliminate probation starvation in LossThenLatency strategy`
- `chore(upstream): sync upstream main (acbf1c6)`

---

## 4. Code Decoupling: Preventing Upstream Merge Conflicts

To prevent merge hell when syncing upstream commits:

### 1. Additive-First Architecture
- Place new features, data structures, metrics, and optimizers in **new files**
  (e.g., `internal/client/micro_burst.go`, `internal/client/runtime_stats.go`).
- Avoid inlining hundreds of lines of downstream code into upstream files
  (e.g. `balancer.go`, `client.go`).

### 2. Surgical Hooks in Upstream Files
- When upstream files must interact with downstream features, keep modifications to
  **minimal hooks** (1–3 lines):
  ```go
  // Downstream Hook: record runtime stats if enabled
  if s := r.statsCollector; s != nil {
      s.RecordLatency(resolver, latency)
  }
  ```
- Delegate all complex logic to helper methods in your own files.

### 3. No Translation / Doc Churn
- Upstream frequently pushes translations to `README_*.MD` (`README_ZH.MD`,
  `README_RU.MD`, `README_ES.MD`, etc.).
- **DO NOT edit non-English README translation files** for downstream features.
- Keep downstream documentation either in `README.MD` or in dedicated fork doc files.
  Editing translation files guarantees merge conflicts on every upstream README sync.

### 4. Zero Blanket Formatting / Linting Churn
- Do not run auto-formatters (e.g. `gofmt -w .`) over untouched upstream files.
- Keep diffs against `upstream/main` clean and restricted only to intentional logic changes.

---

## 5. Upstream Sync Protocol

### Enabling Git `rerere` (Mandatory)
Ensure Reuse Recorded Resolution is enabled so conflict resolutions are remembered:
```bash
git config rerere.enabled true
git config rerere.autoupdate true
```

### Syncing Upstream Locally
Use the helper script:
```bash
bash ./scripts/sync_upstream.sh
```
Or manually:
```bash
git fetch upstream
git checkout main
git merge upstream/main -m "chore(upstream): sync upstream main ($(git rev-parse --short upstream/main))"
go test ./...
git push origin main
```

### Propagating to the Android Client
Once `origin/main` is verified and pushed, sync the Android repository
(`c:\Dev\Projects\DNStunnel-mobile`):
```powershell
& "$env:LOCALAPPDATA\Programs\Git\bin\bash.exe" ./scripts/sync_core.sh
```
Verify the Android gomobile bridge still compiles:
```bash
GOOS=android GOARCH=arm64 go build ./mobile/...
```

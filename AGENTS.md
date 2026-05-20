# AGENTS.md

## What this is

`ssh swiggy.dev` — an SSH server for ordering Swiggy Instamart groceries from the terminal. Users connect with an SSH key, link their Swiggy account via a browser OAuth 2.1 + PKCE flow, and interact through a Bubbletea TUI.

**Current state**: Auth & identity foundation complete. Instamart integration is next.

---

## Module and baseline

- Module path: `swiggy-ssh`
- Go: `1.23.0` / `toolchain go1.23.6` — do not bump
- No new dependencies without explicit instruction

---

## Architecture

Ports & Adapters (Clean Architecture). Five layer groups:

- **Domain**: `internal/domain/*` entities, domain errors, and ports
- **Application**: `internal/application/*` use cases and orchestration
- **Presentation**: `internal/presentation/ssh`, `internal/presentation/http`, `internal/presentation/tui`
- **Infrastructure**: `internal/infrastructure/*` Postgres, Redis, crypto, and provider adapters
- **Platform**: `internal/platform/*` config and logging

**Hard rule**: presentation adapters must never import infrastructure packages. `cmd/swiggy-ssh/main.go` is the only wiring point.

**Client-agnostic backend**: domain services have no SSH/HTTP/TUI imports. SSH is the first client; future clients (web, WhatsApp, agentic) reuse the same services.

**Skill to load**: use `.agents/skills/swiggy-ssh-clean-architecture/SKILL.md` when doing architecture, package-boundary, refactor, new feature placement, ports/repository/use-case naming, adapter naming, or architecture review work.

---

## Where things live

| What you need | Where to look |
|---|---|
| Domain types, ports, error sentinels | `internal/domain/auth/auth.go`, `internal/domain/identity/identity.go` |
| Auth orchestration (`EnsureValidAccountUseCase.Execute`) | `internal/application/auth/ensure_valid_account.go` |
| Identity/session use cases (`ResolveSSHIdentityUseCase`, `StartTerminalSessionUseCase`, `EndTerminalSessionUseCase`) | `internal/application/identity/` |
| SSH connection + session routing | `internal/presentation/ssh/server.go` |
| Browser auth start/callback handlers | `internal/presentation/http/` |
| TUI screens (Bubbletea v1 + Lipgloss) | `internal/presentation/tui/tui.go` |
| Postgres repositories | `internal/infrastructure/persistence/postgres/postgres.go` |
| DB schema + migrations | `internal/infrastructure/persistence/postgres/migrations/` |
| Redis browser auth attempt service | `internal/infrastructure/cache/redis/redis_logincode.go` |
| Token encryption (AES-256-GCM) | `internal/infrastructure/crypto/aes.go` |
| Config + env vars | `internal/platform/config/config.go` |
| Wiring entrypoint | `cmd/swiggy-ssh/main.go` |
| In-memory mocks for tests | `internal/infrastructure/provider/mock/` |
| Docker / Compose setup | `Dockerfile`, `compose.yaml` |

---

## Key constraints

- `ssh_identities.id` is the durable local principal — there is no `users` table unless a real user identity is introduced later
- Browser auth attempts use high-entropy tokens; raw attempt tokens are **never stored as Redis keys** — Redis keys use SHA-256 hex
- PKCE `code_verifier` is stored only in the TTL-limited Redis auth attempt value and is never rendered or logged
- `OAuthAccount.AccessToken` is **never logged or rendered** — store decrypts on read, callers get plaintext
- `TOKEN_ENCRYPTION_KEY` default is dev-only — production guard in `main.go` refuses startup with it
- `NoOpEncryptor` is for tests only — never wire in production
- Mock tokens are local-only placeholders — no real Swiggy credentials ever committed
- Every service with time logic has an injectable `now func() time.Time` — never call `time.Now()` inside a service method directly

---

## Dependency import rules

| Package | Must NOT import |
|---|---|
| `internal/domain/*` | anything from this repo |
| `internal/application/*` | `internal/infrastructure/*`, `internal/presentation/*`, `internal/platform/*` |
| `internal/presentation/*` | `internal/infrastructure/*` |
| `internal/infrastructure/*` | `internal/presentation/*` |
| `internal/platform/*` | feature packages (`domain`, `application`, `presentation`, `infrastructure`) |

---

## Testing

- `go test ./...` must pass with no external services
- Integration tests in `internal/infrastructure/persistence/postgres/` skip unless `TEST_DATABASE_URL` is set — run with `make test-integration`
- No mocks library — all mocks are hand-written structs
- Interactive TUI tests use `context.WithTimeout(200ms)` so Bubbletea exits cleanly
- Read existing tests before writing new ones to follow established patterns

---

## Running

```bash
make up       # full stack via Docker Compose (builds image, runs migrations, starts everything)
make up-swiggy # full stack with real Swiggy OAuth using SWIGGY_PROVIDER=swiggy
make down     # stop everything
make reset    # wipe all volumes and containers (fresh start)

make dev      # run app on host (requires make up first for Postgres + Redis)
make dev-swiggy # run app on host with real Swiggy OAuth
make migrate  # apply pending migrations against running Postgres
make test     # unit tests
```

---

## Linear workflow

- Move issues to **In Progress** before starting work
- Move to **Done** after reviewer approval
- Post a comment on the issue summarising what was built and what tests pass
- Use the **dev + reviewer agent pattern**: new dev session per issue, new reviewer session per issue; send fixes back to the same dev session and re-review with the same reviewer session
- Linear project: **ssh swiggy.dev** — look up issue IDs via the Linear tool before updating

---

## What's next

- **Instamart integration** (SWGY-15+): product search, cart, checkout via real Swiggy API
- **Keyboard input wiring**: `HomeView`, `LoginSuccessView`, `InstamartView` have `In io.Reader` fields ready — pass `ssh.Channel` as `In` to enable cursor movement in real sessions
- **`UpdateCurrentScreen`**: `TerminalSession.CurrentScreen` is set once at session start and never updated — needs a tracker method when screen navigation is wired
- **Real Instamart provider**: `internal/infrastructure/provider/swiggy/client.go` has auth support; product/cart/checkout APIs are next
- **Audit logging**: `audit_events` table is in the schema, no writes yet

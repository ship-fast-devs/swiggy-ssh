# swiggy.dev SSH

Order Swiggy Instamart groceries from your terminal over SSH.

```
ssh swiggy.dev
```

> **Status**: Auth & identity foundation complete. Instamart integration in progress.

![swiggy.ssh home screen](assets/image.png)

---

## What it is

A Go SSH server that lets you browse and order from Swiggy Instamart without leaving your terminal. Connect with your existing SSH key — the first time you connect you link your Swiggy account via a browser login code. After that, it remembers you.

---

## Prerequisites

- **Docker** with the Compose plugin
- An **SSH key pair** (Ed25519 recommended)
- Go 1.23+ *(only needed for local dev without Docker)*

---

## Quick start

```bash
# 1. Clone and enter the repo
git clone <repo-url>
cd swiggy-ssh

# 2. Copy environment config
cp .env.example .env

# 3. Build and start everything
make up
```

That's it. Compose builds the app image, starts Postgres and Redis, runs migrations, then starts the SSH + HTTP servers — in the right order automatically.

Connect in a second terminal:

```bash
ssh -p 2222 -i ~/.ssh/id_ed25519 localhost
```

**First Instamart session** — choose Instamart from the terminal home screen, then the app shows a browser login URL. Open the URL in your browser and complete Swiggy login; the SSH session continues automatically.

**Returning Instamart sessions** skip the login step while the linked account token is still valid.

---

## Commands

### Docker Compose

| Command | What it does |
|---|---|
| `make up` | Build image + start everything (app, migrate, Postgres, Redis) |
| `make up-swiggy` | Start everything with real Swiggy OAuth instead of mock auth; defaults client id to `swiggy-mcp` |
| `make down` | Stop and remove all containers |
| `make build` | Rebuild the app image without starting |
| `make logs` | Tail app logs |
| `make ps` | Show container status |
| `make reset` | Wipe all volumes and containers (fresh start) |

### Local dev (app on host)

| Command | What it does |
|---|---|
| `make dev` | Run the app on host (requires Postgres + Redis running) |
| `make dev-swiggy` | Run the app on host with real Swiggy OAuth instead of mock auth; defaults client id to `swiggy-mcp` |
| `make migrate` | Apply all pending DB migrations |
| `make migrate-down` | Roll back one migration step |

### Code

| Command | What it does |
|---|---|
| `make test` | Run all unit tests (no DB/Redis needed) |
| `make test-integration` | Run Postgres integration tests (requires `TEST_DATABASE_URL`) |
| `make lint` | Run `go vet` |
| `make fmt` | Run `gofmt` |

---

## Environment variables

Copy `.env.example` to `.env` — mock defaults work out of the box for local development. To use real Swiggy OAuth locally, set:

```env
SWIGGY_PROVIDER=swiggy
SWIGGY_CLIENT_ID=swiggy-mcp
PUBLIC_BASE_URL=http://localhost:8080
SWIGGY_AUTH_AUTHORIZE_URL=https://mcp.swiggy.com/auth/authorize
SWIGGY_AUTH_TOKEN_URL=https://mcp.swiggy.com/auth/token
SWIGGY_AUTH_SCOPES=mcp:tools
```

Then ensure Swiggy has allowlisted this exact redirect URI for your client:

```text
http://localhost:8080/auth/callback
```

Run with Docker:

```bash
make up-swiggy
```

Or run on host:

```bash
make up # Postgres + Redis
make dev-swiggy
```

Every variable is documented with inline comments in `.env.example`.

---

## How the auth flow works

1. **SSH connect** — your Ed25519 public key fingerprint is used to look up an existing durable SSH identity. Unknown or no-key connections start as guest sessions.
2. **Home / Instamart selection** — the SSH TUI opens at the home screen. Auth starts when the user selects Instamart; an unknown public key is registered as a durable SSH identity at this point.
3. **Browser auth attempt** — a short-lived one-time auth attempt is issued. Only a SHA-256 hash of the public attempt token is used as the Redis key; PKCE verifier material is stored only in the TTL-limited Redis value.
4. **Swiggy OAuth** — open the terminal URL. `/auth/start` redirects to `https://mcp.swiggy.com/auth/authorize` with OAuth 2.1 + PKCE params. Swiggy redirects back to `/auth/callback` with `code` and `state`.
5. **Token exchange** — the server exchanges `code + code_verifier` at `https://mcp.swiggy.com/auth/token`, encrypts the returned access token, and marks the auth attempt completed. The SSH session polls every 2 seconds.
6. **Returning identities** — if your OAuth account is already valid for this SSH identity, the browser-auth step is skipped and the session proceeds to the Instamart placeholder.

---

## Project layout

```
cmd/
  swiggy-ssh/           # Main server entrypoint (SSH + HTTP)
  swiggy-ssh-migrate/   # DB migration CLI (up / down / drop)

internal/
  domain/               # Entities, domain errors, and ports only
    auth/               # OAuthAccount, LoginCode, auth ports
    identity/           # SSHIdentity, TerminalSession, identity ports
    instamart/          # Instamart domain (stub, in progress)
  application/          # Client-agnostic use cases
    auth/               # EnsureValidAccountUseCase.Execute orchestration
    identity/           # ResolveSSHIdentity/StartTerminalSession/EndTerminalSession use cases
  presentation/         # Delivery adapters; no infrastructure imports
    ssh/                # SSH connection handling and screen routing
    http/               # /login page and /health handlers
    tui/                # Terminal screens (Bubbletea v1 + Lipgloss)
  infrastructure/       # Frameworks/drivers/adapters wired in cmd only
    cache/redis/        # Redis login-code service and client
    crypto/             # AES-256-GCM token encryption + NoOp for tests
    persistence/postgres/ # Postgres repositories + embedded migrations
    provider/           # Mock and Swiggy provider adapters
  platform/             # Cross-cutting platform concerns
    config/             # Env-based config with safe defaults
    logging/            # Structured slog setup
```

Dependency direction is inward: `presentation` may depend on `application` and `domain`, `application` may depend on `domain`, and `domain` imports no repo packages. Infrastructure adapters implement domain/application ports and are wired only from `cmd/`.

---

## Local development (app on host)

If you prefer a faster code/run loop with the app running directly on your machine:

```bash
# Start Postgres and Redis via Docker
make up

# Apply migrations against the running Postgres
make migrate

# Run the app
make dev
```

Changes to Go code take effect immediately on the next `make dev` run — no image rebuild needed.

---

## Database migrations

Migrations are embedded in the binary using Go `embed` — no external tools needed.

```bash
# Apply all pending migrations
make migrate

# Roll back one step
make migrate-down

# Drop all tables (local/dev only — refused in production)
go run ./cmd/swiggy-ssh-migrate drop
```

Migration files live in `internal/infrastructure/persistence/postgres/migrations/` as paired `*.up.sql` / `*.down.sql` files.

To reset your local database completely:

```bash
make reset    # wipes Docker volumes and containers
make up       # start fresh with migrations applied automatically
```

---

## Running tests

```bash
# Unit tests — no database or Redis required
make test

# Postgres integration tests
TEST_DATABASE_URL="postgres://swiggy:swiggy@localhost:5432/swiggy_ssh?sslmode=disable" make test-integration
```

Unit tests cover: auth service state machine, login code lifecycle, token encryption, TUI screen rendering, SSH session routing, HTTP login handlers, identity resolution.

---

## Port conflicts

The default ports are `2222` (SSH), `8080` (HTTP), `5432` (Postgres), `6379` (Redis). Override any of them in `.env`:

```env
SSH_PORT=2223
HTTP_PORT=8081
POSTGRES_PORT=5433
REDIS_PORT=6380
```

If you change `HTTP_PORT`, also update `PUBLIC_BASE_URL` so the login URL shown in the terminal is correct:

```env
PUBLIC_BASE_URL=http://localhost:8081
```

For local dev (app on host), use matching env vars when starting:

```bash
POSTGRES_PORT=5433 REDIS_PORT=6380 make up
DATABASE_URL=postgres://swiggy:swiggy@localhost:5433/swiggy_ssh?sslmode=disable \
REDIS_URL=redis://localhost:6380/0 make dev
```

---

## Troubleshooting

**`ssh localhost -p 2222` gives `Host key verification failed`**
The server generates a new host key on first run at `.local/ssh_host_ed25519_key`. If you wiped and restarted, remove the old entry:
```bash
ssh-keygen -R [localhost]:2222
```

**`connection refused` on port 2222**
The server isn't running. Run `make up` (Docker) or `make dev` (local) first.

**Browser login page shows "Login code not found or expired"**
The 10-minute TTL elapsed, or the code was already used. Reconnect via SSH to get a new code.

**App can't connect to Postgres or Redis**
Check containers are healthy: `make ps`. Verify `DATABASE_URL` and `REDIS_URL` match your port settings.

**Simulate reconnect-required**
```sql
UPDATE oauth_accounts
SET status = 'reconnect_required'
WHERE ssh_identity_id = '<your-ssh-identity-id>';
```
Then reconnect via SSH — a new login code will be issued automatically.

---

## Tech stack

| Layer | Technology |
|---|---|
| Language | Go 1.23 |
| SSH server | `golang.org/x/crypto/ssh` |
| HTTP server | stdlib `net/http` + `html/template` |
| TUI | Bubbletea v1 + Lipgloss (Charm) |
| Database | Postgres via `pgx/v5` |
| Cache / login codes | Redis via `go-redis/v9` |
| Migrations | `golang-migrate/migrate` (embedded) |
| Token encryption | AES-256-GCM (stdlib `crypto/aes`) |
| Local dependencies | Docker Compose |

---
name: swiggy-ssh-auth
description: Use when building or modifying the end-to-end SSH + browser authentication flow for swiggy-ssh. Covers host key management, SSH public key identity, browser auth attempts, Swiggy OAuth 2.1 + PKCE, token lifecycle, re-authentication, HTTP auth routes, and database schema.
license: MIT
metadata:
  author: swiggy-ssh team
---

# swiggy-ssh Authentication Flow

This skill documents the complete end-to-end authentication architecture of swiggy-ssh — from a user running `ssh swiggy.dev` to a fully authenticated session backed by a Swiggy OAuth account.

---

## Architecture Overview

Two servers run in parallel:

```
swiggy.dev
├── SSH server  (port 2222)  ← user connects via terminal
└── HTTP server (port 8080)  ← user opens in browser to complete login
```

They share one thing: a **browser auth attempt** stored in Redis. The SSH TUI renders a one-time `/auth/start?attempt=...` URL; the HTTP server validates the attempt, redirects to Swiggy OAuth, exchanges the callback code, and marks the attempt completed for the SSH poll loop.

---

## Concept 1: Server Identity — Host Key

**What it is:** A long-lived asymmetric key pair that identifies the server itself, not any user.

**How it works:**
- On first boot, `internal/presentation/ssh/hostkey.go` generates an Ed25519 key pair and saves it to disk at `SSH_HOST_KEY_PATH` with permissions `0600`
- On every subsequent boot, it loads the saved key
- The key is registered with the SSH library via `serverConfig.AddHostKey(hostSigner)` in `internal/presentation/ssh/server.go:102`
- The SSH library uses it automatically during the handshake — no application code needed

**Why it matters:** Without the host key, the SSH library refuses to start. The client uses it to verify it is talking to the real server (TOFU — Trust On First Use on first connection, then `~/.ssh/known_hosts` on subsequent connections).

**Scaling concern:** Each instance generates its own key. Multiple instances behind a load balancer will present different fingerprints to clients, causing scary "fingerprint changed" warnings. Fix: share the host key via a secrets manager (e.g. AWS Secrets Manager) or inject it as an environment variable at deploy time, so all instances use the same key.

**Key file:** `internal/presentation/ssh/hostkey.go`

---

## Concept 2: User Identity — SSH Public Key + Fingerprint

**What it is:** A returning user's SSH key can identify their durable account. Clients may also connect without a key, or with an unknown key, and receive a guest session.

**How it works:**
- The server allows SSH connections with no client key through a guest keyboard-interactive auth path
- When a public key is provided, the server accepts it for transport and preserves metadata in `ssh.Permissions`
- The server computes a short unique fingerprint from the public key: `ssh.FingerprintSHA256(key)`
- Known fingerprints are stored in `ssh_identities.public_key_fingerprint` in Postgres
- On reconnect with a known, non-revoked key, `ResolveSSHIdentityUseCase.Execute(ctx, input)` looks up the fingerprint → finds the durable SSH identity
- Unknown fingerprints return `ErrNotFound` during initial session resolution
- Revoked known keys return `ErrSSHIdentityRevoked` and are not treated as durable identities
- No-key and unknown-key SSH connections start guest terminal sessions with no `ssh_identity_id`; unknown-key sessions keep the fingerprint on the terminal session for observability
- When an unknown-key user chooses Instamart/login, `RegisterSSHIdentityUseCase` provisions or resolves a durable `SSHIdentity` before issuing a browser auth attempt. This gives the OAuth callback a real `ssh_identity_id` for token persistence.

**Why accept any key?** The SSH key is not an access gate. It is a durable-account lookup hint. Unknown keys begin as guest sessions and are only persisted when the user explicitly starts Swiggy login.

**Fingerprint vs public key:**
- Public key: long raw string (`ssh-ed25519 AAAA...`)
- Fingerprint: short hash (`SHA256:uNiVztksCsDhcc0u9e8Bz...`) — easier to store and compare

**Key files:** `internal/presentation/ssh/server.go:150-158`, `internal/domain/identity/identity.go`

---

## Concept 3: Browser Auth Attempt + PKCE Flow

**What it is:** A short-lived one-time auth attempt that bridges the SSH terminal session to browser-based Swiggy OAuth 2.1 + PKCE. Users no longer type a code manually.

**States:**
```
PENDING   → generated, waiting for /auth/start
CLAIMED   → callback is being consumed; replay is blocked
COMPLETED → token persisted, SSH may proceed
CANCELLED → expired, failed, or user cancelled
```

**Flow:**
1. User connects via SSH and lands on the home screen
2. User chooses Instamart
3. If login is required, server creates a browser auth attempt with a high-entropy attempt token and PKCE `code_verifier`; Redis stores the SHA-256 hash as key and the verifier in the TTL-limited value
4. Server shows the user a direct URL: `https://swiggy.dev/auth/start?attempt=...`
5. The TUI renders an OSC-8 clickable label, the full wrapped URL, and supports `c` copy via OSC-52 where terminals allow it
6. User opens `/auth/start`; HTTP validates the pending attempt and redirects to Swiggy `/auth/authorize`
7. Swiggy redirects back to `/auth/callback?code=...&state=...`
8. HTTP validates state, claims the attempt, exchanges `code + code_verifier` at `/auth/token`, persists the encrypted access token, marks the attempt `COMPLETED`
9. SSH poll loop detects `COMPLETED` → session proceeds

**Key files:** `internal/presentation/ssh/server.go`, `internal/application/auth/ensure_valid_account.go`, `internal/presentation/http/handlers.go`

---

## Concept 4: HTTP Auth Routes

**What it is:** A minimal web server that starts and completes browser auth.

**Routes:**
```
GET  /health  → {"status":"ok"} — health check
GET  /auth/start?attempt=... → validates attempt and redirects to Swiggy OAuth
GET  /auth/callback?code=...&state=... → exchanges code and completes attempt
GET  /login → compatibility/help page; prefer /auth/start
POST /login → compatibility redirect where supported
```

**Swiggy OAuth redirect:**
```
https://mcp.swiggy.com/auth/authorize?
  response_type=code&
  client_id=swiggy-mcp&
  redirect_uri=<PUBLIC_BASE_URL>/auth/callback&
  code_challenge=<S256 challenge>&
  code_challenge_method=S256&
  state=<attempt token>&
  scope=mcp:tools
```

**Token exchange:** `POST https://mcp.swiggy.com/auth/token` with `grant_type=authorization_code`, `code`, `code_verifier`, `client_id`, and exact `redirect_uri`.

**Key file:** `internal/presentation/http/handlers.go`

---

## Concept 5: OAuth Token Lifecycle & Re-authentication

**What it is:** After browser login, the server stores a Swiggy OAuth access token linked to the SSH identity. Tokens expire and must be refreshed via re-authentication.

**Token states:**
```
active              → valid, no action needed
expired             → token TTL passed, trigger re-auth
reconnect_required  → token invalidated externally, trigger re-auth
revoked             → hard block, user must contact support
```

**Re-auth flow (`EnsureValidAccountUseCase.Execute` in `auth/ensure_valid_account.go`):**
```
User connects
     ↓
Check token status
      ↓
active             → "Welcome back!"
expired/reconnect  → show new browser auth URL in terminal
                     user logs in browser again via PKCE
                     new token saved → session continues
revoked            → "Access revoked. Contact support."
```

**Current state:** `SWIGGY_PROVIDER=mock` short-circuits browser auth for local tests. `SWIGGY_PROVIDER=swiggy` uses Swiggy OAuth 2.1 + PKCE with default client id `swiggy-mcp`.

**Key file:** `internal/application/auth/ensure_valid_account.go`

---

## Concept 6: Database Schema

Four linked tables in Postgres:

```
ssh_identities             (durable app principal)
  id
  public_key_fingerprint   ← unique, indexed, used for lookup
  public_key               ← full public key stored
  label                    ← e.g. "MacBook Pro"
  first_seen_at, last_seen_at
  revoked_at               ← set when key is revoked (e.g. lost laptop)

oauth_accounts             (one SSH identity → one account per provider)
  id, ssh_identity_id → ssh_identities.id
  provider                 ← e.g. "swiggy"
  provider_user_id
  encrypted_access_token   ← AES-256-GCM encrypted, never stored in plaintext
  token_expires_at, scopes, status

terminal_sessions          (one SSH identity → many durable sessions; guests have no identity id)
  id
  ssh_identity_id → ssh_identities.id
  ssh_fingerprint
  current_screen, selected_address_id
  created_at, last_seen_at, ended_at
```

**Key files:** `internal/infrastructure/persistence/postgres/migrations/000001_auth_identity.up.sql`, `internal/infrastructure/persistence/postgres/postgres.go`

---

## Concept 7: Token Encryption

**What it is:** OAuth access tokens are encrypted with AES-256-GCM before being written to the database, and decrypted in memory when needed.

**Why:** If the database is compromised, encrypted tokens are useless without the encryption key. The key is stored separately.

**How:**
- Encryption key: 32-byte key, base64url-encoded, set via `TOKEN_ENCRYPTION_KEY` environment variable
- Local dev fallback: `DefaultDevTokenEncryptionKey` in `config.go:21` — **publicly known, never use in production**
- Decrypt only happens in memory at runtime inside the Postgres persistence adapter

**Key files:** `internal/infrastructure/crypto/aes.go`, `internal/platform/config/config.go`

---

## Complete End-to-End Flow

```
1. ssh swiggy.dev
        ↓
2. SSH handshake
   - Client verifies server host key fingerprint (TOFU)
   - Encrypted tunnel established
        ↓
3. Server accepts SSH with or without a client public key
   - If a key is provided, computes its fingerprint
   - Looks up fingerprint in DB → known key resolves durable SSH identity
   - Unknown key or no key → guest session at first
        ↓
4. User chooses Instamart from the terminal home screen
        ↓
5. If login is required and a public key is present, server provisions/resolves a durable `SSHIdentity`
   - Creates browser auth attempt with PKCE verifier, TTL default 10 minutes
   - Shows user: "Open swiggy.dev/auth/start?attempt=..."
   - Starts polling Redis every 2 seconds
        ↓
6. User opens browser → visits /auth/start
   - Redirects to Swiggy OAuth authorize endpoint
   - Swiggy redirects back to /auth/callback with code + state
   - Server exchanges code using stored PKCE verifier
   - Token is encrypted and persisted in oauth_accounts
        ↓
7. Poll loop detects COMPLETED
   - `EnsureValidAccountUseCase.Execute` checks the stored OAuth account and re-auths if expired
        ↓
8. Terminal proceeds to the current Instamart placeholder
```

---

## What Is Not Yet Implemented

| Missing piece | Where to add it |
|---------------|----------------|
| Real Instamart API calls using stored token | `internal/infrastructure/provider/swiggy/client.go` |
| Refresh tokens | Not available in Swiggy v1.0; re-run OAuth on 401/expiry |
| Full `LoginCode*` naming cleanup | Auth domain/cache/config follow-up |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SSH_ADDR` | `:2222` | SSH server listen address |
| `HTTP_ADDR` | `:8080` | HTTP server listen address |
| `SSH_HOST_KEY_PATH` | `.local/ssh_host_ed25519_key` | Path to host key file |
| `TOKEN_ENCRYPTION_KEY` | hardcoded dev key | AES-256 key (base64url, 32 bytes) — **must be set in production** |
| `DATABASE_URL` | `postgres://...localhost` | Postgres connection string |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis connection string |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | Base URL shown to users in login prompt |
| `LOGIN_CODE_TTL` | `10m` | How long a browser auth attempt is valid |
| `SWIGGY_PROVIDER` | `mock` | Set to `swiggy` for real Swiggy OAuth |
| `SWIGGY_CLIENT_ID` | `swiggy-mcp` | OAuth client id used for Swiggy auth |
| `SWIGGY_AUTH_AUTHORIZE_URL` | `https://mcp.swiggy.com/auth/authorize` | Swiggy OAuth authorize endpoint |
| `SWIGGY_AUTH_TOKEN_URL` | `https://mcp.swiggy.com/auth/token` | Swiggy OAuth token endpoint |
| `SWIGGY_AUTH_SCOPES` | `mcp:tools` | OAuth scopes requested |
| `APP_ENV` | `local` | Environment name (`local`, `production`, etc.) |

---

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Multiple servers show different host key fingerprints | Share host key via secrets manager — all instances must use the same key |
| `TOKEN_ENCRYPTION_KEY` not set in production | Set to a securely generated 32-byte base64url key — the dev default is public |
| Login URL cannot be copied | Use the wrapped fallback URL; `c` attempts OSC-52 clipboard copy but terminal support varies |
| User sees "fingerprint changed" warning | Old host key on disk differs from new one — delete `.local/ssh_host_ed25519_key` or restore the correct key |
| Token expired but re-auth not triggered | Check `ValidateTokenForUse` — token status must be `expired` or `reconnect_required` to trigger re-auth |
| Unknown SSH key creates a persistent identity too early | Initial resolve must return guest; durable SSH identity creation happens only when the user explicitly starts login |
| OAuth callback has empty SSH identity id | Browser auth attempts must be issued only after durable identity provisioning; callback rejects empty-identity attempts defensively |

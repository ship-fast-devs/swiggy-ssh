package auth

import (
	"context"
	"errors"
	"time"
)

// Service defines authentication behavior for SSH sessions.
type Service interface {
	Authenticate(subject string) (Identity, error)
}

// Identity represents a successfully authenticated principal.
type Identity struct {
	SSHIdentityID string
}

// OAuthAccount represents provider linkage and token state.
type OAuthAccount struct {
	ID             string
	SSHIdentityID  string
	Provider       string
	ProviderUserID *string
	// AccessToken holds the plaintext access token value.
	// The store layer encrypts this at rest (AES-256-GCM) and decrypts on read.
	// Never log, render, or expose this value to clients.
	AccessToken    string
	TokenExpiresAt *time.Time
	Scopes         []string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Repository is the auth persistence boundary.
type Repository interface {
	UpsertOAuthAccount(ctx context.Context, account OAuthAccount) (OAuthAccount, error)
	FindOAuthAccountBySSHIdentityAndProvider(ctx context.Context, sshIdentityID, provider string) (OAuthAccount, error)
}

// BrowserAuthAttempt status values.
const (
	AuthAttemptStatusPending   = "pending"
	AuthAttemptStatusClaimed   = "claimed"
	AuthAttemptStatusCompleted = "completed"
	AuthAttemptStatusCancelled = "cancelled"
	// "expired" is implicit: TTL eviction; GetAuthAttempt returns ErrAuthAttemptNotFound when key is gone.

	// Deprecated login-code names kept for compatibility during the auth-attempt migration.
	LoginCodeStatusPending   = AuthAttemptStatusPending
	LoginCodeStatusCompleted = AuthAttemptStatusCompleted
	LoginCodeStatusCancelled = AuthAttemptStatusCancelled
)

// BrowserAuthAttempt is the domain record for a short-lived terminal-to-browser auth bridge.
type BrowserAuthAttempt struct {
	TokenHash         string // SHA-256 hex of raw attempt token; raw token is never stored
	CodeHash          string // Deprecated compatibility alias of TokenHash.
	CodeVerifier      string // OAuth PKCE private verifier; never log, render, or expose to clients.
	SSHIdentityID     string
	TerminalSessionID string
	Status            string // pending | claimed | completed | cancelled
	ExpiresAt         time.Time
	CreatedAt         time.Time
}

// LoginCode is kept as a compatibility alias while callers move to BrowserAuthAttempt.
type LoginCode = BrowserAuthAttempt

// OAuthAccountStatus values.
const (
	OAuthAccountStatusActive            = "active"
	OAuthAccountStatusExpired           = "expired"
	OAuthAccountStatusReconnectRequired = "reconnect_required"
	OAuthAccountStatusRevoked           = "revoked"
)

// ErrTokenExpired is returned when an OAuth token has passed its expiry time.
var ErrTokenExpired = errors.New("oauth token expired")

// ErrTokenReconnectRequired is returned when the account needs re-authentication.
var ErrTokenReconnectRequired = errors.New("oauth account requires reconnect")

// ErrTokenRevoked is returned when the account has been revoked.
var ErrTokenRevoked = errors.New("oauth account revoked")

// TokenEncryptor is the port for symmetric token encryption at rest.
type TokenEncryptor interface {
	// Encrypt returns an opaque ciphertext string safe to store in the database.
	// The plaintext token must never be stored or logged after this call.
	Encrypt(ctx context.Context, plaintext string) (ciphertext string, err error)

	// Decrypt recovers the plaintext token from a stored ciphertext string.
	// Returns an error if the ciphertext is invalid or tampered.
	Decrypt(ctx context.Context, ciphertext string) (plaintext string, err error)
}

// ValidateTokenForUse checks that the account is active and the token is not expired.
// Returns a typed domain error (ErrTokenExpired, ErrTokenReconnectRequired, ErrTokenRevoked)
// or nil when the token is valid for provider calls.
// now is the caller's current time (use s.now() in services, time.Now().UTC() in tests).
func ValidateTokenForUse(account OAuthAccount, now time.Time) error {
	switch account.Status {
	case OAuthAccountStatusRevoked:
		return ErrTokenRevoked
	case OAuthAccountStatusReconnectRequired:
		return ErrTokenReconnectRequired
	case OAuthAccountStatusExpired:
		return ErrTokenExpired
	}
	if account.Status == OAuthAccountStatusActive && account.TokenExpiresAt == nil {
		return ErrTokenReconnectRequired
	}
	// Also check wall-clock expiry regardless of stored status.
	if account.TokenExpiresAt != nil && now.After(*account.TokenExpiresAt) {
		return ErrTokenExpired
	}
	return nil
}

// ErrOAuthAccountNotFound is returned by Repository when no OAuth account exists for the given SSH identity/provider pair.
var ErrOAuthAccountNotFound = errors.New("oauth account not found")

// ErrSSHIdentityRequired is returned when account lookup or persistence is requested without a durable SSH identity.
var ErrSSHIdentityRequired = errors.New("oauth account requires a durable ssh identity")

// ErrAccountRevoked is surfaced to callers when the OAuth account has been revoked.
var ErrAccountRevoked = errors.New("oauth account has been revoked")

// ErrAuthAttemptNotFound is returned when the attempt key does not exist (expired or never created).
var ErrAuthAttemptNotFound = errors.New("auth attempt not found or expired")

// ErrAuthAttemptAlreadyUsed is returned when attempting to complete an attempt that is not pending.
var ErrAuthAttemptAlreadyUsed = errors.New("auth attempt already used or cancelled")

// ErrBrowserAuthProviderUnavailable is returned when real provider login has not been configured.
var ErrBrowserAuthProviderUnavailable = errors.New("browser auth provider unavailable")

// ErrBrowserAuthProviderCallback is returned when a provider callback cannot produce credentials.
var ErrBrowserAuthProviderCallback = errors.New("browser auth provider callback failed")

// Deprecated error aliases kept for compatibility during the auth-attempt migration.
var ErrLoginCodeNotFound = ErrAuthAttemptNotFound
var ErrLoginCodeAlreadyUsed = ErrAuthAttemptAlreadyUsed

// BrowserAuthAttemptService is the application-layer port for the browser auth-attempt lifecycle.
type BrowserAuthAttemptService interface {
	// IssueAuthAttempt generates an opaque high-entropy token, hashes it, persists the
	// BrowserAuthAttempt record, and returns the raw token (shown only in the direct login URL).
	IssueAuthAttempt(ctx context.Context, sshIdentityID, terminalSessionID string) (rawToken string, record BrowserAuthAttempt, err error)

	// GetAuthAttempt looks up the record by raw token. Returns ErrAuthAttemptNotFound if
	// the key has expired or was never issued.
	GetAuthAttempt(ctx context.Context, rawToken string) (BrowserAuthAttempt, error)

	// CompleteAuthAttempt atomically marks the attempt completed. Returns
	// ErrAuthAttemptNotFound if expired, ErrAuthAttemptAlreadyUsed if not pending.
	CompleteAuthAttempt(ctx context.Context, rawToken string) error

	// ClaimAuthAttempt atomically marks a pending attempt claimed and returns it.
	// Callers must perform credential writes only after this succeeds.
	ClaimAuthAttempt(ctx context.Context, rawToken string) (BrowserAuthAttempt, error)

	// CompleteClaimedAuthAttempt atomically marks a claimed attempt completed after credentials are persisted.
	CompleteClaimedAuthAttempt(ctx context.Context, rawToken string) error

	// CancelClaimedAuthAttempt atomically marks a claimed attempt cancelled after provider failure.
	CancelClaimedAuthAttempt(ctx context.Context, rawToken string) error

	// CancelAuthAttempt marks the attempt cancelled (user dismissed the TUI prompt).
	// Returns ErrAuthAttemptNotFound if expired.
	CancelAuthAttempt(ctx context.Context, rawToken string) error
}

// BrowserAuthStartInput describes provider login redirect construction.
type BrowserAuthStartInput struct {
	State        string
	CallbackURL  string
	CodeVerifier string
}

type BrowserAuthStartOutput struct {
	RedirectURL string
}

// BrowserAuthProvider starts provider-hosted browser authentication.
type BrowserAuthProvider interface {
	StartBrowserAuth(ctx context.Context, input BrowserAuthStartInput) (BrowserAuthStartOutput, error)
}

// BrowserAuthCallbackInput carries provider callback values after state validation.
type BrowserAuthCallbackInput struct {
	State        string
	Code         string
	CallbackURL  string
	CodeVerifier string
}

// BrowserAuthCredentials are provider credentials ready for persistence.
type BrowserAuthCredentials struct {
	ProviderUserID *string
	AccessToken    string
	TokenExpiresAt *time.Time
	Scopes         []string
}

// BrowserAuthCallbackProvider exchanges provider callback data for credentials.
type BrowserAuthCallbackProvider interface {
	ExchangeBrowserAuthCallback(ctx context.Context, input BrowserAuthCallbackInput) (BrowserAuthCredentials, error)
}

// LoginCodeService is kept for compatibility while the app migrates names.
type LoginCodeService interface {
	BrowserAuthAttemptService

	IssueLoginCode(ctx context.Context, sshIdentityID, terminalSessionID string) (rawCode string, record LoginCode, err error)

	// GetLoginCode looks up the record by raw code. Returns ErrLoginCodeNotFound if
	// the key has expired or was never issued.
	GetLoginCode(ctx context.Context, rawCode string) (LoginCode, error)

	// CompleteLoginCode atomically marks the code completed. Returns
	// ErrLoginCodeNotFound if expired, ErrLoginCodeAlreadyUsed if not pending.
	CompleteLoginCode(ctx context.Context, rawCode string) error

	// CancelLoginCode marks the code cancelled (user dismissed the TUI prompt).
	// Returns ErrLoginCodeNotFound if expired.
	CancelLoginCode(ctx context.Context, rawCode string) error
}

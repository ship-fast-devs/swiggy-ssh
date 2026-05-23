package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	domainauth "swiggy-ssh/internal/domain/auth"
)

type OAuthAccount = domainauth.OAuthAccount
type Repository = domainauth.Repository
type BrowserAuthAttempt = domainauth.BrowserAuthAttempt
type BrowserAuthAttemptService = domainauth.BrowserAuthAttemptService
type BrowserAuthProvider = domainauth.BrowserAuthProvider
type BrowserAuthCallbackProvider = domainauth.BrowserAuthCallbackProvider
type BrowserAuthStartInput = domainauth.BrowserAuthStartInput
type BrowserAuthStartOutput = domainauth.BrowserAuthStartOutput
type BrowserAuthCallbackInput = domainauth.BrowserAuthCallbackInput
type BrowserAuthCredentials = domainauth.BrowserAuthCredentials
type LoginCode = domainauth.LoginCode
type LoginCodeService = domainauth.LoginCodeService
type TokenEncryptor = domainauth.TokenEncryptor

const (
	AuthAttemptStatusPending   = domainauth.AuthAttemptStatusPending
	AuthAttemptStatusClaimed   = domainauth.AuthAttemptStatusClaimed
	AuthAttemptStatusCompleted = domainauth.AuthAttemptStatusCompleted
	AuthAttemptStatusCancelled = domainauth.AuthAttemptStatusCancelled

	OAuthAccountStatusActive            = domainauth.OAuthAccountStatusActive
	OAuthAccountStatusExpired           = domainauth.OAuthAccountStatusExpired
	OAuthAccountStatusReconnectRequired = domainauth.OAuthAccountStatusReconnectRequired
	OAuthAccountStatusRevoked           = domainauth.OAuthAccountStatusRevoked
)

const (
	LoginCodeStatusPending   = domainauth.LoginCodeStatusPending
	LoginCodeStatusCompleted = domainauth.LoginCodeStatusCompleted
	LoginCodeStatusCancelled = domainauth.LoginCodeStatusCancelled
)

var ErrTokenExpired = domainauth.ErrTokenExpired
var ErrTokenReconnectRequired = domainauth.ErrTokenReconnectRequired
var ErrTokenRevoked = domainauth.ErrTokenRevoked
var ErrOAuthAccountNotFound = domainauth.ErrOAuthAccountNotFound
var ErrAccountRevoked = domainauth.ErrAccountRevoked
var ErrAuthAttemptNotFound = domainauth.ErrAuthAttemptNotFound
var ErrAuthAttemptAlreadyUsed = domainauth.ErrAuthAttemptAlreadyUsed
var ErrBrowserAuthProviderUnavailable = domainauth.ErrBrowserAuthProviderUnavailable
var ErrBrowserAuthProviderCallback = domainauth.ErrBrowserAuthProviderCallback
var ErrLoginCodeNotFound = domainauth.ErrLoginCodeNotFound
var ErrLoginCodeAlreadyUsed = domainauth.ErrLoginCodeAlreadyUsed
var ErrSSHIdentityRequired = domainauth.ErrSSHIdentityRequired

var ValidateTokenForUse = domainauth.ValidateTokenForUse

const (
	MockProvider = "swiggy"
	mockTokenTTL = 24 * time.Hour
)

// EnsureValidAccountInput contains the account context needed to validate or establish an OAuth account.
type EnsureValidAccountInput struct {
	SSHIdentityID      string
	AllowFirstAuth     bool
	Reauth             func(ctx context.Context) error
	AuthAttemptService BrowserAuthAttemptService
	TerminalSessionID  string
	PublicBaseURL      string
}

// EnsureValidAccountOutput tells the caller what happened so it can render the right terminal message.
type EnsureValidAccountOutput struct {
	Account          OAuthAccount
	IsFirstAuth      bool // true when a new account record was created
	WasReauth        bool // true when the user went through a re-auth loop
	AuthRequired     bool
	LoginURL         string
	AuthAttemptToken string
}

// EnsureValidAccountUseCase orchestrates the OAuth account lifecycle for a terminal session.
// It is client-agnostic: SSH, web, and future clients all call the same service.
type EnsureValidAccountUseCase struct {
	repo Repository
	now  func() time.Time
}

// NewEnsureValidAccountUseCase constructs an EnsureValidAccountUseCase with its dependencies.
func NewEnsureValidAccountUseCase(repo Repository) *EnsureValidAccountUseCase {
	return &EnsureValidAccountUseCase{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// Execute checks or establishes a valid OAuth account for input.SSHIdentityID.
//
// Behaviour:
//   - If SSHIdentityID is empty → returns ErrSSHIdentityRequired without querying/persisting accounts.
//   - If no account exists and AllowFirstAuth is true with AuthAttemptService → returns AuthRequired with a direct login URL.
//   - If no account exists and AllowFirstAuth is true without AuthAttemptService → creates a mock active account and returns IsFirstAuth=true.
//   - If no account exists and AllowFirstAuth is false → returns ErrOAuthAccountNotFound.
//   - If account exists and is valid → returns the account as-is.
//   - If account is expired or reconnect_required → calls reauth to issue a new login code,
//     waits for completion (via reauth callback), then refreshes the account record.
//   - If account is revoked → returns ErrAccountRevoked.
//
// input.Reauth is a callback the caller provides to run the re-auth login-code flow.
// It should issue a new login code, show it to the user, poll for completion,
// and return nil on success or an error (including context cancellation) on failure.
func (s *EnsureValidAccountUseCase) Execute(ctx context.Context, input EnsureValidAccountInput) (EnsureValidAccountOutput, error) {
	if input.SSHIdentityID == "" {
		return EnsureValidAccountOutput{}, ErrSSHIdentityRequired
	}

	account, err := s.repo.FindOAuthAccountBySSHIdentityAndProvider(ctx, input.SSHIdentityID, MockProvider)
	if err != nil {
		if errors.Is(err, ErrOAuthAccountNotFound) {
			if !input.AllowFirstAuth {
				return EnsureValidAccountOutput{}, ErrOAuthAccountNotFound
			}
			if input.AuthAttemptService != nil {
				return s.issueAuthRequired(ctx, input)
			}
			newAccount, createErr := s.createMockAccount(ctx, input.SSHIdentityID)
			if createErr != nil {
				return EnsureValidAccountOutput{}, fmt.Errorf("create mock oauth account: %w", createErr)
			}
			return EnsureValidAccountOutput{Account: newAccount, IsFirstAuth: true}, nil
		}
		return EnsureValidAccountOutput{}, fmt.Errorf("find oauth account: %w", err)
	}

	if err := ValidateTokenForUse(account, s.now()); err != nil {
		switch {
		case errors.Is(err, ErrTokenRevoked):
			return EnsureValidAccountOutput{}, ErrAccountRevoked
		case errors.Is(err, ErrTokenExpired), errors.Is(err, ErrTokenReconnectRequired):
			if input.AuthAttemptService != nil {
				return s.issueAuthRequired(ctx, input)
			}
			if input.Reauth == nil {
				return EnsureValidAccountOutput{}, err
			}
			if reauthErr := input.Reauth(ctx); reauthErr != nil {
				return EnsureValidAccountOutput{}, fmt.Errorf("reauth: %w", reauthErr)
			}
			refreshed, refreshErr := s.refreshMockAccount(ctx, input.SSHIdentityID, account)
			if refreshErr != nil {
				return EnsureValidAccountOutput{}, fmt.Errorf("refresh mock account after reauth: %w", refreshErr)
			}
			return EnsureValidAccountOutput{Account: refreshed, WasReauth: true}, nil
		default:
			return EnsureValidAccountOutput{}, err
		}
	}

	return EnsureValidAccountOutput{Account: account}, nil
}

func (s *EnsureValidAccountUseCase) RequireReconnect(ctx context.Context, sshIdentityID string) error {
	if sshIdentityID == "" {
		return ErrSSHIdentityRequired
	}
	account, err := s.repo.FindOAuthAccountBySSHIdentityAndProvider(ctx, sshIdentityID, MockProvider)
	if err != nil {
		return fmt.Errorf("find oauth account: %w", err)
	}
	if account.Status == OAuthAccountStatusRevoked {
		return ErrAccountRevoked
	}
	account.Status = OAuthAccountStatusReconnectRequired
	if _, err := s.repo.UpsertOAuthAccount(ctx, account); err != nil {
		return fmt.Errorf("mark oauth account reconnect required: %w", err)
	}
	return nil
}

func (s *EnsureValidAccountUseCase) issueAuthRequired(ctx context.Context, input EnsureValidAccountInput) (EnsureValidAccountOutput, error) {
	rawAttempt, _, err := input.AuthAttemptService.IssueAuthAttempt(ctx, input.SSHIdentityID, input.TerminalSessionID)
	if err != nil {
		return EnsureValidAccountOutput{}, fmt.Errorf("issue auth attempt: %w", err)
	}
	return EnsureValidAccountOutput{
		AuthRequired:     true,
		LoginURL:         input.PublicBaseURL + "/auth/start?attempt=" + url.QueryEscape(rawAttempt),
		AuthAttemptToken: rawAttempt,
	}, nil
}

// createMockAccount creates a new mock OAuth account for a first-time user.
// The mock token is a placeholder — no real Swiggy credentials.
func (s *EnsureValidAccountUseCase) createMockAccount(ctx context.Context, sshIdentityID string) (OAuthAccount, error) {
	now := s.now()
	expiresAt := now.Add(mockTokenTTL)

	return s.repo.UpsertOAuthAccount(ctx, OAuthAccount{
		SSHIdentityID:  sshIdentityID,
		Provider:       MockProvider,
		AccessToken:    mockAccessToken(sshIdentityID),
		TokenExpiresAt: &expiresAt,
		Scopes:         []string{"profile:read"},
		Status:         OAuthAccountStatusActive,
	})
}

// refreshMockAccount updates an existing account with a new mock token after re-auth.
func (s *EnsureValidAccountUseCase) refreshMockAccount(ctx context.Context, sshIdentityID string, existing OAuthAccount) (OAuthAccount, error) {
	now := s.now()
	expiresAt := now.Add(mockTokenTTL)

	existing.AccessToken = mockAccessToken(sshIdentityID)
	existing.TokenExpiresAt = &expiresAt
	existing.Status = OAuthAccountStatusActive
	return s.repo.UpsertOAuthAccount(ctx, existing)
}

// mockAccessToken returns the placeholder token value for a given SSH identity ID.
// Mock tokens are never real Swiggy credentials.
func mockAccessToken(sshIdentityID string) string {
	return fmt.Sprintf("mock-token-%s", sshIdentityID)
}

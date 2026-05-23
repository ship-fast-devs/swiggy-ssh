package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"swiggy-ssh/internal/application/auth"
)

// --- Mock repo ---

type mockAuthRepo struct {
	account   auth.OAuthAccount
	findErr   error
	findCalls int
	upserted  auth.OAuthAccount
	upsertErr error
}

type mockAttemptService struct{}

func (m *mockAttemptService) IssueAuthAttempt(_ context.Context, sshIdentityID, terminalSessionID string) (string, auth.BrowserAuthAttempt, error) {
	rawToken := "opaque-attempt-token"
	h := sha256.Sum256([]byte(rawToken))
	return rawToken, auth.BrowserAuthAttempt{
		TokenHash:         hex.EncodeToString(h[:]),
		SSHIdentityID:     sshIdentityID,
		TerminalSessionID: terminalSessionID,
		Status:            auth.AuthAttemptStatusPending,
	}, nil
}

func (m *mockAttemptService) GetAuthAttempt(_ context.Context, _ string) (auth.BrowserAuthAttempt, error) {
	return auth.BrowserAuthAttempt{}, nil
}

func (m *mockAttemptService) CompleteAuthAttempt(_ context.Context, _ string) error { return nil }

func (m *mockAttemptService) ClaimAuthAttempt(_ context.Context, _ string) (auth.BrowserAuthAttempt, error) {
	return auth.BrowserAuthAttempt{Status: auth.AuthAttemptStatusClaimed}, nil
}

func (m *mockAttemptService) CompleteClaimedAuthAttempt(_ context.Context, _ string) error {
	return nil
}

func (m *mockAttemptService) CancelClaimedAuthAttempt(_ context.Context, _ string) error {
	return nil
}

func (m *mockAttemptService) CancelAuthAttempt(_ context.Context, _ string) error { return nil }

func (r *mockAuthRepo) FindOAuthAccountBySSHIdentityAndProvider(_ context.Context, _, _ string) (auth.OAuthAccount, error) {
	r.findCalls++
	return r.account, r.findErr
}

func (r *mockAuthRepo) UpsertOAuthAccount(_ context.Context, a auth.OAuthAccount) (auth.OAuthAccount, error) {
	r.upserted = a
	return a, r.upsertErr
}

// --- Tests ---

func TestEnsureValidAccountMissingAccountWithoutFirstAuthReturnsNotFound(t *testing.T) {
	repo := &mockAuthRepo{findErr: auth.ErrOAuthAccountNotFound}
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	_, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{
		SSHIdentityID: "identity-1",
	})
	if !errors.Is(err, auth.ErrOAuthAccountNotFound) {
		t.Fatalf("expected ErrOAuthAccountNotFound, got %v", err)
	}
}

func TestEnsureValidAccountFirstAuth(t *testing.T) {
	repo := &mockAuthRepo{findErr: auth.ErrOAuthAccountNotFound}
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	result, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{
		SSHIdentityID:  "identity-1",
		AllowFirstAuth: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsFirstAuth {
		t.Fatal("expected IsFirstAuth=true")
	}
	if result.Account.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected active status, got %s", result.Account.Status)
	}
	if repo.upserted.SSHIdentityID != "identity-1" {
		t.Fatalf("expected upserted identity-1, got %s", repo.upserted.SSHIdentityID)
	}
}

func TestEnsureValidAccountMissingAccountReturnsDirectAuthURL(t *testing.T) {
	repo := &mockAuthRepo{findErr: auth.ErrOAuthAccountNotFound}
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	result, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{
		SSHIdentityID:      "identity-1",
		AllowFirstAuth:     true,
		AuthAttemptService: &mockAttemptService{},
		TerminalSessionID:  "session-1",
		PublicBaseURL:      "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AuthRequired {
		t.Fatal("expected AuthRequired=true")
	}
	if result.LoginURL != "http://localhost:8080/auth/start?attempt=opaque-attempt-token" {
		t.Fatalf("unexpected login URL: %s", result.LoginURL)
	}
	if result.AuthAttemptToken != "opaque-attempt-token" {
		t.Fatalf("unexpected auth attempt token: %s", result.AuthAttemptToken)
	}
}

func TestEnsureValidAccountGuestAuthAttemptReturnsControlledError(t *testing.T) {
	repo := &mockAuthRepo{}
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	_, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{
		AllowFirstAuth:     true,
		AuthAttemptService: &mockAttemptService{},
		TerminalSessionID:  "session-1",
		PublicBaseURL:      "http://localhost:8080",
	})
	if !errors.Is(err, auth.ErrSSHIdentityRequired) {
		t.Fatalf("expected ErrSSHIdentityRequired, got %v", err)
	}
	if repo.findCalls != 0 {
		t.Fatalf("expected no oauth lookup for guest, got %d", repo.findCalls)
	}
}

func TestEnsureValidAccountGuestWithoutAuthAttemptReturnsControlledError(t *testing.T) {
	repo := &mockAuthRepo{}
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	_, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{AllowFirstAuth: true})
	if !errors.Is(err, auth.ErrSSHIdentityRequired) {
		t.Fatalf("expected ErrSSHIdentityRequired, got %v", err)
	}
	if repo.findCalls != 0 {
		t.Fatalf("expected no oauth lookup for guest, got %d", repo.findCalls)
	}
}

func TestEnsureValidAccountReturningValid(t *testing.T) {
	future := time.Now().UTC().Add(1 * time.Hour)
	repo := &mockAuthRepo{
		account: auth.OAuthAccount{
			Status:         auth.OAuthAccountStatusActive,
			TokenExpiresAt: &future,
			AccessToken:    "mock-token-user-1",
		},
	}
	reauthCalled := false
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	result, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{SSHIdentityID: "identity-1", Reauth: func(_ context.Context) error {
		reauthCalled = true
		return nil
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsFirstAuth || result.WasReauth {
		t.Fatal("expected no first-auth or reauth for valid returning user")
	}
	if reauthCalled {
		t.Fatal("reauth callback must not be called for valid token")
	}
}

func TestEnsureValidAccountExpiredTriggersReauth(t *testing.T) {
	past := time.Now().UTC().Add(-1 * time.Hour)
	repo := &mockAuthRepo{
		account: auth.OAuthAccount{
			Status:         auth.OAuthAccountStatusActive,
			TokenExpiresAt: &past,
			AccessToken:    "old-token",
		},
	}
	reauthCalled := false
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	result, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{SSHIdentityID: "identity-1", Reauth: func(_ context.Context) error {
		reauthCalled = true
		return nil
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reauthCalled {
		t.Fatal("expected reauth callback to be called")
	}
	if !result.WasReauth {
		t.Fatal("expected WasReauth=true")
	}
	if result.Account.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected refreshed account to be active, got %s", result.Account.Status)
	}
}

func TestEnsureValidAccountExpiredCanReturnDirectAuthURL(t *testing.T) {
	past := time.Now().UTC().Add(-1 * time.Hour)
	repo := &mockAuthRepo{
		account: auth.OAuthAccount{
			Status:         auth.OAuthAccountStatusActive,
			TokenExpiresAt: &past,
			AccessToken:    "old-token",
		},
	}
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	result, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{
		SSHIdentityID:      "identity-1",
		AuthAttemptService: &mockAttemptService{},
		TerminalSessionID:  "session-1",
		PublicBaseURL:      "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AuthRequired {
		t.Fatal("expected AuthRequired=true")
	}
	if result.LoginURL != "http://localhost:8080/auth/start?attempt=opaque-attempt-token" {
		t.Fatalf("unexpected login URL: %s", result.LoginURL)
	}
}

func TestEnsureValidAccountReconnectRequiredTriggersReauth(t *testing.T) {
	repo := &mockAuthRepo{
		account: auth.OAuthAccount{
			Status:      auth.OAuthAccountStatusReconnectRequired,
			AccessToken: "stale-token",
		},
	}
	reauthCalled := false
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	result, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{SSHIdentityID: "identity-1", Reauth: func(_ context.Context) error {
		reauthCalled = true
		return nil
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reauthCalled {
		t.Fatal("expected reauth for reconnect_required")
	}
	if !result.WasReauth {
		t.Fatal("expected WasReauth=true")
	}
}

func TestRequireReconnectMarksExistingAccountAndPreservesToken(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour)
	repo := &mockAuthRepo{account: auth.OAuthAccount{
		ID:             "account-1",
		SSHIdentityID:  "identity-1",
		Provider:       auth.MockProvider,
		AccessToken:    "real-token-value",
		TokenExpiresAt: &future,
		Scopes:         []string{"mcp:tools"},
		Status:         auth.OAuthAccountStatusActive,
	}}
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	if err := useCase.RequireReconnect(context.Background(), "identity-1"); err != nil {
		t.Fatalf("require reconnect: %v", err)
	}
	if repo.upserted.Status != auth.OAuthAccountStatusReconnectRequired {
		t.Fatalf("expected reconnect_required, got %s", repo.upserted.Status)
	}
	if repo.upserted.AccessToken != "real-token-value" {
		t.Fatal("access token must be preserved")
	}
	if repo.upserted.TokenExpiresAt == nil || !repo.upserted.TokenExpiresAt.Equal(future) {
		t.Fatalf("token expiry must be preserved, got %#v", repo.upserted.TokenExpiresAt)
	}
}

func TestEnsureValidAccountRevokedReturnsError(t *testing.T) {
	repo := &mockAuthRepo{
		account: auth.OAuthAccount{
			Status:      auth.OAuthAccountStatusRevoked,
			AccessToken: "revoked-token",
		},
	}
	reauthCalled := false
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	_, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{SSHIdentityID: "identity-1", Reauth: func(_ context.Context) error {
		reauthCalled = true
		return nil
	}})
	if !errors.Is(err, auth.ErrAccountRevoked) {
		t.Fatalf("expected ErrAccountRevoked, got %v", err)
	}
	if reauthCalled {
		t.Fatal("reauth callback must not be called for revoked account")
	}
}

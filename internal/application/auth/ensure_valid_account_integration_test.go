package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"swiggy-ssh/internal/application/auth"
)

// repoKey is the composite key for the in-memory auth repo.
type repoKey struct{ sshIdentityID, provider string }

// intAuthRepo is a simple in-memory repository for integration testing.
type intAuthRepo struct {
	accounts map[repoKey]auth.OAuthAccount
}

func newIntAuthRepo() *intAuthRepo {
	return &intAuthRepo{accounts: make(map[repoKey]auth.OAuthAccount)}
}

func (r *intAuthRepo) UpsertOAuthAccount(_ context.Context, a auth.OAuthAccount) (auth.OAuthAccount, error) {
	if a.SSHIdentityID == "" {
		return auth.OAuthAccount{}, errors.New("upsert: sshIdentityID required")
	}
	r.accounts[repoKey{a.SSHIdentityID, a.Provider}] = a
	return a, nil
}

func (r *intAuthRepo) FindOAuthAccountBySSHIdentityAndProvider(_ context.Context, sshIdentityID, provider string) (auth.OAuthAccount, error) {
	a, ok := r.accounts[repoKey{sshIdentityID, provider}]
	if !ok {
		return auth.OAuthAccount{}, auth.ErrOAuthAccountNotFound
	}
	return a, nil
}

func TestAuthIntegrationFirstTimeIdentity(t *testing.T) {
	repo := newIntAuthRepo()
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	result, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{
		SSHIdentityID:  "identity-new",
		AllowFirstAuth: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsFirstAuth {
		t.Fatal("expected IsFirstAuth for new user")
	}
	if result.Account.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected active, got %s", result.Account.Status)
	}
	// Account persisted
	found, err := repo.FindOAuthAccountBySSHIdentityAndProvider(context.Background(), "identity-new", auth.MockProvider)
	if err != nil {
		t.Fatalf("find after first auth: %v", err)
	}
	if found.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected active in repo, got %s", found.Status)
	}
}

func TestAuthIntegrationReturningValidIdentity(t *testing.T) {
	repo := newIntAuthRepo()
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	// Seed a valid account
	future := time.Now().UTC().Add(2 * time.Hour)
	repo.accounts[repoKey{"identity-returning", auth.MockProvider}] = auth.OAuthAccount{
		SSHIdentityID:  "identity-returning",
		Provider:       auth.MockProvider,
		Status:         auth.OAuthAccountStatusActive,
		AccessToken:    "valid-token",
		TokenExpiresAt: &future,
	}

	reauthCalled := false
	result, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{SSHIdentityID: "identity-returning", Reauth: func(_ context.Context) error {
		reauthCalled = true
		return nil
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reauthCalled {
		t.Fatal("reauth must not be called for valid returning user")
	}
	if result.IsFirstAuth || result.WasReauth {
		t.Fatal("expected plain returning user result")
	}
}

func TestAuthIntegrationExpiredUserReauth(t *testing.T) {
	repo := newIntAuthRepo()
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	// Seed expired account
	past := time.Now().UTC().Add(-1 * time.Hour)
	repo.accounts[repoKey{"identity-expired", auth.MockProvider}] = auth.OAuthAccount{
		SSHIdentityID:  "identity-expired",
		Provider:       auth.MockProvider,
		Status:         auth.OAuthAccountStatusActive,
		AccessToken:    "expired-token",
		TokenExpiresAt: &past,
	}

	reauthCount := 0
	result, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{SSHIdentityID: "identity-expired", Reauth: func(_ context.Context) error {
		reauthCount++
		return nil
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reauthCount != 1 {
		t.Fatalf("expected reauth called once, got %d", reauthCount)
	}
	if !result.WasReauth {
		t.Fatal("expected WasReauth=true")
	}
	if result.Account.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected active after reauth, got %s", result.Account.Status)
	}
	// Token refreshed in repo
	found, _ := repo.FindOAuthAccountBySSHIdentityAndProvider(context.Background(), "identity-expired", auth.MockProvider)
	if found.TokenExpiresAt == nil || found.TokenExpiresAt.Before(time.Now().UTC()) {
		t.Fatal("expected refreshed future expiry in repo")
	}
}

func TestAuthIntegrationReconnectRequiredReauth(t *testing.T) {
	repo := newIntAuthRepo()
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	// Seed account with reconnect_required status (token not wall-clock expired)
	future := time.Now().UTC().Add(2 * time.Hour)
	repo.accounts[repoKey{"identity-reconnect", auth.MockProvider}] = auth.OAuthAccount{
		SSHIdentityID:  "identity-reconnect",
		Provider:       auth.MockProvider,
		Status:         auth.OAuthAccountStatusReconnectRequired,
		AccessToken:    "stale-token",
		TokenExpiresAt: &future, // not expired by time, but status forces reauth
	}

	reauthCount := 0
	result, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{SSHIdentityID: "identity-reconnect", Reauth: func(_ context.Context) error {
		reauthCount++
		return nil
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reauthCount != 1 {
		t.Fatalf("expected reauth called once, got %d", reauthCount)
	}
	if !result.WasReauth {
		t.Fatal("expected WasReauth=true")
	}
	if result.Account.Status != auth.OAuthAccountStatusActive {
		t.Fatalf("expected active after reauth, got %s", result.Account.Status)
	}
}

func TestAuthIntegrationRevokedUserBlocked(t *testing.T) {
	repo := newIntAuthRepo()
	useCase := auth.NewEnsureValidAccountUseCase(repo)

	repo.accounts[repoKey{"identity-revoked", auth.MockProvider}] = auth.OAuthAccount{
		SSHIdentityID: "identity-revoked",
		Provider:      auth.MockProvider,
		Status:        auth.OAuthAccountStatusRevoked,
		AccessToken:   "revoked-token",
	}

	_, err := useCase.Execute(context.Background(), auth.EnsureValidAccountInput{SSHIdentityID: "identity-revoked", Reauth: func(_ context.Context) error {
		t.Fatal("reauth must not be called for revoked account")
		return nil
	}})
	if !errors.Is(err, auth.ErrAccountRevoked) {
		t.Fatalf("expected ErrAccountRevoked, got %v", err)
	}
}

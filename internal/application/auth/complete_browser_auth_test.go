package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"swiggy-ssh/internal/application/auth"
)

type claimAttemptService struct {
	attempt   auth.BrowserAuthAttempt
	err       error
	completed bool
	cancelled bool
}

func (s *claimAttemptService) IssueAuthAttempt(context.Context, string, string) (string, auth.BrowserAuthAttempt, error) {
	return "", auth.BrowserAuthAttempt{}, nil
}

func (s *claimAttemptService) GetAuthAttempt(context.Context, string) (auth.BrowserAuthAttempt, error) {
	return s.attempt, nil
}

func (s *claimAttemptService) CompleteAuthAttempt(context.Context, string) error { return nil }

func (s *claimAttemptService) CompleteClaimedAuthAttempt(context.Context, string) error {
	s.completed = true
	s.attempt.Status = auth.AuthAttemptStatusCompleted
	return nil
}

func (s *claimAttemptService) CancelClaimedAuthAttempt(context.Context, string) error {
	s.cancelled = true
	s.attempt.Status = auth.AuthAttemptStatusCancelled
	return nil
}

func (s *claimAttemptService) ClaimAuthAttempt(context.Context, string) (auth.BrowserAuthAttempt, error) {
	if s.err != nil {
		return auth.BrowserAuthAttempt{}, s.err
	}
	s.attempt.Status = auth.AuthAttemptStatusClaimed
	return s.attempt, nil
}

func (s *claimAttemptService) CancelAuthAttempt(context.Context, string) error { return nil }

type countingAuthRepo struct {
	upserts  int
	upserted auth.OAuthAccount
}

type callbackProvider struct {
	err          error
	codeVerifier string
	exchanges    int
}

func (p *callbackProvider) ExchangeBrowserAuthCallback(_ context.Context, input auth.BrowserAuthCallbackInput) (auth.BrowserAuthCredentials, error) {
	p.exchanges++
	p.codeVerifier = input.CodeVerifier
	if p.err != nil {
		return auth.BrowserAuthCredentials{}, p.err
	}
	expires := time.Now().UTC().Add(time.Hour)
	return auth.BrowserAuthCredentials{AccessToken: "token-1", TokenExpiresAt: &expires, Scopes: []string{"mcp:tools"}}, nil
}

func (r *countingAuthRepo) FindOAuthAccountBySSHIdentityAndProvider(context.Context, string, string) (auth.OAuthAccount, error) {
	return auth.OAuthAccount{}, auth.ErrOAuthAccountNotFound
}

func (r *countingAuthRepo) UpsertOAuthAccount(_ context.Context, account auth.OAuthAccount) (auth.OAuthAccount, error) {
	r.upserts++
	r.upserted = account
	return account, nil
}

func TestCompleteBrowserAuthClaimsBeforeAccountWrite(t *testing.T) {
	repo := &countingAuthRepo{}
	svc := &claimAttemptService{attempt: auth.BrowserAuthAttempt{
		SSHIdentityID: "identity-1",
		Status:        auth.AuthAttemptStatusPending,
		ExpiresAt:     time.Now().UTC().Add(time.Minute),
	}}
	useCase := auth.NewCompleteBrowserAuthUseCase(repo, svc)

	result, err := useCase.Execute(context.Background(), "opaque")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Attempt.Status != auth.AuthAttemptStatusCompleted {
		t.Fatalf("expected completed attempt, got %s", result.Attempt.Status)
	}
	if repo.upserts != 1 {
		t.Fatalf("expected one account upsert, got %d", repo.upserts)
	}
	if !svc.completed {
		t.Fatal("expected claimed attempt to be completed after account write")
	}
}

func TestCompleteBrowserAuthReplayDoesNotUpsertAccount(t *testing.T) {
	repo := &countingAuthRepo{}
	svc := &claimAttemptService{err: auth.ErrAuthAttemptAlreadyUsed}
	useCase := auth.NewCompleteBrowserAuthUseCase(repo, svc)

	_, err := useCase.Execute(context.Background(), "opaque")
	if !errors.Is(err, auth.ErrAuthAttemptAlreadyUsed) {
		t.Fatalf("expected ErrAuthAttemptAlreadyUsed, got %v", err)
	}
	if repo.upserts != 0 {
		t.Fatalf("replay must not upsert account, got %d upserts", repo.upserts)
	}
}

func TestCompleteBrowserAuthCallbackUsesClaimedVerifier(t *testing.T) {
	repo := &countingAuthRepo{}
	svc := &claimAttemptService{attempt: auth.BrowserAuthAttempt{
		SSHIdentityID: "identity-1",
		Status:        auth.AuthAttemptStatusPending,
		CodeVerifier:  "verifier-1",
		ExpiresAt:     time.Now().UTC().Add(time.Minute),
	}}
	callback := &callbackProvider{}
	useCase := auth.NewCompleteBrowserAuthUseCase(repo, svc, callback)

	_, err := useCase.ExecuteCallback(context.Background(), auth.BrowserAuthCallbackInput{State: "opaque", Code: "code-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callback.codeVerifier != "verifier-1" {
		t.Fatalf("expected claimed verifier, got %q", callback.codeVerifier)
	}
	if repo.upserts != 1 || !svc.completed {
		t.Fatalf("expected upsert and completion, got upserts=%d completed=%v", repo.upserts, svc.completed)
	}
}

func TestCompleteBrowserAuthCallbackPersistsFirstLoginAccountForAttemptIdentity(t *testing.T) {
	repo := &countingAuthRepo{}
	svc := &claimAttemptService{attempt: auth.BrowserAuthAttempt{
		SSHIdentityID: "first-login-identity",
		Status:        auth.AuthAttemptStatusPending,
		CodeVerifier:  "verifier-1",
		ExpiresAt:     time.Now().UTC().Add(time.Minute),
	}}
	callback := &callbackProvider{}
	useCase := auth.NewCompleteBrowserAuthUseCase(repo, svc, callback)

	result, err := useCase.ExecuteCallback(context.Background(), auth.BrowserAuthCallbackInput{State: "opaque", Code: "code-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.upserted.SSHIdentityID != "first-login-identity" {
		t.Fatalf("expected account for first-login-identity, got %q", repo.upserted.SSHIdentityID)
	}
	if result.Account.SSHIdentityID != "first-login-identity" {
		t.Fatalf("expected result account for first-login-identity, got %q", result.Account.SSHIdentityID)
	}
}

func TestCompleteBrowserAuthCallbackFailedExchangeDoesNotComplete(t *testing.T) {
	repo := &countingAuthRepo{}
	svc := &claimAttemptService{attempt: auth.BrowserAuthAttempt{
		SSHIdentityID: "identity-1",
		Status:        auth.AuthAttemptStatusPending,
		CodeVerifier:  "verifier-1",
		ExpiresAt:     time.Now().UTC().Add(time.Minute),
	}}
	useCase := auth.NewCompleteBrowserAuthUseCase(repo, svc, &callbackProvider{err: auth.ErrBrowserAuthProviderCallback})

	_, err := useCase.ExecuteCallback(context.Background(), auth.BrowserAuthCallbackInput{State: "opaque", Code: "code-1"})
	if !errors.Is(err, auth.ErrBrowserAuthProviderCallback) {
		t.Fatalf("expected callback error, got %v", err)
	}
	if repo.upserts != 0 || svc.completed {
		t.Fatalf("failed exchange must not persist or complete, got upserts=%d completed=%v", repo.upserts, svc.completed)
	}
	if !svc.cancelled {
		t.Fatal("failed exchange must cancel the claimed attempt")
	}
}

func TestCompleteBrowserAuthGuestAttemptDoesNotCompleteOrUpsert(t *testing.T) {
	repo := &countingAuthRepo{}
	svc := &claimAttemptService{attempt: auth.BrowserAuthAttempt{
		Status:    auth.AuthAttemptStatusPending,
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}}
	useCase := auth.NewCompleteBrowserAuthUseCase(repo, svc)

	_, err := useCase.Execute(context.Background(), "opaque")
	if !errors.Is(err, auth.ErrSSHIdentityRequired) {
		t.Fatalf("expected ErrSSHIdentityRequired, got %v", err)
	}
	if repo.upserts != 0 || svc.completed {
		t.Fatalf("guest attempt must not persist or complete, got upserts=%d completed=%v", repo.upserts, svc.completed)
	}
	if !svc.cancelled {
		t.Fatal("guest attempt should be cancelled after being claimed")
	}
}

func TestCompleteBrowserAuthCallbackGuestAttemptDoesNotExchangeCompleteOrUpsert(t *testing.T) {
	repo := &countingAuthRepo{}
	svc := &claimAttemptService{attempt: auth.BrowserAuthAttempt{
		Status:       auth.AuthAttemptStatusPending,
		CodeVerifier: "verifier-1",
		ExpiresAt:    time.Now().UTC().Add(time.Minute),
	}}
	callback := &callbackProvider{}
	useCase := auth.NewCompleteBrowserAuthUseCase(repo, svc, callback)

	_, err := useCase.ExecuteCallback(context.Background(), auth.BrowserAuthCallbackInput{State: "opaque", Code: "code-1"})
	if !errors.Is(err, auth.ErrSSHIdentityRequired) {
		t.Fatalf("expected ErrSSHIdentityRequired, got %v", err)
	}
	if callback.exchanges != 0 {
		t.Fatalf("guest callback must not exchange provider token, got %d exchanges", callback.exchanges)
	}
	if repo.upserts != 0 || svc.completed {
		t.Fatalf("guest callback must not persist or complete, got upserts=%d completed=%v", repo.upserts, svc.completed)
	}
	if !svc.cancelled {
		t.Fatal("guest callback should be cancelled after being claimed")
	}
}

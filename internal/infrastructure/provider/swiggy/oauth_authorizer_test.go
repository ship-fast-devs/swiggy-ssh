package swiggy

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	domainauth "swiggy-ssh/internal/domain/auth"
)

func TestOAuthAccountAuthorizerAddsBearerToken(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	repo := &authorizerRepo{account: domainauth.OAuthAccount{
		SSHIdentityID:  "identity-1",
		Provider:       oauthProvider,
		AccessToken:    "token-1",
		TokenExpiresAt: &expires,
		Status:         domainauth.OAuthAccountStatusActive,
	}}
	authorizer := NewOAuthAccountAuthorizer(repo)
	req, err := http.NewRequest(http.MethodPost, "https://mcp.swiggy.com/im", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	ctx := domainauth.ContextWithUserID(context.Background(), "identity-1")

	if err := authorizer.AuthorizeMCPRequest(ctx, req); err != nil {
		t.Fatalf("authorize request: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer token-1" {
		t.Fatalf("unexpected authorization header %q", got)
	}
	if repo.sshIdentityID != "identity-1" || repo.provider != oauthProvider {
		t.Fatalf("unexpected lookup identity/provider: %q %q", repo.sshIdentityID, repo.provider)
	}
}

func TestOAuthAccountAuthorizerRequiresUserContext(t *testing.T) {
	authorizer := NewOAuthAccountAuthorizer(&authorizerRepo{})
	req, err := http.NewRequest(http.MethodPost, "https://mcp.swiggy.com/im", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	err = authorizer.AuthorizeMCPRequest(context.Background(), req)
	if !errors.Is(err, domainauth.ErrSSHIdentityRequired) {
		t.Fatalf("expected ssh identity required, got %v", err)
	}
}

func TestOAuthAccountAuthorizerRejectsExpiredToken(t *testing.T) {
	expires := time.Now().Add(-time.Hour)
	authorizer := NewOAuthAccountAuthorizer(&authorizerRepo{account: domainauth.OAuthAccount{
		SSHIdentityID:  "identity-1",
		Provider:       oauthProvider,
		AccessToken:    "token-1",
		TokenExpiresAt: &expires,
		Status:         domainauth.OAuthAccountStatusActive,
	}})
	req, err := http.NewRequest(http.MethodPost, "https://mcp.swiggy.com/im", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	ctx := domainauth.ContextWithUserID(context.Background(), "identity-1")

	err = authorizer.AuthorizeMCPRequest(ctx, req)
	if !errors.Is(err, domainauth.ErrTokenExpired) {
		t.Fatalf("expected token expired, got %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("authorization header should not be set, got %q", got)
	}
}

type authorizerRepo struct {
	account       domainauth.OAuthAccount
	sshIdentityID string
	provider      string
	err           error
}

func (r *authorizerRepo) UpsertOAuthAccount(context.Context, domainauth.OAuthAccount) (domainauth.OAuthAccount, error) {
	return domainauth.OAuthAccount{}, nil
}

func (r *authorizerRepo) FindOAuthAccountBySSHIdentityAndProvider(_ context.Context, sshIdentityID, provider string) (domainauth.OAuthAccount, error) {
	r.sshIdentityID = sshIdentityID
	r.provider = provider
	if r.err != nil {
		return domainauth.OAuthAccount{}, r.err
	}
	return r.account, nil
}

package auth

import (
	"context"
	"fmt"
	"time"
)

// CompleteBrowserAuthOutput reports the account created or refreshed by a completed browser auth attempt.
type CompleteBrowserAuthOutput struct {
	Attempt BrowserAuthAttempt
	Account OAuthAccount
}

// CompleteBrowserAuthUseCase completes the terminal-to-browser auth attempt and stores provider credentials.
// The current provider implementation writes mock credentials; real Swiggy callback exchange stays behind this seam.
type CompleteBrowserAuthUseCase struct {
	repo       Repository
	attemptSvc BrowserAuthAttemptService
	callback   BrowserAuthCallbackProvider
	now        func() time.Time
}

func NewCompleteBrowserAuthUseCase(repo Repository, attemptSvc BrowserAuthAttemptService, callback ...BrowserAuthCallbackProvider) *CompleteBrowserAuthUseCase {
	useCase := &CompleteBrowserAuthUseCase{
		repo:       repo,
		attemptSvc: attemptSvc,
		now:        func() time.Time { return time.Now().UTC() },
	}
	if len(callback) > 0 {
		useCase.callback = callback[0]
	}
	return useCase
}

func (s *CompleteBrowserAuthUseCase) Execute(ctx context.Context, rawAttemptToken string) (CompleteBrowserAuthOutput, error) {
	expiresAt := s.now().Add(mockTokenTTL)
	return s.completeWithCredentials(ctx, rawAttemptToken, BrowserAuthCredentials{
		AccessToken:    mockAccessToken(""),
		TokenExpiresAt: &expiresAt,
		Scopes:         []string{"profile:read"},
	})
}

func (s *CompleteBrowserAuthUseCase) ExecuteCallback(ctx context.Context, input BrowserAuthCallbackInput) (CompleteBrowserAuthOutput, error) {
	if s.callback == nil {
		return CompleteBrowserAuthOutput{}, ErrBrowserAuthProviderUnavailable
	}
	attempt, err := s.attemptSvc.ClaimAuthAttempt(ctx, input.State)
	if err != nil {
		return CompleteBrowserAuthOutput{}, err
	}
	if attempt.SSHIdentityID == "" {
		if cancelErr := s.attemptSvc.CancelClaimedAuthAttempt(ctx, input.State); cancelErr != nil {
			return CompleteBrowserAuthOutput{}, fmt.Errorf("cancel guest browser auth attempt: %w", cancelErr)
		}
		return CompleteBrowserAuthOutput{}, ErrSSHIdentityRequired
	}
	input.CodeVerifier = attempt.CodeVerifier
	credentials, err := s.callback.ExchangeBrowserAuthCallback(ctx, input)
	if err != nil {
		if cancelErr := s.attemptSvc.CancelClaimedAuthAttempt(ctx, input.State); cancelErr != nil {
			return CompleteBrowserAuthOutput{}, fmt.Errorf("cancel failed browser auth attempt: %w", cancelErr)
		}
		return CompleteBrowserAuthOutput{}, err
	}
	return s.completeClaimedWithCredentials(ctx, input.State, attempt, credentials)
}

func (s *CompleteBrowserAuthUseCase) completeWithCredentials(ctx context.Context, rawAttemptToken string, credentials BrowserAuthCredentials) (CompleteBrowserAuthOutput, error) {
	attempt, err := s.attemptSvc.ClaimAuthAttempt(ctx, rawAttemptToken)
	if err != nil {
		return CompleteBrowserAuthOutput{}, err
	}
	return s.completeClaimedWithCredentials(ctx, rawAttemptToken, attempt, credentials)
}

func (s *CompleteBrowserAuthUseCase) completeClaimedWithCredentials(ctx context.Context, rawAttemptToken string, attempt BrowserAuthAttempt, credentials BrowserAuthCredentials) (CompleteBrowserAuthOutput, error) {
	if attempt.SSHIdentityID == "" {
		if err := s.attemptSvc.CancelClaimedAuthAttempt(ctx, rawAttemptToken); err != nil {
			return CompleteBrowserAuthOutput{}, fmt.Errorf("cancel guest browser auth attempt: %w", err)
		}
		return CompleteBrowserAuthOutput{}, ErrSSHIdentityRequired
	}

	var err error
	var account OAuthAccount
	accessToken := credentials.AccessToken
	if accessToken == mockAccessToken("") {
		accessToken = mockAccessToken(attempt.SSHIdentityID)
	}
	account, err = s.repo.UpsertOAuthAccount(ctx, OAuthAccount{
		SSHIdentityID:  attempt.SSHIdentityID,
		Provider:       MockProvider,
		ProviderUserID: credentials.ProviderUserID,
		AccessToken:    accessToken,
		TokenExpiresAt: credentials.TokenExpiresAt,
		Scopes:         credentials.Scopes,
		Status:         OAuthAccountStatusActive,
	})
	if err != nil {
		return CompleteBrowserAuthOutput{}, fmt.Errorf("store oauth account: %w", err)
	}
	if err := s.attemptSvc.CompleteClaimedAuthAttempt(ctx, rawAttemptToken); err != nil {
		return CompleteBrowserAuthOutput{}, err
	}
	attempt.Status = AuthAttemptStatusCompleted

	return CompleteBrowserAuthOutput{Attempt: attempt, Account: account}, nil
}

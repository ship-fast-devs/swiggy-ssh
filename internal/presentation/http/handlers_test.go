package http

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"swiggy-ssh/internal/application/auth"
)

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testAuthAttemptService is a minimal stub implementing auth.BrowserAuthAttemptService.
// Set completeErr to control what CompleteAuthAttempt returns.
type testAuthAttemptService struct {
	completeErr error
	getStatus   string
	claimed     bool
	completed   bool
	cancelled   bool
}

func (m *testAuthAttemptService) IssueAuthAttempt(_ context.Context, _, _ string) (string, auth.BrowserAuthAttempt, error) {
	return "", auth.BrowserAuthAttempt{}, nil
}

func (m *testAuthAttemptService) GetAuthAttempt(_ context.Context, _ string) (auth.BrowserAuthAttempt, error) {
	status := m.getStatus
	if status == "" {
		status = auth.AuthAttemptStatusPending
	}
	return auth.BrowserAuthAttempt{Status: status, CodeVerifier: "verifier-1"}, nil
}

func (m *testAuthAttemptService) CompleteAuthAttempt(_ context.Context, _ string) error {
	return m.completeErr
}

func (m *testAuthAttemptService) ClaimAuthAttempt(_ context.Context, _ string) (auth.BrowserAuthAttempt, error) {
	if m.completeErr != nil {
		return auth.BrowserAuthAttempt{}, m.completeErr
	}
	m.claimed = true
	return auth.BrowserAuthAttempt{SSHIdentityID: "identity-1", Status: auth.AuthAttemptStatusClaimed, CodeVerifier: "verifier-1"}, nil
}

func (m *testAuthAttemptService) CompleteClaimedAuthAttempt(_ context.Context, _ string) error {
	m.completed = true
	return nil
}

func (m *testAuthAttemptService) CancelClaimedAuthAttempt(_ context.Context, _ string) error {
	m.cancelled = true
	return nil
}

func (m *testAuthAttemptService) CancelAuthAttempt(_ context.Context, _ string) error {
	return nil
}

func newTestServer(svc auth.BrowserAuthAttemptService) *Server {
	return New(":0", newDiscardLogger(), svc, nil, nil, "http://localhost:8080", "mock")
}

type testBrowserAuthProvider struct{}

func (p testBrowserAuthProvider) StartBrowserAuth(_ context.Context, input auth.BrowserAuthStartInput) (auth.BrowserAuthStartOutput, error) {
	values := url.Values{}
	values.Set("response_type", "code")
	values.Set("client_id", "client-1")
	values.Set("redirect_uri", input.CallbackURL)
	values.Set("code_challenge", testCodeChallenge(input.CodeVerifier))
	values.Set("code_challenge_method", "S256")
	values.Set("state", input.State)
	values.Set("scope", "mcp:tools")
	return auth.BrowserAuthStartOutput{RedirectURL: "https://swiggy.example/login?" + values.Encode()}, nil
}

func testCodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type testCallbackProvider struct {
	err error
}

func (p testCallbackProvider) ExchangeBrowserAuthCallback(_ context.Context, input auth.BrowserAuthCallbackInput) (auth.BrowserAuthCredentials, error) {
	if p.err != nil {
		return auth.BrowserAuthCredentials{}, p.err
	}
	expires := time.Now().UTC().Add(time.Hour)
	return auth.BrowserAuthCredentials{
		AccessToken:    "provider-token-" + input.Code,
		TokenExpiresAt: &expires,
		Scopes:         []string{"profile:read"},
	}, nil
}

type testAuthRepo struct {
	upserts int
}

func (r *testAuthRepo) FindOAuthAccountBySSHIdentityAndProvider(context.Context, string, string) (auth.OAuthAccount, error) {
	return auth.OAuthAccount{}, auth.ErrOAuthAccountNotFound
}

func (r *testAuthRepo) UpsertOAuthAccount(_ context.Context, account auth.OAuthAccount) (auth.OAuthAccount, error) {
	r.upserts++
	return account, nil
}

func TestHandleHealthOK(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testAuthAttemptService{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"ok"`) {
		t.Fatalf("expected body to contain ok, got: %s", body)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got: %s", ct)
	}
}

func TestHandleLoginGetRendersHelpfulPage(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testAuthAttemptService{})
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()

	srv.handleLoginGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Browser Login") {
		t.Fatalf("expected body to contain 'Browser Login', got: %s", body)
	}
	if !strings.Contains(body, "direct URL") {
		t.Fatalf("expected body to contain direct URL guidance, got: %s", body)
	}
}

func TestHandleAuthStartSuccess(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testAuthAttemptService{completeErr: nil})
	req := httptest.NewRequest(http.MethodGet, "/auth/start?attempt=opaque-token", nil)
	w := httptest.NewRecorder()

	srv.handleAuthStart(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Login complete") {
		t.Fatalf("expected body to contain 'Login complete', got: %s", body)
	}
}

func TestHandleAuthStartExpiredAttempt(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testAuthAttemptService{completeErr: auth.ErrAuthAttemptNotFound})
	req := httptest.NewRequest(http.MethodGet, "/auth/start?attempt=expired", nil)
	w := httptest.NewRecorder()

	srv.handleAuthStart(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "not found or expired") {
		t.Fatalf("expected body to contain 'not found or expired', got: %s", body)
	}
}

func TestHandleAuthStartAlreadyUsed(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testAuthAttemptService{completeErr: auth.ErrAuthAttemptAlreadyUsed})
	req := httptest.NewRequest(http.MethodGet, "/auth/start?attempt=used", nil)
	w := httptest.NewRecorder()

	srv.handleAuthStart(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "already been used") {
		t.Fatalf("expected body to contain 'already been used', got: %s", body)
	}
}

func TestHandleAuthStartMissingAttempt(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testAuthAttemptService{})
	req := httptest.NewRequest(http.MethodGet, "/auth/start", nil)
	w := httptest.NewRecorder()

	srv.handleAuthStart(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Missing auth attempt") {
		t.Fatalf("expected body to contain missing attempt, got: %s", body)
	}
}

func TestHandleAuthStartRedirectsInProviderMode(t *testing.T) {
	t.Parallel()

	svc := &testAuthAttemptService{}
	start := auth.NewStartBrowserAuthUseCase(svc, testBrowserAuthProvider{})
	srv := New(":0", newDiscardLogger(), svc, nil, start, "http://localhost:8080", "swiggy")
	req := httptest.NewRequest(http.MethodGet, "/auth/start?attempt=opaque%20token", nil)
	w := httptest.NewRecorder()

	srv.handleAuthStart(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	if u.Scheme+"://"+u.Host+u.Path != "https://swiggy.example/login" {
		t.Fatalf("expected provider redirect, got %s", location)
	}
	q := u.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != "client-1" || q.Get("redirect_uri") != "http://localhost:8080/auth/callback" {
		t.Fatalf("missing OAuth redirect params: %s", location)
	}
	if q.Get("code_challenge") != testCodeChallenge("verifier-1") || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("missing PKCE redirect params: %s", location)
	}
	if q.Get("state") != "opaque token" || q.Get("scope") != "mcp:tools" {
		t.Fatalf("missing state/scope redirect params: %s", location)
	}
}

func TestHandleLoginRedirectEscapesAttempt(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&testAuthAttemptService{})
	req := httptest.NewRequest(http.MethodGet, "/login?attempt=opaque%20token", nil)
	w := httptest.NewRecorder()

	srv.handleLoginGet(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if got := w.Header().Get("Location"); got != "/auth/start?attempt=opaque+token" {
		t.Fatalf("unexpected redirect location: %s", got)
	}
}

func TestHandleAuthCallbackProviderModeCompletesWithState(t *testing.T) {
	t.Parallel()

	svc := &testAuthAttemptService{}
	repo := &testAuthRepo{}
	complete := auth.NewCompleteBrowserAuthUseCase(repo, svc, testCallbackProvider{})
	srv := New(":0", newDiscardLogger(), svc, complete, nil, "http://localhost:8080", "swiggy")
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=opaque&code=provider-code", nil)
	w := httptest.NewRecorder()

	srv.handleAuthCallback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !svc.claimed || !svc.completed {
		t.Fatal("expected attempt to be claimed and completed")
	}
	if repo.upserts != 1 {
		t.Fatalf("expected one account upsert, got %d", repo.upserts)
	}
}

func TestHandleAuthCallbackRequiresState(t *testing.T) {
	t.Parallel()

	svc := &testAuthAttemptService{}
	srv := New(":0", newDiscardLogger(), svc, nil, nil, "http://localhost:8080", "swiggy")
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?attempt=opaque&code=provider-code", nil)
	w := httptest.NewRecorder()

	srv.handleAuthCallback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected rendered error 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Missing auth state") {
		t.Fatalf("expected missing state error, got %s", w.Body.String())
	}
}

func TestHandleAuthCallbackRequiresCodeInProviderMode(t *testing.T) {
	t.Parallel()

	svc := &testAuthAttemptService{}
	srv := New(":0", newDiscardLogger(), svc, nil, nil, "http://localhost:8080", "swiggy")
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=opaque", nil)
	w := httptest.NewRecorder()

	srv.handleAuthCallback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected rendered error 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Missing Swiggy authorization code") {
		t.Fatalf("expected missing code error, got %s", w.Body.String())
	}
}

func TestHandleAuthCallbackProviderFailureDoesNotClaim(t *testing.T) {
	t.Parallel()

	svc := &testAuthAttemptService{}
	repo := &testAuthRepo{}
	complete := auth.NewCompleteBrowserAuthUseCase(repo, svc, testCallbackProvider{err: auth.ErrBrowserAuthProviderCallback})
	srv := New(":0", newDiscardLogger(), svc, complete, nil, "http://localhost:8080", "swiggy")
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=opaque&code=provider-code", nil)
	w := httptest.NewRecorder()

	srv.handleAuthCallback(w, req)

	if !svc.claimed || svc.completed || !svc.cancelled {
		t.Fatal("provider failure must claim and cancel but not complete attempt")
	}
	if repo.upserts != 0 {
		t.Fatalf("provider failure must not upsert account, got %d", repo.upserts)
	}
}

func TestHandleAuthCallbackReplayDoesNotUpsertTwice(t *testing.T) {
	t.Parallel()

	svc := &testAuthAttemptService{completeErr: auth.ErrAuthAttemptAlreadyUsed}
	repo := &testAuthRepo{}
	complete := auth.NewCompleteBrowserAuthUseCase(repo, svc, testCallbackProvider{})
	srv := New(":0", newDiscardLogger(), svc, complete, nil, "http://localhost:8080", "swiggy")
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=opaque&code=provider-code", nil)
	w := httptest.NewRecorder()

	srv.handleAuthCallback(w, req)

	if repo.upserts != 0 {
		t.Fatalf("replay must not upsert account, got %d", repo.upserts)
	}
}

package mock

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"swiggy-ssh/internal/domain/auth"
)

// MockLoginCodeService is an in-memory implementation of auth.LoginCodeService for tests.
type MockLoginCodeService struct {
	mu            sync.Mutex
	records       map[string]*mockRecord // keyed by rawCode
	ttl           time.Duration
	now           func() time.Time
	generateToken func() (string, error)
}

type mockRecord struct {
	record auth.LoginCode
}

func NewMockLoginCodeService(ttl time.Duration) *MockLoginCodeService {
	return &MockLoginCodeService{
		records:       make(map[string]*mockRecord),
		ttl:           ttl,
		now:           func() time.Time { return time.Now().UTC() },
		generateToken: defaultGenerateToken,
	}
}

func defaultGenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func mockHashToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

func (m *MockLoginCodeService) IssueAuthAttempt(ctx context.Context, sshIdentityID, terminalSessionID string) (string, auth.BrowserAuthAttempt, error) {
	rawToken, err := m.generateToken()
	if err != nil {
		return "", auth.BrowserAuthAttempt{}, err
	}
	codeVerifier, err := m.generateToken()
	if err != nil {
		return "", auth.BrowserAuthAttempt{}, err
	}

	now := m.now()
	tokenHash := mockHashToken(rawToken)
	record := auth.BrowserAuthAttempt{
		TokenHash:         tokenHash,
		CodeHash:          tokenHash,
		CodeVerifier:      codeVerifier,
		SSHIdentityID:     sshIdentityID,
		TerminalSessionID: terminalSessionID,
		Status:            auth.AuthAttemptStatusPending,
		ExpiresAt:         now.Add(m.ttl),
		CreatedAt:         now,
	}

	m.mu.Lock()
	m.records[rawToken] = &mockRecord{record: record}
	m.mu.Unlock()

	return rawToken, record, nil
}

func (m *MockLoginCodeService) GetAuthAttempt(ctx context.Context, rawToken string) (auth.BrowserAuthAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.records[rawToken]
	if !ok {
		return auth.BrowserAuthAttempt{}, auth.ErrAuthAttemptNotFound
	}
	if m.now().After(r.record.ExpiresAt) {
		return auth.BrowserAuthAttempt{}, auth.ErrAuthAttemptNotFound
	}
	return r.record, nil
}

func (m *MockLoginCodeService) CompleteAuthAttempt(ctx context.Context, rawToken string) error {
	return m.transition(rawToken, auth.AuthAttemptStatusPending, auth.AuthAttemptStatusCompleted)
}

func (m *MockLoginCodeService) CompleteClaimedAuthAttempt(ctx context.Context, rawToken string) error {
	return m.transition(rawToken, auth.AuthAttemptStatusClaimed, auth.AuthAttemptStatusCompleted)
}

func (m *MockLoginCodeService) CancelClaimedAuthAttempt(ctx context.Context, rawToken string) error {
	return m.transition(rawToken, auth.AuthAttemptStatusClaimed, auth.AuthAttemptStatusCancelled)
}

func (m *MockLoginCodeService) ClaimAuthAttempt(ctx context.Context, rawToken string) (auth.BrowserAuthAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.records[rawToken]
	if !ok {
		return auth.BrowserAuthAttempt{}, auth.ErrAuthAttemptNotFound
	}
	if m.now().After(r.record.ExpiresAt) {
		return auth.BrowserAuthAttempt{}, auth.ErrAuthAttemptNotFound
	}
	if r.record.Status != auth.AuthAttemptStatusPending {
		return auth.BrowserAuthAttempt{}, auth.ErrAuthAttemptAlreadyUsed
	}
	r.record.Status = auth.AuthAttemptStatusClaimed
	return r.record, nil
}

func (m *MockLoginCodeService) CancelAuthAttempt(ctx context.Context, rawToken string) error {
	return m.transition(rawToken, auth.AuthAttemptStatusPending, auth.AuthAttemptStatusCancelled)
}

func (m *MockLoginCodeService) transition(rawCode, fromStatus, toStatus string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.records[rawCode]
	if !ok {
		return auth.ErrAuthAttemptNotFound
	}
	if m.now().After(r.record.ExpiresAt) {
		return auth.ErrAuthAttemptNotFound
	}
	if r.record.Status != fromStatus {
		return auth.ErrAuthAttemptAlreadyUsed
	}
	r.record.Status = toStatus
	return nil
}

// SetNow replaces the clock function for deterministic tests.
func (m *MockLoginCodeService) SetNow(f func() time.Time) {
	m.mu.Lock()
	m.now = f
	m.mu.Unlock()
}

// SetGenerateCode replaces the code generator for deterministic tests.
func (m *MockLoginCodeService) SetGenerateCode(f func() (string, error)) {
	m.mu.Lock()
	m.generateToken = f
	m.mu.Unlock()
}

func (m *MockLoginCodeService) IssueLoginCode(ctx context.Context, sshIdentityID, terminalSessionID string) (string, auth.LoginCode, error) {
	return m.IssueAuthAttempt(ctx, sshIdentityID, terminalSessionID)
}

func (m *MockLoginCodeService) GetLoginCode(ctx context.Context, rawCode string) (auth.LoginCode, error) {
	return m.GetAuthAttempt(ctx, rawCode)
}

func (m *MockLoginCodeService) CompleteLoginCode(ctx context.Context, rawCode string) error {
	return m.CompleteAuthAttempt(ctx, rawCode)
}

func (m *MockLoginCodeService) CancelLoginCode(ctx context.Context, rawCode string) error {
	return m.CancelAuthAttempt(ctx, rawCode)
}

// Compile-time interface assertion.
var _ auth.LoginCodeService = (*MockLoginCodeService)(nil)

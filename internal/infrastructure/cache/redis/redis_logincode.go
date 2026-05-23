package redis

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"swiggy-ssh/internal/domain/auth"

	"github.com/redis/go-redis/v9"
)

// RedisLoginCodeService implements auth browser-attempt services backed by Redis.
// Keys: "authattempt:<sha256-hex-of-raw-token>"
// Value: JSON-encoded authAttemptRecord
// TTL: set on IssueAuthAttempt; key eviction = expiry.
type RedisLoginCodeService struct {
	client *redis.Client
	ttl    time.Duration
	now    func() time.Time
}

// authAttemptRecord is what is stored in Redis (never contains raw token).
type authAttemptRecord struct {
	TokenHash         string    `json:"token_hash"`
	CodeHash          string    `json:"code_hash,omitempty"`
	CodeVerifier      string    `json:"code_verifier,omitempty"`
	SSHIdentityID     string    `json:"ssh_identity_id"`
	TerminalSessionID string    `json:"terminal_session_id"`
	Status            string    `json:"status"`
	ExpiresAt         time.Time `json:"expires_at"`
	CreatedAt         time.Time `json:"created_at"`
}

func NewRedisLoginCodeService(client *redis.Client, ttl time.Duration) *RedisLoginCodeService {
	return &RedisLoginCodeService{
		client: client,
		ttl:    ttl,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// hashToken returns the SHA-256 hex digest of the raw attempt token.
// Raw tokens are never stored — only this digest is persisted.
func hashToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

// redisKey returns the Redis key for a given raw attempt token.
func redisKey(rawToken string) string {
	return "authattempt:" + hashToken(rawToken)
}

// generateRawAttemptToken produces a high-entropy opaque URL token.
func generateRawAttemptToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *RedisLoginCodeService) IssueAuthAttempt(ctx context.Context, sshIdentityID, terminalSessionID string) (string, auth.BrowserAuthAttempt, error) {
	rawToken, err := generateRawAttemptToken()
	if err != nil {
		return "", auth.BrowserAuthAttempt{}, err
	}
	codeVerifier, err := generateRawAttemptToken()
	if err != nil {
		return "", auth.BrowserAuthAttempt{}, err
	}

	now := s.now()
	tokenHash := hashToken(rawToken)
	record := authAttemptRecord{
		TokenHash:         tokenHash,
		CodeHash:          tokenHash,
		CodeVerifier:      codeVerifier,
		SSHIdentityID:     sshIdentityID,
		TerminalSessionID: terminalSessionID,
		Status:            auth.AuthAttemptStatusPending,
		ExpiresAt:         now.Add(s.ttl),
		CreatedAt:         now,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return "", auth.BrowserAuthAttempt{}, fmt.Errorf("marshal auth attempt: %w", err)
	}

	key := redisKey(rawToken)
	if err := s.client.Set(ctx, key, data, s.ttl).Err(); err != nil {
		return "", auth.BrowserAuthAttempt{}, fmt.Errorf("store auth attempt: %w", err)
	}

	return rawToken, toAuthAttempt(record), nil
}

func (s *RedisLoginCodeService) GetAuthAttempt(ctx context.Context, rawToken string) (auth.BrowserAuthAttempt, error) {
	record, err := s.loadRecord(ctx, rawToken)
	if err != nil {
		return auth.BrowserAuthAttempt{}, err
	}
	return toAuthAttempt(record), nil
}

func (s *RedisLoginCodeService) CompleteAuthAttempt(ctx context.Context, rawToken string) error {
	return s.transition(ctx, rawToken, auth.AuthAttemptStatusPending, auth.AuthAttemptStatusCompleted)
}

func (s *RedisLoginCodeService) CompleteClaimedAuthAttempt(ctx context.Context, rawToken string) error {
	return s.transition(ctx, rawToken, auth.AuthAttemptStatusClaimed, auth.AuthAttemptStatusCompleted)
}

func (s *RedisLoginCodeService) CancelClaimedAuthAttempt(ctx context.Context, rawToken string) error {
	return s.transition(ctx, rawToken, auth.AuthAttemptStatusClaimed, auth.AuthAttemptStatusCancelled)
}

func (s *RedisLoginCodeService) ClaimAuthAttempt(ctx context.Context, rawToken string) (auth.BrowserAuthAttempt, error) {
	record, err := s.claim(ctx, rawToken)
	if err != nil {
		return auth.BrowserAuthAttempt{}, err
	}
	return toAuthAttempt(record), nil
}

func (s *RedisLoginCodeService) CancelAuthAttempt(ctx context.Context, rawToken string) error {
	return s.transition(ctx, rawToken, auth.AuthAttemptStatusPending, auth.AuthAttemptStatusCancelled)
}

// loadRecord fetches and unmarshals the record for rawToken.
func (s *RedisLoginCodeService) loadRecord(ctx context.Context, rawToken string) (authAttemptRecord, error) {
	key := redisKey(rawToken)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return authAttemptRecord{}, auth.ErrAuthAttemptNotFound
		}
		return authAttemptRecord{}, fmt.Errorf("get auth attempt: %w", err)
	}

	var record authAttemptRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return authAttemptRecord{}, fmt.Errorf("unmarshal auth attempt: %w", err)
	}
	if record.TokenHash == "" {
		record.TokenHash = record.CodeHash
	}
	return record, nil
}

// transition atomically reads, validates status, updates, and re-stores.
// Uses a Lua script for atomicity: read-check-write in a single round-trip.
func (s *RedisLoginCodeService) transition(ctx context.Context, rawCode, fromStatus, toStatus string) error {
	// Lua script: atomically load, check status, update if matching.
	// KEYS[1] = redis key
	// ARGV[1] = expected current status (fromStatus)
	// ARGV[2] = new status (toStatus)
	// Returns: 0 = key not found, 1 = success, 2 = wrong status
	const luaScript = `
local data = redis.call('GET', KEYS[1])
if not data then return 0 end
local rec = cjson.decode(data)
if rec.status ~= ARGV[1] then return 2 end
rec.status = ARGV[2]
local pttl = redis.call('PTTL', KEYS[1])
if pttl > 0 then
  redis.call('SET', KEYS[1], cjson.encode(rec), 'PX', pttl)
elseif pttl == -1 then
  -- key has no TTL (unexpected; IssueLoginCode always sets one) — preserve state
  redis.call('SET', KEYS[1], cjson.encode(rec))
else
  -- pttl == -2: key vanished between GET and SET (race) — treat as not found
  return 0
end
return 1
`
	key := redisKey(rawCode)
	result, err := s.client.Eval(ctx, luaScript, []string{key}, fromStatus, toStatus).Int()
	if err != nil {
		return fmt.Errorf("transition auth attempt: %w", err)
	}
	switch result {
	case 0:
		return auth.ErrAuthAttemptNotFound
	case 2:
		return auth.ErrAuthAttemptAlreadyUsed
	}
	return nil
}

func (s *RedisLoginCodeService) claim(ctx context.Context, rawToken string) (authAttemptRecord, error) {
	const luaScript = `
local data = redis.call('GET', KEYS[1])
if not data then return {0} end
local rec = cjson.decode(data)
if rec.status ~= ARGV[1] then return {2} end
rec.status = ARGV[2]
local pttl = redis.call('PTTL', KEYS[1])
if pttl > 0 then
  redis.call('SET', KEYS[1], cjson.encode(rec), 'PX', pttl)
elseif pttl == -1 then
  redis.call('SET', KEYS[1], cjson.encode(rec))
else
  return {0}
end
return {1, cjson.encode(rec)}
`
	key := redisKey(rawToken)
	values, err := s.client.Eval(ctx, luaScript, []string{key}, auth.AuthAttemptStatusPending, auth.AuthAttemptStatusClaimed).Slice()
	if err != nil {
		return authAttemptRecord{}, fmt.Errorf("claim auth attempt: %w", err)
	}
	code, ok := values[0].(int64)
	if !ok {
		return authAttemptRecord{}, fmt.Errorf("claim auth attempt: unexpected redis result")
	}
	switch code {
	case 0:
		return authAttemptRecord{}, auth.ErrAuthAttemptNotFound
	case 2:
		return authAttemptRecord{}, auth.ErrAuthAttemptAlreadyUsed
	}
	data, ok := values[1].(string)
	if !ok {
		return authAttemptRecord{}, fmt.Errorf("claim auth attempt: missing record")
	}
	var record authAttemptRecord
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return authAttemptRecord{}, fmt.Errorf("unmarshal claimed auth attempt: %w", err)
	}
	if record.TokenHash == "" {
		record.TokenHash = record.CodeHash
	}
	return record, nil
}

func toAuthAttempt(r authAttemptRecord) auth.BrowserAuthAttempt {
	return auth.BrowserAuthAttempt{
		TokenHash:         r.TokenHash,
		CodeHash:          r.TokenHash,
		CodeVerifier:      r.CodeVerifier,
		SSHIdentityID:     r.SSHIdentityID,
		TerminalSessionID: r.TerminalSessionID,
		Status:            r.Status,
		ExpiresAt:         r.ExpiresAt,
		CreatedAt:         r.CreatedAt,
	}
}

func (s *RedisLoginCodeService) IssueLoginCode(ctx context.Context, sshIdentityID, terminalSessionID string) (string, auth.LoginCode, error) {
	return s.IssueAuthAttempt(ctx, sshIdentityID, terminalSessionID)
}

func (s *RedisLoginCodeService) GetLoginCode(ctx context.Context, rawCode string) (auth.LoginCode, error) {
	return s.GetAuthAttempt(ctx, rawCode)
}

func (s *RedisLoginCodeService) CompleteLoginCode(ctx context.Context, rawCode string) error {
	return s.CompleteAuthAttempt(ctx, rawCode)
}

func (s *RedisLoginCodeService) ClaimLoginCode(ctx context.Context, rawCode string) (auth.LoginCode, error) {
	return s.ClaimAuthAttempt(ctx, rawCode)
}

func (s *RedisLoginCodeService) CompleteClaimedLoginCode(ctx context.Context, rawCode string) error {
	return s.CompleteClaimedAuthAttempt(ctx, rawCode)
}

func (s *RedisLoginCodeService) CancelClaimedLoginCode(ctx context.Context, rawCode string) error {
	return s.CancelClaimedAuthAttempt(ctx, rawCode)
}

func (s *RedisLoginCodeService) CancelLoginCode(ctx context.Context, rawCode string) error {
	return s.CancelAuthAttempt(ctx, rawCode)
}

// Compile-time interface assertion.
var _ auth.LoginCodeService = (*RedisLoginCodeService)(nil)

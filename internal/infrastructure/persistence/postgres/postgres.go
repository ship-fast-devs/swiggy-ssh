package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"swiggy-ssh/internal/domain/auth"
	"swiggy-ssh/internal/domain/identity"
)

var ErrNotFound = identity.ErrNotFound

type PostgresStore struct {
	pool      *pgxpool.Pool
	encryptor auth.TokenEncryptor
}

func NewPostgresStore(ctx context.Context, databaseURL string, encryptor auth.TokenEncryptor) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	return &PostgresStore{pool: pool, encryptor: encryptor}, nil
}

func (s *PostgresStore) Close() {
	s.pool.Close()
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PostgresStore) CreateSSHIdentity(ctx context.Context, sshIdentity identity.SSHIdentity) (identity.SSHIdentity, error) {
	if sshIdentity.ID == "" {
		sshIdentity.ID = uuid.NewString()
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO ssh_identities (id, public_key_fingerprint, public_key, label, last_seen_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, public_key_fingerprint, public_key, label, first_seen_at, last_seen_at, revoked_at
	`, sshIdentity.ID, sshIdentity.PublicKeyFingerprint, sshIdentity.PublicKey, sshIdentity.Label, sshIdentity.LastSeenAt, sshIdentity.RevokedAt)

	created := identity.SSHIdentity{}
	if err := row.Scan(
		&created.ID,
		&created.PublicKeyFingerprint,
		&created.PublicKey,
		&created.Label,
		&created.FirstSeenAt,
		&created.LastSeenAt,
		&created.RevokedAt,
	); err != nil {
		if isSSHFingerprintUniqueViolation(err) {
			return identity.SSHIdentity{}, identity.ErrSSHIdentityAlreadyExists
		}
		return identity.SSHIdentity{}, err
	}

	return created, nil
}

func isSSHFingerprintUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "23505" && pgErr.ConstraintName == "ssh_identities_public_key_fingerprint_key"
}

func (s *PostgresStore) FindSSHIdentityByFingerprint(ctx context.Context, fingerprint string) (identity.SSHIdentity, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, public_key_fingerprint, public_key, label, first_seen_at, last_seen_at, revoked_at
		FROM ssh_identities
		WHERE public_key_fingerprint = $1
	`, fingerprint)

	sshIdentity := identity.SSHIdentity{}
	if err := row.Scan(
		&sshIdentity.ID,
		&sshIdentity.PublicKeyFingerprint,
		&sshIdentity.PublicKey,
		&sshIdentity.Label,
		&sshIdentity.FirstSeenAt,
		&sshIdentity.LastSeenAt,
		&sshIdentity.RevokedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.SSHIdentity{}, ErrNotFound
		}

		return identity.SSHIdentity{}, err
	}

	return sshIdentity, nil
}

func (s *PostgresStore) UpdateSSHIdentityLastSeen(ctx context.Context, fingerprint string, lastSeenAt time.Time) error {
	result, err := s.pool.Exec(ctx, `UPDATE ssh_identities SET last_seen_at = $2 WHERE public_key_fingerprint = $1`, fingerprint, lastSeenAt)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *PostgresStore) UpsertOAuthAccount(ctx context.Context, account auth.OAuthAccount) (auth.OAuthAccount, error) {
	if account.ID == "" {
		account.ID = uuid.NewString()
	}

	encryptedToken, err := s.encryptor.Encrypt(ctx, account.AccessToken)
	if err != nil {
		return auth.OAuthAccount{}, fmt.Errorf("encrypt access token: %w", err)
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO oauth_accounts (
			id,
			ssh_identity_id,
			provider,
			provider_user_id,
			encrypted_access_token,
			token_expires_at,
			scopes,
			status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (ssh_identity_id, provider)
		DO UPDATE SET
			provider_user_id = EXCLUDED.provider_user_id,
			encrypted_access_token = EXCLUDED.encrypted_access_token,
			token_expires_at = EXCLUDED.token_expires_at,
			scopes = EXCLUDED.scopes,
			status = EXCLUDED.status,
			updated_at = now()
		RETURNING id, ssh_identity_id, provider, provider_user_id, encrypted_access_token, token_expires_at, scopes, status, created_at, updated_at
	`,
		account.ID,
		account.SSHIdentityID,
		account.Provider,
		account.ProviderUserID,
		encryptedToken,
		account.TokenExpiresAt,
		account.Scopes,
		account.Status,
	)

	upserted := auth.OAuthAccount{}
	var tokenExpiresAt sql.NullTime
	var upsertedEncryptedToken string
	if err := row.Scan(
		&upserted.ID,
		&upserted.SSHIdentityID,
		&upserted.Provider,
		&upserted.ProviderUserID,
		&upsertedEncryptedToken,
		&tokenExpiresAt,
		&upserted.Scopes,
		&upserted.Status,
		&upserted.CreatedAt,
		&upserted.UpdatedAt,
	); err != nil {
		return auth.OAuthAccount{}, err
	}
	upserted.TokenExpiresAt = nullTimeToPointer(tokenExpiresAt)

	plainToken, err := s.encryptor.Decrypt(ctx, upsertedEncryptedToken)
	if err != nil {
		return auth.OAuthAccount{}, fmt.Errorf("decrypt access token: %w", err)
	}
	upserted.AccessToken = plainToken

	return upserted, nil
}

func (s *PostgresStore) CreateTerminalSession(ctx context.Context, session identity.TerminalSession) (identity.TerminalSession, error) {
	if session.ID == "" {
		session.ID = uuid.NewString()
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO terminal_sessions (id, ssh_identity_id, ssh_fingerprint, client, client_session_id, current_screen, selected_address_id, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		RETURNING id, ssh_identity_id, ssh_fingerprint, client, client_session_id, current_screen, selected_address_id, created_at, last_seen_at, ended_at
	`, session.ID, session.SSHIdentityID, session.SSHFingerprint, session.Client, session.ClientSessionID, session.CurrentScreen, session.SelectedAddressID)

	created := identity.TerminalSession{}
	if err := row.Scan(
		&created.ID,
		&created.SSHIdentityID,
		&created.SSHFingerprint,
		&created.Client,
		&created.ClientSessionID,
		&created.CurrentScreen,
		&created.SelectedAddressID,
		&created.CreatedAt,
		&created.LastSeenAt,
		&created.EndedAt,
	); err != nil {
		return identity.TerminalSession{}, err
	}

	return created, nil
}

func (s *PostgresStore) MarkTerminalSessionEnded(ctx context.Context, sessionID string, endedAt time.Time) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE terminal_sessions
		SET ended_at = $2, last_seen_at = $2
		WHERE id = $1
	`, sessionID, endedAt)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *PostgresStore) AttachSSHIdentityToTerminalSession(ctx context.Context, sessionID, sshIdentityID string) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE terminal_sessions
		SET ssh_identity_id = $2, last_seen_at = now()
		WHERE id = $1
	`, sessionID, sshIdentityID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *PostgresStore) FindOAuthAccountBySSHIdentityAndProvider(ctx context.Context, sshIdentityID, provider string) (auth.OAuthAccount, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, ssh_identity_id, provider, provider_user_id, encrypted_access_token, token_expires_at, scopes, status, created_at, updated_at
		FROM oauth_accounts
		WHERE ssh_identity_id = $1 AND provider = $2
	`, sshIdentityID, provider)

	account := auth.OAuthAccount{}
	var tokenExpiresAt sql.NullTime
	var encryptedToken string
	if err := row.Scan(
		&account.ID,
		&account.SSHIdentityID,
		&account.Provider,
		&account.ProviderUserID,
		&encryptedToken,
		&tokenExpiresAt,
		&account.Scopes,
		&account.Status,
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.OAuthAccount{}, auth.ErrOAuthAccountNotFound
		}

		return auth.OAuthAccount{}, err
	}

	account.TokenExpiresAt = nullTimeToPointer(tokenExpiresAt)

	plainToken, err := s.encryptor.Decrypt(ctx, encryptedToken)
	if err != nil {
		return auth.OAuthAccount{}, fmt.Errorf("decrypt access token: %w", err)
	}
	account.AccessToken = plainToken

	return account, nil
}

func nullTimeToPointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}

	result := value.Time
	return &result
}

var (
	_ identity.Repository        = (*PostgresStore)(nil)
	_ identity.SessionRepository = (*PostgresStore)(nil)
	_ auth.Repository            = (*PostgresStore)(nil)
)

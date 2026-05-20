package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"swiggy-ssh/internal/domain/auth"
	"swiggy-ssh/internal/domain/identity"
	"swiggy-ssh/internal/infrastructure/crypto"
)

func testDatabaseURL(t *testing.T) string {
	t.Helper()

	if databaseURL := os.Getenv("TEST_DATABASE_URL"); databaseURL != "" {
		return databaseURL
	}

	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		return databaseURL
	}

	t.Skip("set TEST_DATABASE_URL or DATABASE_URL to run Postgres integration tests")
	return ""
}

func newTestStore(t *testing.T) *PostgresStore {
	t.Helper()

	ctx := context.Background()
	databaseURL := testDatabaseURL(t)

	if err := MigrateUp(ctx, databaseURL); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	store, err := NewPostgresStore(ctx, databaseURL, crypto.NoOpEncryptor{})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	cleanupTables(t, store)

	return store
}

func cleanupTables(t *testing.T, store *PostgresStore) {
	t.Helper()

	ctx := context.Background()
	_, err := store.pool.Exec(ctx, `
		TRUNCATE TABLE
			audit_events,
			terminal_sessions,
			oauth_accounts,
			ssh_identities
		RESTART IDENTITY
	`)
	if err != nil {
		t.Fatalf("cleanup tables: %v", err)
	}
}

func TestSSHIdentityCreateFindUpdate(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	label := "laptop"
	createdIdentity, err := store.CreateSSHIdentity(ctx, identity.SSHIdentity{
		PublicKeyFingerprint: "SHA256:test-fingerprint",
		PublicKey:            "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest test@example.com",
		Label:                &label,
	})
	if err != nil {
		t.Fatalf("create ssh identity: %v", err)
	}

	if createdIdentity.FirstSeenAt.IsZero() {
		t.Fatal("expected first_seen_at to be set")
	}

	found, err := store.FindSSHIdentityByFingerprint(ctx, "SHA256:test-fingerprint")
	if err != nil {
		t.Fatalf("find ssh identity: %v", err)
	}

	lastSeen := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.UpdateSSHIdentityLastSeen(ctx, found.PublicKeyFingerprint, lastSeen); err != nil {
		t.Fatalf("update ssh identity last seen: %v", err)
	}

	updated, err := store.FindSSHIdentityByFingerprint(ctx, found.PublicKeyFingerprint)
	if err != nil {
		t.Fatalf("find updated ssh identity: %v", err)
	}

	if updated.LastSeenAt == nil {
		t.Fatal("expected ssh identity last_seen_at to be set")
	}

	if updated.RevokedAt != nil {
		t.Fatalf("expected revoked_at to be nil, got %v", *updated.RevokedAt)
	}
}

func TestOAuthAccountUpsertAndFind(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	sshIdentity, err := store.CreateSSHIdentity(ctx, identity.SSHIdentity{
		PublicKeyFingerprint: "SHA256:oauth-fingerprint",
		PublicKey:            "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIoauth oauth@example.com",
	})
	if err != nil {
		t.Fatalf("create ssh identity: %v", err)
	}

	inserted, err := store.UpsertOAuthAccount(ctx, auth.OAuthAccount{
		SSHIdentityID:  sshIdentity.ID,
		Provider:       "swiggy",
		AccessToken:    "enc-token-1",
		TokenExpiresAt: nil,
		Scopes:         []string{"profile:read"},
		Status:         "reconnect_required",
	})
	if err != nil {
		t.Fatalf("upsert insert oauth account: %v", err)
	}

	if inserted.TokenExpiresAt != nil {
		t.Fatalf("expected nil token_expires_at on insert, got %v", *inserted.TokenExpiresAt)
	}

	foundInserted, err := store.FindOAuthAccountBySSHIdentityAndProvider(ctx, sshIdentity.ID, "swiggy")
	if err != nil {
		t.Fatalf("find inserted oauth account: %v", err)
	}

	if foundInserted.AccessToken != "enc-token-1" {
		t.Fatalf("expected encrypted_access_token enc-token-1, got <redacted>")
	}

	if foundInserted.TokenExpiresAt != nil {
		t.Fatalf("expected nil token_expires_at in find, got %v", *foundInserted.TokenExpiresAt)
	}

	expiresAt := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Microsecond)
	updated, err := store.UpsertOAuthAccount(ctx, auth.OAuthAccount{
		SSHIdentityID:  sshIdentity.ID,
		Provider:       "swiggy",
		AccessToken:    "enc-token-2",
		TokenExpiresAt: &expiresAt,
		Scopes:         []string{"profile:read", "cart:write"},
		Status:         "active",
	})
	if err != nil {
		t.Fatalf("upsert update oauth account: %v", err)
	}

	if updated.TokenExpiresAt == nil {
		t.Fatal("expected non-nil token_expires_at after update")
	}

	if updated.AccessToken != "enc-token-2" {
		t.Fatalf("expected encrypted_access_token enc-token-2, got <redacted>")
	}

	foundUpdated, err := store.FindOAuthAccountBySSHIdentityAndProvider(ctx, sshIdentity.ID, "swiggy")
	if err != nil {
		t.Fatalf("find updated oauth account: %v", err)
	}

	if foundUpdated.TokenExpiresAt == nil {
		t.Fatal("expected non-nil token_expires_at in find after update")
	}

	if foundUpdated.Status != "active" {
		t.Fatalf("expected status active, got %s", foundUpdated.Status)
	}
}

func TestTerminalSessionCreateAndEndLifecycle(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	sshIdentity, err := store.CreateSSHIdentity(ctx, identity.SSHIdentity{
		PublicKeyFingerprint: "SHA256:terminal-fingerprint",
		PublicKey:            "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIterminal terminal@example.com",
	})
	if err != nil {
		t.Fatalf("create ssh identity: %v", err)
	}

	fingerprint := sshIdentity.PublicKeyFingerprint
	selectedAddress := identity.SelectedAddressIDUnsetPlaceholder
	created, err := store.CreateTerminalSession(ctx, identity.TerminalSession{
		SSHIdentityID:     &sshIdentity.ID,
		SSHFingerprint:    &fingerprint,
		Client:            identity.ClientProtocolSSH,
		ClientSessionID:   "test-conn-hex",
		CurrentScreen:     identity.ScreenSSHSessionPlaceholder,
		SelectedAddressID: &selectedAddress,
	})
	if err != nil {
		t.Fatalf("create terminal session: %v", err)
	}

	if created.ID == "" {
		t.Fatal("expected terminal session id")
	}
	if created.SSHIdentityID == nil || *created.SSHIdentityID != sshIdentity.ID {
		t.Fatalf("expected ssh_identity_id %s", sshIdentity.ID)
	}
	if created.CurrentScreen != identity.ScreenSSHSessionPlaceholder {
		t.Fatalf("expected safe current_screen %s, got %s", identity.ScreenSSHSessionPlaceholder, created.CurrentScreen)
	}
	if created.Client != identity.ClientProtocolSSH {
		t.Fatalf("expected client %s, got %s", identity.ClientProtocolSSH, created.Client)
	}
	if created.ClientSessionID != "test-conn-hex" {
		t.Fatalf("expected client_session_id test-conn-hex, got %s", created.ClientSessionID)
	}

	var persistedScreen string
	if err := store.pool.QueryRow(ctx, `SELECT current_screen FROM terminal_sessions WHERE id = $1`, created.ID).Scan(&persistedScreen); err != nil {
		t.Fatalf("query persisted current_screen: %v", err)
	}
	if persistedScreen != identity.ScreenSSHSessionPlaceholder {
		t.Fatalf("expected persisted safe current_screen %s, got %s", identity.ScreenSSHSessionPlaceholder, persistedScreen)
	}

	var persistedClient, persistedClientSessionID string
	if err := store.pool.QueryRow(ctx, `SELECT client, client_session_id FROM terminal_sessions WHERE id = $1`, created.ID).Scan(&persistedClient, &persistedClientSessionID); err != nil {
		t.Fatalf("query persisted client fields: %v", err)
	}
	if persistedClient != identity.ClientProtocolSSH {
		t.Fatalf("expected persisted client %s, got %s", identity.ClientProtocolSSH, persistedClient)
	}
	if persistedClientSessionID != "test-conn-hex" {
		t.Fatalf("expected persisted client_session_id test-conn-hex, got %s", persistedClientSessionID)
	}

	endedAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.MarkTerminalSessionEnded(ctx, created.ID, endedAt); err != nil {
		t.Fatalf("mark terminal session ended: %v", err)
	}

	var gotEndedAt time.Time
	if err := store.pool.QueryRow(ctx, `SELECT ended_at FROM terminal_sessions WHERE id = $1`, created.ID).Scan(&gotEndedAt); err != nil {
		t.Fatalf("query ended_at: %v", err)
	}
	if gotEndedAt.IsZero() {
		t.Fatal("expected ended_at to be set")
	}
}

func TestAttachSSHIdentityToGuestTerminalSession(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	created, err := store.CreateTerminalSession(ctx, identity.TerminalSession{
		Client:          identity.ClientProtocolSSH,
		ClientSessionID: "guest-conn-hex",
		CurrentScreen:   identity.ScreenSSHSessionPlaceholder,
	})
	if err != nil {
		t.Fatalf("create guest terminal session: %v", err)
	}
	if created.SSHIdentityID != nil {
		t.Fatalf("guest session must start without ssh identity, got %s", *created.SSHIdentityID)
	}

	sshIdentity, err := store.CreateSSHIdentity(ctx, identity.SSHIdentity{
		PublicKeyFingerprint: "SHA256:first-login-fingerprint",
		PublicKey:            "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIfirst first@example.com",
	})
	if err != nil {
		t.Fatalf("create ssh identity: %v", err)
	}

	if err := store.AttachSSHIdentityToTerminalSession(ctx, created.ID, sshIdentity.ID); err != nil {
		t.Fatalf("attach ssh identity: %v", err)
	}

	var attachedID string
	if err := store.pool.QueryRow(ctx, `SELECT ssh_identity_id FROM terminal_sessions WHERE id = $1`, created.ID).Scan(&attachedID); err != nil {
		t.Fatalf("query attached identity: %v", err)
	}
	if attachedID != sshIdentity.ID {
		t.Fatalf("expected attached identity %s, got %s", sshIdentity.ID, attachedID)
	}
}

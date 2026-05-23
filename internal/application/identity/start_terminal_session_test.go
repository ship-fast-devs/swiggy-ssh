package identity

import (
	"context"
	"testing"
	"time"
)

type testSessionRepo struct {
	created            TerminalSession
	endedID            string
	endedAt            time.Time
	attachedSessionID  string
	attachedIdentityID string
}

func (r *testSessionRepo) CreateTerminalSession(_ context.Context, session TerminalSession) (TerminalSession, error) {
	session.ID = "session-1"
	now := time.Now().UTC()
	session.CreatedAt = now
	r.created = session
	return session, nil
}

func (r *testSessionRepo) MarkTerminalSessionEnded(_ context.Context, sessionID string, endedAt time.Time) error {
	r.endedID = sessionID
	r.endedAt = endedAt
	return nil
}

func (r *testSessionRepo) AttachSSHIdentityToTerminalSession(_ context.Context, sessionID, sshIdentityID string) error {
	r.attachedSessionID = sessionID
	r.attachedIdentityID = sshIdentityID
	return nil
}

func TestStartTerminalSessionLinksResolvedIdentity(t *testing.T) {
	repo := &testSessionRepo{}
	useCase := NewStartTerminalSessionUseCase(repo)

	sshIdentityID := "identity-1"
	fingerprint := "SHA256:abc"
	selectedAddressID := SelectedAddressIDUnsetPlaceholder

	created, err := useCase.Execute(context.Background(), StartTerminalSessionInput{
		Client:            ClientProtocolSSH,
		ClientSessionID:   "conn-1",
		SSHFingerprint:    &fingerprint,
		CurrentScreen:     ScreenSSHSessionPlaceholder,
		SelectedAddressID: &selectedAddressID,
		ResolvedIdentity:  &SessionIdentity{SSHIdentity: SSHIdentity{ID: sshIdentityID}},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	if created.ID == "" {
		t.Fatal("expected created session id")
	}
	if repo.created.SSHIdentityID == nil || *repo.created.SSHIdentityID != sshIdentityID {
		t.Fatalf("expected ssh_identity_id %s", sshIdentityID)
	}
	if repo.created.CurrentScreen != ScreenSSHSessionPlaceholder {
		t.Fatalf("unexpected current screen: %s", repo.created.CurrentScreen)
	}
}

func TestAttachSSHIdentityToTerminalSession(t *testing.T) {
	repo := &testSessionRepo{}
	useCase := NewAttachSSHIdentityToTerminalSessionUseCase(repo)

	err := useCase.Execute(context.Background(), AttachSSHIdentityToTerminalSessionInput{
		SessionID:     "session-1",
		SSHIdentityID: "identity-1",
	})
	if err != nil {
		t.Fatalf("attach identity: %v", err)
	}
	if repo.attachedSessionID != "session-1" || repo.attachedIdentityID != "identity-1" {
		t.Fatalf("expected attach session-1/identity-1, got %s/%s", repo.attachedSessionID, repo.attachedIdentityID)
	}
}

func TestStartTerminalSessionAllowsGuestIdentity(t *testing.T) {
	repo := &testSessionRepo{}
	useCase := NewStartTerminalSessionUseCase(repo)
	selectedAddressID := SelectedAddressIDUnsetPlaceholder

	_, err := useCase.Execute(context.Background(), StartTerminalSessionInput{
		Client:            ClientProtocolSSH,
		ClientSessionID:   "conn-guest",
		CurrentScreen:     ScreenSSHSessionPlaceholder,
		SelectedAddressID: &selectedAddressID,
	})
	if err != nil {
		t.Fatalf("start guest session: %v", err)
	}

	if repo.created.SSHIdentityID != nil {
		t.Fatalf("guest session must not have ssh_identity_id: %v", *repo.created.SSHIdentityID)
	}
	if repo.created.SSHFingerprint != nil {
		t.Fatalf("no-key guest session must not have ssh fingerprint: %v", *repo.created.SSHFingerprint)
	}
}

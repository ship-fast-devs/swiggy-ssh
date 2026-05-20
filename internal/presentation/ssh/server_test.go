package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"testing"
	"time"

	applicationauth "swiggy-ssh/internal/application/auth"
	applicationidentity "swiggy-ssh/internal/application/identity"

	"golang.org/x/crypto/ssh"
)

type serverAuthRepo struct {
	findCalls          int
	foundSSHIdentityID string
}

func (r *serverAuthRepo) FindOAuthAccountBySSHIdentityAndProvider(_ context.Context, sshIdentityID, _ string) (applicationauth.OAuthAccount, error) {
	r.findCalls++
	r.foundSSHIdentityID = sshIdentityID
	return applicationauth.OAuthAccount{}, applicationauth.ErrOAuthAccountNotFound
}

func (r *serverAuthRepo) UpsertOAuthAccount(context.Context, applicationauth.OAuthAccount) (applicationauth.OAuthAccount, error) {
	return applicationauth.OAuthAccount{}, nil
}

type serverAttemptService struct {
	issued              bool
	issuedSSHIdentityID string
}

func (s *serverAttemptService) IssueAuthAttempt(_ context.Context, sshIdentityID, terminalSessionID string) (string, applicationauth.BrowserAuthAttempt, error) {
	s.issued = true
	s.issuedSSHIdentityID = sshIdentityID
	return "guest-token", applicationauth.BrowserAuthAttempt{
		SSHIdentityID:     sshIdentityID,
		TerminalSessionID: terminalSessionID,
		Status:            applicationauth.AuthAttemptStatusPending,
	}, nil
}

func (s *serverAttemptService) GetAuthAttempt(context.Context, string) (applicationauth.BrowserAuthAttempt, error) {
	return applicationauth.BrowserAuthAttempt{}, nil
}

func (s *serverAttemptService) CompleteAuthAttempt(context.Context, string) error { return nil }

func (s *serverAttemptService) ClaimAuthAttempt(context.Context, string) (applicationauth.BrowserAuthAttempt, error) {
	return applicationauth.BrowserAuthAttempt{}, nil
}

func (s *serverAttemptService) CompleteClaimedAuthAttempt(context.Context, string) error { return nil }

func (s *serverAttemptService) CancelClaimedAuthAttempt(context.Context, string) error { return nil }

func (s *serverAttemptService) CancelAuthAttempt(context.Context, string) error { return nil }

type serverIdentityRepo struct {
	identityByFP map[string]applicationidentity.SSHIdentity
}

type serverSessionRepo struct {
	created                   applicationidentity.TerminalSession
	attachedTerminalSessionID string
	attachedSSHIdentityID     string
}

func newServerIdentityRepo() *serverIdentityRepo {
	return &serverIdentityRepo{identityByFP: map[string]applicationidentity.SSHIdentity{}}
}

func (r *serverIdentityRepo) CreateSSHIdentity(_ context.Context, sshIdentity applicationidentity.SSHIdentity) (applicationidentity.SSHIdentity, error) {
	sshIdentity.ID = "ssh-identity-1"
	sshIdentity.FirstSeenAt = time.Now().UTC()
	r.identityByFP[sshIdentity.PublicKeyFingerprint] = sshIdentity
	return sshIdentity, nil
}

func (r *serverIdentityRepo) FindSSHIdentityByFingerprint(_ context.Context, fingerprint string) (applicationidentity.SSHIdentity, error) {
	sshIdentity, ok := r.identityByFP[fingerprint]
	if !ok {
		return applicationidentity.SSHIdentity{}, applicationidentity.ErrNotFound
	}
	return sshIdentity, nil
}

func (r *serverIdentityRepo) UpdateSSHIdentityLastSeen(_ context.Context, fingerprint string, lastSeenAt time.Time) error {
	sshIdentity := r.identityByFP[fingerprint]
	sshIdentity.LastSeenAt = &lastSeenAt
	r.identityByFP[fingerprint] = sshIdentity
	return nil
}

func (r *serverSessionRepo) CreateTerminalSession(_ context.Context, session applicationidentity.TerminalSession) (applicationidentity.TerminalSession, error) {
	session.ID = "session-1"
	r.created = session
	return session, nil
}

func (r *serverSessionRepo) AttachSSHIdentityToTerminalSession(_ context.Context, sessionID, sshIdentityID string) error {
	r.attachedTerminalSessionID = sessionID
	r.attachedSSHIdentityID = sshIdentityID
	return nil
}

func (r *serverSessionRepo) MarkTerminalSessionEnded(context.Context, string, time.Time) error {
	return nil
}

func TestPublicKeyPermissionsIncludesSafeMetadata(t *testing.T) {
	t.Parallel()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	sshSigner, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("new signer failed: %v", err)
	}

	permissions := publicKeyPermissions(sshSigner.PublicKey())
	if permissions == nil {
		t.Fatalf("permissions should not be nil")
	}

	gotType := permissions.Extensions["pubkey_type"]
	if gotType != sshSigner.PublicKey().Type() {
		t.Fatalf("unexpected key type: got %s want %s", gotType, sshSigner.PublicKey().Type())
	}

	gotFP := permissions.Extensions["pubkey_fingerprint"]
	wantFP := ssh.FingerprintSHA256(sshSigner.PublicKey())
	if gotFP != wantFP {
		t.Fatalf("unexpected fingerprint: got %s want %s", gotFP, wantFP)
	}

	if permissions.Extensions["pubkey_authorized"] == "" {
		t.Fatal("expected authorized public key extension")
	}
}

func TestServerConfigAcceptsNoClientKey(t *testing.T) {
	t.Parallel()

	config := newServerConfig()
	config.AddHostKey(newTestSigner(t))

	listener := newTestListener(t)
	defer listener.Close()

	serverDone := make(chan error, 1)
	go func() {
		serverConn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer serverConn.Close()
		conn, _, _, err := ssh.NewServerConn(serverConn, config)
		if err == nil {
			_ = conn.Close()
		}
		serverDone <- err
	}()

	clientConfig := &ssh.ClientConfig{
		User: "guest",
		Auth: []ssh.AuthMethod{ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			return []string{}, nil
		})},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	clientConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	client, _, _, err := ssh.NewClientConn(clientConn, "test", clientConfig)
	if err != nil {
		t.Fatalf("no-key client handshake: %v", err)
	}
	_ = client.Close()

	if err := <-serverDone; err != nil {
		t.Fatalf("server handshake: %v", err)
	}
}

func TestServerConfigPreservesProvidedClientKeyMetadata(t *testing.T) {
	t.Parallel()

	config := newServerConfig()
	config.AddHostKey(newTestSigner(t))
	clientSigner := newTestSigner(t)
	listener := newTestListener(t)
	defer listener.Close()

	serverDone := make(chan *ssh.Permissions, 1)
	serverErrs := make(chan error, 1)
	go func() {
		serverConn, err := listener.Accept()
		if err != nil {
			serverErrs <- err
			return
		}
		defer serverConn.Close()
		conn, _, _, err := ssh.NewServerConn(serverConn, config)
		if err != nil {
			serverErrs <- err
			return
		}
		serverDone <- conn.Permissions
		_ = conn.Close()
	}()

	clientConfig := &ssh.ClientConfig{
		User:            "known",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	clientConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	client, _, _, err := ssh.NewClientConn(clientConn, "test", clientConfig)
	if err != nil {
		t.Fatalf("public-key client handshake: %v", err)
	}
	_ = client.Close()

	select {
	case err := <-serverErrs:
		t.Fatalf("server handshake: %v", err)
	case permissions := <-serverDone:
		if permissions == nil {
			t.Fatal("expected public key permissions")
		}
		gotFP := permissions.Extensions["pubkey_fingerprint"]
		wantFP := ssh.FingerprintSHA256(clientSigner.PublicKey())
		if gotFP != wantFP {
			t.Fatalf("unexpected fingerprint: got %s want %s", gotFP, wantFP)
		}
	}
}

func TestServerConfigAllowsNoClientKey(t *testing.T) {
	t.Parallel()

	config := newServerConfig()
	if config == nil {
		t.Fatal("expected server config")
	}
	if config.PublicKeyCallback == nil {
		t.Fatal("server config must preserve public key metadata when a key is provided")
	}
	if config.KeyboardInteractiveCallback == nil {
		t.Fatal("server config must allow SSH clients without keys")
	}
}

func TestBeginBrowserAuthForGuestReturnsControlledError(t *testing.T) {
	t.Parallel()

	repo := &serverAuthRepo{}
	attemptSvc := &serverAttemptService{}
	server := &SSHServer{
		authAttemptSvc: attemptSvc,
		publicBaseURL:  "http://localhost:8080",
		authUseCase:    applicationauth.NewEnsureValidAccountUseCase(repo),
	}

	_, err := server.beginBrowserAuth(context.Background(), "", "session-1")
	if !errors.Is(err, applicationauth.ErrSSHIdentityRequired) {
		t.Fatalf("expected ErrSSHIdentityRequired, got %v", err)
	}
	if repo.findCalls != 0 {
		t.Fatalf("expected no oauth lookup for guest auth, got %d", repo.findCalls)
	}
	if attemptSvc.issued {
		t.Fatalf("guest auth attempt must not be issued, got ssh identity id %s", attemptSvc.issuedSSHIdentityID)
	}
}

func TestFirstLoginRegistersSSHIdentityBeforeAuthAttempt(t *testing.T) {
	t.Parallel()

	authRepo := &serverAuthRepo{}
	attemptSvc := &serverAttemptService{}
	identityRepo := newServerIdentityRepo()
	sessionRepo := &serverSessionRepo{}
	server := &SSHServer{
		registrar:      applicationidentity.NewRegisterSSHIdentityUseCase(identityRepo),
		attachSession:  applicationidentity.NewAttachSSHIdentityToTerminalSessionUseCase(sessionRepo),
		authAttemptSvc: attemptSvc,
		publicBaseURL:  "http://localhost:8080",
		authUseCase:    applicationauth.NewEnsureValidAccountUseCase(authRepo),
	}
	signer := newTestSigner(t)
	publicKeyAuthorized := string(ssh.MarshalAuthorizedKey(signer.PublicKey()))

	createdSession, err := sessionRepo.CreateTerminalSession(context.Background(), applicationidentity.TerminalSession{Client: applicationidentity.ClientProtocolSSH, ClientSessionID: "conn-1"})
	if err != nil {
		t.Fatalf("create guest session: %v", err)
	}
	if createdSession.SSHIdentityID != nil {
		t.Fatalf("unknown-key session must start guest, got identity %s", *createdSession.SSHIdentityID)
	}

	sshIdentityID, err := server.establishDurableSSHIdentityForBrowserAuth(context.Background(), "", publicKeyAuthorized, createdSession.ID)
	if err != nil {
		t.Fatalf("ensure durable ssh identity: %v", err)
	}
	if sshIdentityID == "" {
		t.Fatal("expected non-empty durable ssh identity id")
	}
	if sessionRepo.attachedTerminalSessionID != createdSession.ID || sessionRepo.attachedSSHIdentityID != sshIdentityID {
		t.Fatalf("expected session attach %s/%s, got %s/%s", createdSession.ID, sshIdentityID, sessionRepo.attachedTerminalSessionID, sessionRepo.attachedSSHIdentityID)
	}
	result, err := server.beginBrowserAuth(context.Background(), sshIdentityID, createdSession.ID)
	if err != nil {
		t.Fatalf("begin browser auth: %v", err)
	}
	if !result.AuthRequired || !attemptSvc.issued {
		t.Fatal("expected auth attempt to be issued")
	}
	if attemptSvc.issuedSSHIdentityID != sshIdentityID {
		t.Fatalf("expected auth attempt ssh identity %s, got %s", sshIdentityID, attemptSvc.issuedSSHIdentityID)
	}
	if authRepo.foundSSHIdentityID == "" {
		t.Fatal("oauth lookup must use durable ssh identity id")
	}
	if len(identityRepo.identityByFP) != 1 {
		t.Fatalf("expected durable ssh identity, got identities=%d", len(identityRepo.identityByFP))
	}
}

func newTestSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("new signer failed: %v", err)
	}
	return signer
}

func newTestListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return listener
}

func TestParseViewportRequestFromPTY(t *testing.T) {
	t.Parallel()

	req := &ssh.Request{
		Type: "pty-req",
		Payload: ssh.Marshal(ptyRequestPayload{
			Term:          "xterm-256color",
			Columns:       120,
			Rows:          40,
			TerminalModes: "",
		}),
	}
	viewport, ok := parseViewportRequest(req)
	if !ok {
		t.Fatal("expected pty request to parse")
	}
	if viewport.Width != 120 || viewport.Height != 40 {
		t.Fatalf("unexpected viewport: got %+v", viewport)
	}
}

func TestParseViewportRequestFromWindowChange(t *testing.T) {
	t.Parallel()

	req := &ssh.Request{
		Type: "window-change",
		Payload: ssh.Marshal(windowChangePayload{
			Columns: 132,
			Rows:    48,
		}),
	}
	viewport, ok := parseViewportRequest(req)
	if !ok {
		t.Fatal("expected window-change request to parse")
	}
	if viewport.Width != 132 || viewport.Height != 48 {
		t.Fatalf("unexpected viewport: got %+v", viewport)
	}
}

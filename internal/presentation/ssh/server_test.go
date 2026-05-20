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
	domainfood "swiggy-ssh/internal/domain/food"
	domaininstamart "swiggy-ssh/internal/domain/instamart"

	"golang.org/x/crypto/ssh"
)

type serverAuthRepo struct {
	findCalls   int
	foundUserID string
}

func (r *serverAuthRepo) FindOAuthAccountByUserAndProvider(_ context.Context, userID, _ string) (applicationauth.OAuthAccount, error) {
	r.findCalls++
	r.foundUserID = userID
	return applicationauth.OAuthAccount{}, applicationauth.ErrOAuthAccountNotFound
}

func (r *serverAuthRepo) UpsertOAuthAccount(context.Context, applicationauth.OAuthAccount) (applicationauth.OAuthAccount, error) {
	return applicationauth.OAuthAccount{}, nil
}

type serverAttemptService struct {
	issued       bool
	issuedUserID string
}

func (s *serverAttemptService) IssueAuthAttempt(_ context.Context, userID, terminalSessionID string) (string, applicationauth.BrowserAuthAttempt, error) {
	s.issued = true
	s.issuedUserID = userID
	return "guest-token", applicationauth.BrowserAuthAttempt{
		UserID:            userID,
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
	userByID     map[string]applicationidentity.User
	identityByFP map[string]applicationidentity.SSHIdentity
}

func newServerIdentityRepo() *serverIdentityRepo {
	return &serverIdentityRepo{userByID: map[string]applicationidentity.User{}, identityByFP: map[string]applicationidentity.SSHIdentity{}}
}

func (r *serverIdentityRepo) CreateUser(context.Context, applicationidentity.User) (applicationidentity.User, error) {
	panic("unexpected call")
}

func (r *serverIdentityRepo) FindUserByID(_ context.Context, userID string) (applicationidentity.User, error) {
	user, ok := r.userByID[userID]
	if !ok {
		return applicationidentity.User{}, applicationidentity.ErrNotFound
	}
	return user, nil
}

func (r *serverIdentityRepo) UpdateUserLastSeen(_ context.Context, userID string, lastSeenAt time.Time) error {
	user := r.userByID[userID]
	user.LastSeenAt = &lastSeenAt
	r.userByID[userID] = user
	return nil
}

func (r *serverIdentityRepo) CreateSSHIdentity(context.Context, applicationidentity.SSHIdentity) (applicationidentity.SSHIdentity, error) {
	panic("unexpected call")
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

func (r *serverIdentityRepo) CreateUserWithSSHIdentity(_ context.Context, user applicationidentity.User, sshIdentity applicationidentity.SSHIdentity) (applicationidentity.User, applicationidentity.SSHIdentity, error) {
	user.ID = "user-1"
	user.CreatedAt = time.Now().UTC()
	sshIdentity.ID = "ssh-identity-1"
	sshIdentity.UserID = user.ID
	sshIdentity.FirstSeenAt = time.Now().UTC()
	r.userByID[user.ID] = user
	r.identityByFP[sshIdentity.PublicKeyFingerprint] = sshIdentity
	return user, sshIdentity, nil
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

func TestFoodAddressForSelectedUsesFoodSpecificAddressID(t *testing.T) {
	selected := domaininstamart.Address{
		ID:          "im-address-1",
		Label:       "Home",
		DisplayLine: "Test Area",
		Category:    "Home",
		PhoneMasked: "****3210",
	}
	foodAddresses := []domainfood.Address{
		{ID: "food-address-1", Label: "Home", DisplayLine: "Test Area", Category: "Home", PhoneMasked: "****3210"},
	}

	address, ok := foodAddressForSelected(foodAddresses, selected)
	if !ok {
		t.Fatal("expected matching food address")
	}
	if address.ID != "food-address-1" {
		t.Fatalf("expected food-specific address ID, got %q", address.ID)
	}
}

func TestFoodAddressForSelectedDoesNotGuessWhenSelectionCannotBeMatched(t *testing.T) {
	selected := domaininstamart.Address{ID: "im-address-2", Label: "Office"}
	foodAddresses := []domainfood.Address{{ID: "food-address-1", Label: "Home"}}

	_, ok := foodAddressForSelected(foodAddresses, selected)
	if ok {
		t.Fatal("must not guess a Food address when the selected address cannot be matched")
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
	if !errors.Is(err, applicationauth.ErrOAuthAccountUserRequired) {
		t.Fatalf("expected ErrOAuthAccountUserRequired, got %v", err)
	}
	if repo.findCalls != 0 {
		t.Fatalf("expected no oauth lookup for guest auth, got %d", repo.findCalls)
	}
	if attemptSvc.issued {
		t.Fatalf("guest auth attempt must not be issued, got user id %s", attemptSvc.issuedUserID)
	}
}

func TestFirstLoginRegistersSSHIdentityBeforeAuthAttempt(t *testing.T) {
	t.Parallel()

	authRepo := &serverAuthRepo{}
	attemptSvc := &serverAttemptService{}
	identityRepo := newServerIdentityRepo()
	server := &SSHServer{
		registrar:      applicationidentity.NewRegisterSSHIdentityUseCase(identityRepo),
		authAttemptSvc: attemptSvc,
		publicBaseURL:  "http://localhost:8080",
		authUseCase:    applicationauth.NewEnsureValidAccountUseCase(authRepo),
	}
	signer := newTestSigner(t)
	publicKeyAuthorized := string(ssh.MarshalAuthorizedKey(signer.PublicKey()))

	userID, err := server.ensureDurableUserForBrowserAuth(context.Background(), "", publicKeyAuthorized)
	if err != nil {
		t.Fatalf("ensure durable user: %v", err)
	}
	if userID == "" {
		t.Fatal("expected non-empty durable user id")
	}
	result, err := server.beginBrowserAuth(context.Background(), userID, "session-1")
	if err != nil {
		t.Fatalf("begin browser auth: %v", err)
	}
	if !result.AuthRequired || !attemptSvc.issued {
		t.Fatal("expected auth attempt to be issued")
	}
	if attemptSvc.issuedUserID != userID {
		t.Fatalf("expected auth attempt user %s, got %s", userID, attemptSvc.issuedUserID)
	}
	if authRepo.foundUserID == "" {
		t.Fatal("oauth lookup must use durable user id")
	}
	if len(identityRepo.userByID) != 1 || len(identityRepo.identityByFP) != 1 {
		t.Fatalf("expected durable user and ssh identity, got users=%d identities=%d", len(identityRepo.userByID), len(identityRepo.identityByFP))
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

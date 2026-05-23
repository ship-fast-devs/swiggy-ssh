package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type testRepo struct {
	identityByFP    map[string]SSHIdentity
	createCalled    bool
	updatedFP       string
	updatedLastSeen time.Time
}

func newTestRepo() *testRepo {
	return &testRepo{identityByFP: map[string]SSHIdentity{}}
}

func (r *testRepo) FindSSHIdentityByFingerprint(_ context.Context, fingerprint string) (SSHIdentity, error) {
	identity, ok := r.identityByFP[fingerprint]
	if !ok {
		return SSHIdentity{}, ErrNotFound
	}
	return identity, nil
}

func (r *testRepo) CreateSSHIdentity(_ context.Context, sshIdentity SSHIdentity) (SSHIdentity, error) {
	r.createCalled = true
	now := time.Now().UTC()
	sshIdentity.ID = "new-identity"
	sshIdentity.FirstSeenAt = now
	r.identityByFP[sshIdentity.PublicKeyFingerprint] = sshIdentity
	return sshIdentity, nil
}

func (r *testRepo) UpdateSSHIdentityLastSeen(_ context.Context, fingerprint string, lastSeenAt time.Time) error {
	r.updatedFP = fingerprint
	r.updatedLastSeen = lastSeenAt
	identity := r.identityByFP[fingerprint]
	identity.LastSeenAt = &lastSeenAt
	r.identityByFP[fingerprint] = identity
	return nil
}

func newSSHPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return signer.PublicKey()
}

func TestResolveSSHIdentityFoundIdentity(t *testing.T) {
	repo := newTestRepo()
	useCase := NewResolveSSHIdentityUseCase(repo)
	fixedNow := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	useCase.now = func() time.Time { return fixedNow }

	key := newSSHPublicKey(t)
	fingerprint := ssh.FingerprintSHA256(key)
	repo.identityByFP[fingerprint] = SSHIdentity{ID: "i1", PublicKeyFingerprint: fingerprint}

	resolved, err := useCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: "ssh", Key: key})
	if err != nil {
		t.Fatalf("resolve key: %v", err)
	}

	if resolved.SSHIdentity.ID != "i1" {
		t.Fatalf("expected ssh identity i1, got %s", resolved.SSHIdentity.ID)
	}
	if repo.updatedFP != fingerprint {
		t.Fatalf("expected fingerprint update %s, got %s", fingerprint, repo.updatedFP)
	}
	if !repo.updatedLastSeen.Equal(fixedNow) {
		t.Fatalf("expected updated last-seen %v, got %v", fixedNow, repo.updatedLastSeen)
	}
}

func TestResolveSSHIdentityUnknownIdentityReturnsNotFound(t *testing.T) {
	repo := newTestRepo()
	useCase := NewResolveSSHIdentityUseCase(repo)
	key := newSSHPublicKey(t)

	_, err := useCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: "ssh", Key: key})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if repo.createCalled {
		t.Fatal("unknown key must not create ssh identity")
	}
}

func TestResolveSSHIdentityRevokedIdentityRejected(t *testing.T) {
	repo := newTestRepo()
	useCase := NewResolveSSHIdentityUseCase(repo)
	key := newSSHPublicKey(t)
	fingerprint := ssh.FingerprintSHA256(key)
	revokedAt := time.Now().UTC()
	repo.identityByFP[fingerprint] = SSHIdentity{ID: "i1", PublicKeyFingerprint: fingerprint, RevokedAt: &revokedAt}

	_, err := useCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: "ssh", Key: key})
	if !errors.Is(err, ErrSSHIdentityRevoked) {
		t.Fatalf("expected ErrSSHIdentityRevoked, got %v", err)
	}
}

func TestResolveSSHIdentityMissingKeyRejected(t *testing.T) {
	repo := newTestRepo()
	useCase := NewResolveSSHIdentityUseCase(repo)

	_, err := useCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: "ssh"})
	if !errors.Is(err, ErrMissingSSHPublicKey) {
		t.Fatalf("expected ErrMissingSSHPublicKey, got %v", err)
	}
}

func TestRegisterSSHIdentityCreatesDurableIdentityAndReconnectResolvesSameIdentity(t *testing.T) {
	repo := newTestRepo()
	registerUseCase := NewRegisterSSHIdentityUseCase(repo)
	resolveUseCase := NewResolveSSHIdentityUseCase(repo)
	fixedNow := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	registerUseCase.now = func() time.Time { return fixedNow }
	resolveUseCase.now = func() time.Time { return fixedNow.Add(time.Minute) }
	key := newSSHPublicKey(t)

	registered, err := registerUseCase.Execute(context.Background(), RegisterSSHIdentityInput{Client: ClientProtocolSSH, Key: key})
	if err != nil {
		t.Fatalf("register key: %v", err)
	}
	if registered.SSHIdentity.ID == "" {
		t.Fatal("expected durable ssh identity id")
	}

	reconnected, err := resolveUseCase.Execute(context.Background(), ResolveSSHIdentityInput{Client: ClientProtocolSSH, Key: key})
	if err != nil {
		t.Fatalf("resolve reconnected key: %v", err)
	}
	if reconnected.SSHIdentity.ID != registered.SSHIdentity.ID {
		t.Fatalf("expected same ssh identity %s on reconnect, got %s", registered.SSHIdentity.ID, reconnected.SSHIdentity.ID)
	}
}

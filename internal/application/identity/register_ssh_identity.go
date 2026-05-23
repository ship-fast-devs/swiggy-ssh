package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	domainidentity "swiggy-ssh/internal/domain/identity"
)

type RegisterSSHIdentityInput struct {
	Client string
	Key    ssh.PublicKey
	Label  *string
}

type RegisterSSHIdentityUseCase struct {
	repo Repository
	now  func() time.Time
}

func NewRegisterSSHIdentityUseCase(repo Repository) *RegisterSSHIdentityUseCase {
	return &RegisterSSHIdentityUseCase{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// Execute resolves an existing SSH identity or creates a durable identity for the provided key.
func (r *RegisterSSHIdentityUseCase) Execute(ctx context.Context, input RegisterSSHIdentityInput) (SessionIdentity, error) {
	if input.Key == nil {
		return SessionIdentity{}, ErrMissingSSHPublicKey
	}

	fingerprint := ssh.FingerprintSHA256(input.Key)
	registeredAt := r.now()

	sshIdentity, err := r.repo.FindSSHIdentityByFingerprint(ctx, fingerprint)
	if err == nil {
		return r.resolveExistingSSHIdentity(ctx, input.Client, fingerprint, sshIdentity, registeredAt)
	}
	if !errors.Is(err, ErrNotFound) {
		return SessionIdentity{}, err
	}

	sshIdentity, err = r.repo.CreateSSHIdentity(ctx, SSHIdentity{
		PublicKeyFingerprint: fingerprint,
		PublicKey:            strings.TrimSpace(string(ssh.MarshalAuthorizedKey(input.Key))),
		Label:                input.Label,
		LastSeenAt:           &registeredAt,
	})
	if errors.Is(err, ErrSSHIdentityAlreadyExists) {
		sshIdentity, findErr := r.repo.FindSSHIdentityByFingerprint(ctx, fingerprint)
		if findErr != nil {
			return SessionIdentity{}, findErr
		}
		return r.resolveExistingSSHIdentity(ctx, input.Client, fingerprint, sshIdentity, registeredAt)
	}
	if err != nil {
		return SessionIdentity{}, err
	}

	return domainidentity.SessionIdentity{Client: input.Client, SSHIdentity: sshIdentity}, nil
}

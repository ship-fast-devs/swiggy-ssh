package identity

import (
	"context"
	"errors"
	"time"
)

var ErrMissingSSHPublicKey = errors.New("identity: missing ssh public key")
var ErrSSHIdentityRevoked = errors.New("identity: ssh identity revoked")
var ErrNotFound = errors.New("identity: not found")
var ErrSSHIdentityAlreadyExists = errors.New("identity: ssh identity already exists")

// SSHIdentity is the durable app principal backed by an SSH public key.
type SSHIdentity struct {
	ID                   string
	PublicKeyFingerprint string
	PublicKey            string
	Label                *string
	FirstSeenAt          time.Time
	LastSeenAt           *time.Time
	RevokedAt            *time.Time
}

// Repository is the identity persistence boundary.
type Repository interface {
	CreateSSHIdentity(ctx context.Context, sshIdentity SSHIdentity) (SSHIdentity, error)
	FindSSHIdentityByFingerprint(ctx context.Context, fingerprint string) (SSHIdentity, error)
	UpdateSSHIdentityLastSeen(ctx context.Context, fingerprint string, lastSeenAt time.Time) error
}

// SessionIdentity is the resolved principal for an incoming client identity.
type SessionIdentity struct {
	Client      string
	SSHIdentity SSHIdentity
}

const (
	ClientProtocolSSH                 = "ssh"
	ScreenSSHSessionPlaceholder       = "ssh_session_placeholder"
	SelectedAddressIDUnsetPlaceholder = "unselected"
)

type TerminalSession struct {
	ID                string
	Client            string
	ClientSessionID   string
	SSHIdentityID     *string
	SSHFingerprint    *string
	CurrentScreen     string
	SelectedAddressID *string
	CreatedAt         time.Time
	LastSeenAt        *time.Time
	EndedAt           *time.Time
}

type SessionRepository interface {
	CreateTerminalSession(ctx context.Context, session TerminalSession) (TerminalSession, error)
	AttachSSHIdentityToTerminalSession(ctx context.Context, sessionID, sshIdentityID string) error
	MarkTerminalSessionEnded(ctx context.Context, sessionID string, endedAt time.Time) error
}

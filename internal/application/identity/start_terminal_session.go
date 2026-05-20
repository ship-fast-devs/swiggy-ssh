package identity

import (
	"context"

	domainidentity "swiggy-ssh/internal/domain/identity"
)

type TerminalSession = domainidentity.TerminalSession
type SessionRepository = domainidentity.SessionRepository

const (
	ClientProtocolSSH                 = domainidentity.ClientProtocolSSH
	ScreenSSHSessionPlaceholder       = domainidentity.ScreenSSHSessionPlaceholder
	SelectedAddressIDUnsetPlaceholder = domainidentity.SelectedAddressIDUnsetPlaceholder
)

type StartTerminalSessionInput struct {
	Client            string
	ClientSessionID   string
	SSHFingerprint    *string
	CurrentScreen     string
	SelectedAddressID *string
	ResolvedIdentity  *SessionIdentity
}

type StartTerminalSessionUseCase struct {
	repo SessionRepository
}

func NewStartTerminalSessionUseCase(repo SessionRepository) *StartTerminalSessionUseCase {
	return &StartTerminalSessionUseCase{
		repo: repo,
	}
}

func (uc *StartTerminalSessionUseCase) Execute(ctx context.Context, input StartTerminalSessionInput) (TerminalSession, error) {
	var sshIdentityID *string
	if input.ResolvedIdentity != nil {
		sshIdentityID = &input.ResolvedIdentity.SSHIdentity.ID
	}

	return uc.repo.CreateTerminalSession(ctx, TerminalSession{
		Client:            input.Client,
		ClientSessionID:   input.ClientSessionID,
		SSHIdentityID:     sshIdentityID,
		SSHFingerprint:    input.SSHFingerprint,
		CurrentScreen:     input.CurrentScreen,
		SelectedAddressID: input.SelectedAddressID,
	})
}

package identity

import "context"

type AttachSSHIdentityToTerminalSessionInput struct {
	SessionID     string
	SSHIdentityID string
}

type AttachSSHIdentityToTerminalSessionUseCase struct {
	repo SessionRepository
}

func NewAttachSSHIdentityToTerminalSessionUseCase(repo SessionRepository) *AttachSSHIdentityToTerminalSessionUseCase {
	return &AttachSSHIdentityToTerminalSessionUseCase{repo: repo}
}

func (uc *AttachSSHIdentityToTerminalSessionUseCase) Execute(ctx context.Context, input AttachSSHIdentityToTerminalSessionInput) error {
	return uc.repo.AttachSSHIdentityToTerminalSession(ctx, input.SessionID, input.SSHIdentityID)
}

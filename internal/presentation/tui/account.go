package tui

import (
	"context"
	"fmt"
	"io"

	"swiggy-ssh/internal/application/auth"
)

// AccountHomeView renders the minimal account status for a returning user.
// Inline (no full-screen box): shown as a brief status block before the main menu.
type AccountHomeView struct {
	SSHFingerprint string
	Account        auth.OAuthAccount
}

func (v AccountHomeView) Render(ctx context.Context, w io.Writer) error {
	status := v.Account.Status
	if status == "" {
		status = auth.OAuthAccountStatusActive
	}

	var statusRendered string
	switch status {
	case auth.OAuthAccountStatusActive:
		statusRendered = connStyle.Render(status)
	case auth.OAuthAccountStatusRevoked:
		statusRendered = errorStyle.Render(status)
	default:
		statusRendered = accentStyle.Render(status)
	}

	content := "\r\n" +
		"  ┌────────────────────────────────────────┐\r\n" +
		"  │       " + brandStyle.Render("GET /account") + creamStyle.Render(" 200 OK") + "             │\r\n" +
		"  └────────────────────────────────────────┘\r\n" +
		"\r\n" +
		"  " + mutedStyle.Render("response:") + "\r\n" +
		"  " + brandStyle.Render("ssh_key :") + " " + creamStyle.Render(v.SSHFingerprint) + "\r\n" +
		"  " + brandStyle.Render("provider:") + " " + creamStyle.Render("swiggy") + "\r\n" +
		"  " + brandStyle.Render("status  :") + " " + statusRendered + "\r\n"
	_, err := fmt.Fprint(w, centerInViewport(content, viewportFromContext(ctx)))
	return err
}

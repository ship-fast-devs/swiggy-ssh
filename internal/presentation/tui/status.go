package tui

import (
	"context"
	"fmt"
	"io"
)

// RevokedView renders the account-revoked message.
type RevokedView struct{}

func (v RevokedView) Render(ctx context.Context, w io.Writer) error {
	content := "\r\n" +
		"  " + errorStyle.Render("GET /account 403 Forbidden") + "\r\n" +
		"  " + mutedStyle.Render("error:") + "\r\n" +
		"  " + brandStyle.Render("code   :") + " " + errorStyle.Render("account_revoked") + "\r\n" +
		"  " + brandStyle.Render("message:") + " " + creamStyle.Render("Your account access has been revoked. Please contact support.") + "\r\n"
	_, err := fmt.Fprint(w, centerInViewport(content, viewportFromContext(ctx)))
	return err
}

// ErrorView renders a generic recoverable error message.
type ErrorView struct {
	Message string
}

func (v ErrorView) Render(ctx context.Context, w io.Writer) error {
	content := "\r\n" +
		"  " + errorStyle.Render("GET /account 500 Internal Server Error") + "\r\n" +
		"  " + mutedStyle.Render("error:") + "\r\n" +
		"  " + brandStyle.Render("code   :") + " " + errorStyle.Render("request_failed") + "\r\n" +
		"  " + brandStyle.Render("message:") + " " + creamStyle.Render(v.Message) + "\r\n"
	_, err := fmt.Fprint(w, centerInViewport(content, viewportFromContext(ctx)))
	return err
}

// Compile-time assertions that all views implement View.
var (
	_ View = HomeView{}
	_ View = LoginWaitingView{}
	_ View = LoginSuccessView{}
	_ View = ReconnectView{}
	_ View = AccountHomeView{}
	_ View = InstamartPlaceholderView{}
	_ View = InstamartView{}
	_ View = RevokedView{}
	_ View = ErrorView{}
)

package tui_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"swiggy-ssh/internal/application/auth"
	"swiggy-ssh/internal/presentation/tui"
)

// renderCtx returns a context with a short timeout suitable for interactive views.
// Interactive Bubbletea models block until ctx.Done() fires; the timeout ensures
// the test program exits promptly.
func renderCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 200*time.Millisecond)
}

// --- HomeView ---

func TestHomeViewRendersMenuItems(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.HomeView{}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"swiggy.ssh",
		"Instamart",
		"Food",
		"Reorder usuals",
		"coming soon",
		"j/k move",
		"● Connected SSH",
		"⣿⣿⣿", // braille logo
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in HomeView output", want)
		}
	}
}

func TestHomeViewUsesSwiggyOrange(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	var buf bytes.Buffer
	if err := (tui.HomeView{}).Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(buf.String(), "\x1b[38;2;252;128;25m") {
		t.Fatal("expected home view to render Swiggy orange ANSI color")
	}
}

func TestHomeViewCentersInViewport(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	ctx = tui.WithViewport(ctx, tui.Viewport{Width: 100, Height: 30})

	var buf bytes.Buffer
	if err := (tui.HomeView{}).Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	out := buf.String()
	// Viewport centering adds horizontal padding — box should not start at column 0.
	if !strings.Contains(out, "          ┌") {
		t.Fatal("expected home view to be horizontally padded in viewport")
	}
}

func TestInstamartPlaceholderViewUsesInput(t *testing.T) {
	var buf bytes.Buffer
	err := tui.InstamartPlaceholderView{
		SSHIdentityID: "identity-1",
		In:            strings.NewReader("q"),
	}.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "Instamart") {
		t.Fatal("expected Instamart screen output")
	}
}

// --- LoginWaitingView ---

func TestLoginWaitingViewRendersDirectURL(t *testing.T) {
	// LoginWaitingView is static — Init returns tea.Quit immediately.
	v := tui.LoginWaitingView{LoginURL: "http://localhost:8080/auth/start?attempt=opaque"}
	var buf bytes.Buffer
	if err := v.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"http://localhost:8080/auth/start?attempt=opaque",
		"one-time link",
		"Waiting for browser login",
		"not connected",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in LoginWaitingView output", want)
		}
	}
}

func TestLoginWaitingViewURLAppearsOnce(t *testing.T) {
	loginURL := "http://localhost:8080/auth/start?attempt=opaque"
	v := tui.LoginWaitingView{LoginURL: loginURL}
	var buf bytes.Buffer
	_ = v.Render(context.Background(), &buf)
	out := buf.String()
	if !strings.Contains(out, "\x1b]8;;"+loginURL) {
		t.Fatal("expected OSC-8 hyperlink target to include URL")
	}
	if !strings.Contains(out, loginURL) {
		t.Fatal("expected fallback URL to appear")
	}
}

func TestLoginWaitingViewWrapsLongURL(t *testing.T) {
	loginURL := "http://localhost:8080/auth/start?attempt=" + strings.Repeat("abcdef", 18)
	v := tui.LoginWaitingView{LoginURL: loginURL}
	var buf bytes.Buffer
	_ = v.Render(context.Background(), &buf)
	out := buf.String()
	if !strings.Contains(out, loginURL[:70]) || !strings.Contains(out, loginURL[70:]) {
		t.Fatal("expected long URL to be wrapped instead of clipped")
	}
}

func TestLoginWaitingViewCopyKeyEmitsOSC52WhenInteractive(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	loginURL := "http://localhost:8080/auth/start?attempt=opaque"
	v := tui.LoginWaitingView{LoginURL: loginURL, In: strings.NewReader("c")}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(loginURL))
	if !strings.Contains(out, want) {
		t.Fatal("expected c key to emit OSC-52 clipboard sequence")
	}
	if !strings.Contains(out, "copy attempted") {
		t.Fatal("expected visible copy attempted status")
	}
}

func TestLoginWaitingViewInteractiveFooterShowsCopyAndClickHints(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.LoginWaitingView{
		LoginURL: "http://localhost:8080/auth/start?attempt=opaque",
		In:       strings.NewReader(""),
	}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"c copy URL", "click Open Swiggy login"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected footer hint %q", want)
		}
	}
}

// --- LoginSuccessView ---

func TestLoginSuccessViewShowsSignedInAs(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.LoginSuccessView{
		IsFirstAuth: true,
		DisplayName: "Alice",
		Email:       "alice@example.com",
	}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"✓ Swiggy account connected",
		"Alice",
		"alice@example.com",
		"Instamart",
		"j/k move",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in LoginSuccessView output", want)
		}
	}
	// AccessToken must never appear
	if strings.Contains(out, v.Account.AccessToken) && v.Account.AccessToken != "" {
		t.Fatal("access token must not appear in TUI output")
	}
}

func TestLoginSuccessViewEmptyDisplayName(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.LoginSuccessView{DisplayName: "", Email: ""}
	var buf bytes.Buffer
	_ = v.Render(ctx, &buf)
	out := buf.String()

	if !strings.Contains(out, "Unknown") {
		t.Fatal("expected 'Unknown' when DisplayName is empty")
	}
	if !strings.Contains(out, "(no email)") {
		t.Fatal("expected '(no email)' when Email is empty")
	}
}

func TestLoginSuccessViewFirstAuth(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.LoginSuccessView{IsFirstAuth: true, DisplayName: "Sujith", Email: "sujith@example.com"}
	var buf bytes.Buffer
	_ = v.Render(ctx, &buf)
	if !strings.Contains(buf.String(), "✓ Swiggy account connected") {
		t.Fatal("expected connected message in first-auth LoginSuccessView")
	}
}

func TestLoginSuccessViewWasReauth(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.LoginSuccessView{WasReauth: true, DisplayName: "Sujith", Email: "sujith@example.com"}
	var buf bytes.Buffer
	_ = v.Render(ctx, &buf)
	if !strings.Contains(buf.String(), "✓ Swiggy account connected") {
		t.Fatal("expected connected message in reauth LoginSuccessView")
	}
}

func TestLoginSuccessViewWelcomeBack(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.LoginSuccessView{DisplayName: "Sujith", Email: "sujith@example.com"}
	var buf bytes.Buffer
	_ = v.Render(ctx, &buf)
	if !strings.Contains(buf.String(), "✓ Swiggy account connected") {
		t.Fatal("expected connected message in LoginSuccessView")
	}
}

// --- InstamartView ---

func TestInstamartViewRendersAddress(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.InstamartView{
		AddressLabel:  "Work",
		AddressLine:   "Koramangala, Bengaluru",
		CartItemCount: 3,
	}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"Work",
		"Koramangala",
		"3 items",
		"Search products",
		"/ search",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in InstamartView output", want)
		}
	}
}

// --- InstamartPlaceholderView ---

func TestInstamartPlaceholderViewDelegates(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.InstamartPlaceholderView{SSHIdentityID: "identity-1", StatusMessage: "Guest session connected"}
	var buf bytes.Buffer
	_ = v.Render(ctx, &buf)
	out := buf.String()

	for _, want := range []string{
		"Instamart",
		"Search products",
		"Home", // default address label
		"Guest session connected",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in InstamartPlaceholderView output", want)
		}
	}
}

// --- AccountHomeView ---

func TestAccountHomeViewShowsFingerprint(t *testing.T) {
	// AccountHomeView is static (plain fmt.Fprint, no tea.Program).
	expiresAt := time.Now().Add(time.Hour)
	v := tui.AccountHomeView{
		SSHFingerprint: "SHA256:abcdef",
		Account: auth.OAuthAccount{
			AccessToken:    "super-secret-token-abc123",
			Status:         auth.OAuthAccountStatusActive,
			TokenExpiresAt: &expiresAt,
		},
	}
	var buf bytes.Buffer
	_ = v.Render(context.Background(), &buf)
	out := buf.String()
	if !strings.Contains(out, "SHA256:abcdef") {
		t.Fatal("expected fingerprint in output")
	}
	if !strings.Contains(out, "active") {
		t.Fatal("expected active status in output")
	}
	// AccessToken must not appear in TUI output
	if strings.Contains(out, v.Account.AccessToken) {
		t.Fatal("access token must not appear in TUI output")
	}
}

// --- RevokedView ---

func TestRevokedViewRenders(t *testing.T) {
	v := tui.RevokedView{}
	var buf bytes.Buffer
	_ = v.Render(context.Background(), &buf)
	if !strings.Contains(buf.String(), "revoked") {
		t.Fatal("expected revoked message")
	}
}

// --- ReconnectView ---

func TestReconnectViewRendersDirectURL(t *testing.T) {
	// ReconnectView is inline plain text — no tea.Program.
	loginURL := "http://localhost:8080/auth/start?attempt=reco"
	v := tui.ReconnectView{LoginURL: loginURL}
	var buf bytes.Buffer
	if err := v.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, loginURL) {
		t.Fatal("expected URL in reconnect view output")
	}
	if !strings.Contains(out, "\x1b]8;;"+loginURL) {
		t.Fatal("expected OSC-8 hyperlink target to include URL")
	}
}

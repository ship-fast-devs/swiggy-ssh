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

func TestHomeViewRendersSplash(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.HomeView{}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"swiggy.dev",
		"enter continue",
		"▸",
		"auth required",
		"⣿⣿⣿", // braille logo
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in HomeView output", want)
		}
	}
	if got := strings.Count(out, "\r\n"); got != 24 {
		t.Fatalf("expected home view to use fixed 80x24 frame, got %d rows", got)
	}
	if strings.Contains(out, "press enter to continue") {
		t.Fatal("did not expect duplicate body continue prompt on splash")
	}
	for _, removed := range []string{"Instamart", "Food", "Reorder usuals", "Account"} {
		if strings.Contains(out, removed) {
			t.Fatalf("did not expect menu item %q on splash", removed)
		}
	}
}

func TestHomeViewRendersSelectedAddressReadiness(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.HomeView{Session: tui.HomeSessionState{
		Authenticated:        true,
		AddressStatus:        tui.HomeAddressSelected,
		SelectedAddressIndex: 0,
		Addresses:            []tui.HomeAddressOption{{ID: "addr-1", Label: "Home"}},
	}}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "deploying to") || !strings.Contains(buf.String(), "Home") {
		t.Fatalf("expected selected address readiness, got %q", buf.String())
	}
}

func TestHomeAddressPickerDoesNotRenderFullAddress(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.HomeView{
		Session: tui.HomeSessionState{
			Authenticated:        true,
			AddressStatus:        tui.HomeAddressSelected,
			SelectedAddressIndex: 0,
			Addresses: []tui.HomeAddressOption{{
				ID:          "addr-1",
				Label:       "Home",
				DisplayLine: "Flat 42, Secret Street, Bengaluru",
				PhoneMasked: "****6438",
			}},
		},
		In: strings.NewReader("\r3"),
	}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Home", "****6438", "Flat 42"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in address picker output", want)
		}
	}
	if strings.Contains(out, "Secret Street") || strings.Contains(out, "Bengaluru") {
		t.Fatalf("full address must not render in root address picker: %q", out)
	}
}

func TestHomeViewRendersMenuItemsAfterContinue(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	v := tui.HomeView{In: strings.NewReader("\r")}
	var buf bytes.Buffer
	if err := v.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{"Instamart", "Food", "swiggy.ai", "coming soon", "j/k move"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in HomeView menu output", want)
		}
	}
	for _, removed := range []string{"Reorder usuals", "Account", "press enter to continue"} {
		if strings.Contains(out, removed) {
			t.Fatalf("did not expect %q in menu output", removed)
		}
	}
	if got := strings.Count(out, "\r\n"); got != 24 {
		t.Fatalf("expected menu to use fixed 80x24 frame, got %d rows", got)
	}
}

func TestHomeViewUsesLogoGradient(t *testing.T) {
	ctx, cancel := renderCtx()
	defer cancel()
	var buf bytes.Buffer
	if err := (tui.HomeView{}).Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(buf.String(), "\x1b[38;2;233;113;18m") {
		t.Fatal("expected home logo gradient to render #E97112")
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
		UserID: "user-1",
		In:     strings.NewReader("q"),
	}.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "env=instamart") {
		t.Fatal("expected Instamart flow output")
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
		"enter continue",
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
		"cart=3",
		"grep products",
		"/ grep",
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
	v := tui.InstamartPlaceholderView{UserID: "user-1", StatusMessage: "Guest session connected"}
	var buf bytes.Buffer
	_ = v.Render(ctx, &buf)
	out := buf.String()

	for _, want := range []string{
		"env=instamart",
		"grep products",
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

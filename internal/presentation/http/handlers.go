package http

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"

	"swiggy-ssh/internal/application/auth"
)

var tmplLoginInfo = template.Must(template.New("login_info").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>swiggy.dev · Complete Login</title>
  <style>
    :root { color-scheme: dark; --bg: #0d0f0a; --panel: #15180f; --ink: #fff7ed; --muted: #b9b09f; --orange: #fc8019; --green: #52ff8f; --line: #3a3326; }
    * { box-sizing: border-box; }
    body { min-height: 100vh; margin: 0; display: grid; place-items: center; padding: 24px; background: radial-gradient(circle at top left, rgba(252, 128, 25, .20), transparent 34%), linear-gradient(135deg, #090a07, var(--bg)); color: var(--ink); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace; }
    .terminal { width: min(720px, 100%); border: 1px solid var(--line); border-radius: 18px; background: rgba(21, 24, 15, .94); box-shadow: 0 24px 80px rgba(0,0,0,.45), inset 0 0 0 1px rgba(255,255,255,.03); overflow: hidden; }
    .bar { display: flex; gap: 8px; align-items: center; padding: 14px 18px; border-bottom: 1px solid var(--line); background: rgba(0,0,0,.22); }
    .dot { width: 12px; height: 12px; border-radius: 999px; background: var(--orange); box-shadow: 20px 0 #ffd166, 40px 0 var(--green); }
    .title { margin-left: 48px; color: var(--muted); font-size: .82rem; }
    .screen { padding: clamp(22px, 5vw, 42px); }
    .prompt { color: var(--green); font-weight: 700; letter-spacing: .04em; }
    h1 { margin: 10px 0 14px; color: var(--orange); font-size: clamp(1.7rem, 5vw, 3rem); line-height: 1.05; }
    p { color: var(--muted); line-height: 1.65; margin: 0 0 18px; }
    .command { display: inline-block; margin: 12px 0 22px; padding: 10px 12px; border: 1px dashed rgba(252,128,25,.55); border-radius: 10px; color: var(--ink); background: rgba(252,128,25,.08); }
    .cursor { display: inline-block; width: .65em; height: 1em; vertical-align: -0.15em; background: var(--green); animation: blink 1s steps(1) infinite; }
    .pun { color: var(--ink); }
    @keyframes blink { 50% { opacity: 0; } }
    @media (max-width: 520px) { body { padding: 14px; place-items: start center; } .terminal { border-radius: 14px; } .title { display: none; } .screen { padding: 22px; } }
  </style>
</head>
<body>
	<main class="terminal" aria-labelledby="page-title">
		<div class="bar"><span class="dot" aria-hidden="true"></span><span class="title">swiggy.dev · Browser Login</span></div>
		<section class="screen">
			<div class="prompt">guest@swiggy.dev:~$ ssh swiggy.dev</div>
			<h1 id="page-title">HTTP? Not in this checkout path.</h1>
			<p class="pun">This is a terminal-first Swiggy run. Your order lives over SSH; this page only signs the login packet from the direct URL printed in your terminal.</p>
			<div class="command">ssh swiggy.dev <span class="cursor" aria-hidden="true"></span></div>
			<p>If you landed here directly, your auth link probably exited with code 1. Go back to SSH, choose Instamart, and spawn a fresh login process.</p>
		</section>
	</main>
</body>
</html>
`))

var tmplLoginSuccess = template.Must(template.New("login_success").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>swiggy.dev · Login Complete</title>
  <style>
    :root { color-scheme: dark; --bg: #0d0f0a; --panel: #15180f; --ink: #fff7ed; --muted: #b9b09f; --orange: #fc8019; --green: #52ff8f; --line: #3a3326; }
    * { box-sizing: border-box; }
    body { min-height: 100vh; margin: 0; display: grid; place-items: center; padding: 24px; background: radial-gradient(circle at 80% 0%, rgba(82, 255, 143, .16), transparent 32%), linear-gradient(135deg, #090a07, var(--bg)); color: var(--ink); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace; text-align: center; }
    .terminal { width: min(720px, 100%); border: 1px solid var(--line); border-radius: 18px; background: rgba(21, 24, 15, .94); box-shadow: 0 24px 80px rgba(0,0,0,.45), inset 0 0 0 1px rgba(255,255,255,.03); overflow: hidden; }
    .bar { display: flex; gap: 8px; align-items: center; padding: 14px 18px; border-bottom: 1px solid var(--line); background: rgba(0,0,0,.22); }
    .dot { width: 12px; height: 12px; border-radius: 999px; background: var(--orange); box-shadow: 20px 0 #ffd166, 40px 0 var(--green); }
    .title { margin-left: 48px; color: var(--muted); font-size: .82rem; }
    .screen { padding: clamp(26px, 6vw, 52px); }
    .stamp { display: inline-grid; place-items: center; width: 88px; height: 88px; border: 2px solid var(--green); border-radius: 24px; color: var(--green); font-size: 3rem; box-shadow: 0 0 34px rgba(82,255,143,.18); transform: rotate(-4deg); }
    .prompt { margin-top: 24px; color: var(--green); font-weight: 700; letter-spacing: .04em; }
    .sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
    h1 { margin: 12px 0 14px; color: var(--orange); font-size: clamp(1.8rem, 5vw, 3.2rem); line-height: 1.05; }
    p { color: var(--muted); line-height: 1.65; margin: 0 auto 14px; max-width: 520px; }
    .pun { color: var(--ink); }
    .cursor { display: inline-block; width: .65em; height: 1em; vertical-align: -0.15em; background: var(--green); animation: blink 1s steps(1) infinite; }
    @keyframes blink { 50% { opacity: 0; } }
    @media (max-width: 520px) { body { padding: 14px; place-items: start center; } .terminal { border-radius: 14px; } .title { display: none; } .screen { padding: 24px; } }
  </style>
</head>
<body>
  <main class="terminal" aria-labelledby="page-title">
    <div class="bar"><span class="dot" aria-hidden="true"></span><span class="title">swiggy.dev/auth/delivery-confirmed</span></div>
		<section class="screen">
			<div class="stamp" aria-hidden="true">✓</div>
			<p class="sr-only">Login complete</p>
			<div class="prompt">auth@swiggy.dev:~$ swiggy login --complete</div>
			<h1 id="page-title">Swiggy linked. Shell hungry.</h1>
			<p class="pun">Auth passed, token cached, cart process resumed. Return to SSH to grep groceries, pipe them into your cart, and commit the order.</p>
			<p>You can close this tab. The real UI is still running in your terminal.</p>
			<p aria-hidden="true">resuming_ssh_order_loop <span class="cursor"></span></p>
		</section>
	</main>
</body>
</html>
`))

var tmplLoginError = template.Must(template.New("login_error").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>swiggy.dev · Login Error</title>
  <style>
    :root { color-scheme: dark; --bg: #0d0f0a; --panel: #15180f; --ink: #fff7ed; --muted: #b9b09f; --orange: #fc8019; --red: #ff6b6b; --line: #3a3326; }
    * { box-sizing: border-box; }
    body { min-height: 100vh; margin: 0; display: grid; place-items: center; padding: 24px; background: radial-gradient(circle at top right, rgba(255, 107, 107, .16), transparent 34%), linear-gradient(135deg, #090a07, var(--bg)); color: var(--ink); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace; }
    .terminal { width: min(720px, 100%); border: 1px solid var(--line); border-radius: 18px; background: rgba(21, 24, 15, .94); box-shadow: 0 24px 80px rgba(0,0,0,.45), inset 0 0 0 1px rgba(255,255,255,.03); overflow: hidden; }
    .bar { display: flex; gap: 8px; align-items: center; padding: 14px 18px; border-bottom: 1px solid var(--line); background: rgba(0,0,0,.22); }
    .dot { width: 12px; height: 12px; border-radius: 999px; background: var(--red); box-shadow: 20px 0 #ffd166, 40px 0 var(--orange); }
    .title { margin-left: 48px; color: var(--muted); font-size: .82rem; }
    .screen { padding: clamp(22px, 5vw, 42px); }
    .prompt { color: var(--red); font-weight: 700; letter-spacing: .04em; }
    h1 { margin: 10px 0 14px; color: var(--orange); font-size: clamp(1.7rem, 5vw, 3rem); line-height: 1.05; }
    p { color: var(--muted); line-height: 1.65; margin: 0 0 18px; }
    .error-box { background: rgba(255, 107, 107, .10); border: 1px dashed rgba(255, 107, 107, .65); border-radius: 12px; padding: 16px; color: var(--ink); margin: 18px 0 24px; line-height: 1.55; }
    a { color: var(--orange); font-weight: 700; text-decoration-thickness: 2px; text-underline-offset: 4px; }
    @media (max-width: 520px) { body { padding: 14px; place-items: start center; } .terminal { border-radius: 14px; } .title { display: none; } .screen { padding: 22px; } }
  </style>
</head>
<body>
	<main class="terminal" aria-labelledby="page-title">
		<div class="bar"><span class="dot" aria-hidden="true"></span><span class="title">swiggy.dev/auth/oops-all-stacktrace</span></div>
		<section class="screen">
			<div class="prompt">auth@swiggy.dev:~$ swiggy login --retry</div>
			<h1 id="page-title">Auth segfaulted before snacks.</h1>
			<p>This login link could not complete the Swiggy handshake. No cart was corrupted; no token was printed to stdout.</p>
			<div class="error-box">{{.Message}}</div>
			<p><a href="/login">cd terminal && retry login</a></p>
		</section>
	</main>
</body>
</html>
`))

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	attempt := r.URL.Query().Get("attempt")
	if attempt != "" {
		http.Redirect(w, r, "/auth/start?attempt="+url.QueryEscape(attempt), http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmplLoginInfo.Execute(w, nil)
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	attempt := r.URL.Query().Get("attempt")
	if attempt == "" && r.ParseForm() == nil {
		attempt = r.FormValue("attempt")
	}
	if attempt == "" {
		s.renderError(w, "Login codes are no longer used. Open the direct login URL shown in your terminal.")
		return
	}
	http.Redirect(w, r, "/auth/start?attempt="+url.QueryEscape(attempt), http.StatusFound)
}

func (s *Server) handleAuthStart(w http.ResponseWriter, r *http.Request) {
	attempt := r.URL.Query().Get("attempt")
	if attempt == "" {
		s.renderError(w, "Missing auth attempt. Please start a new SSH session.")
		return
	}

	if s.provider != "mock" {
		started, err := s.startProviderAuth(r, attempt)
		if err == nil {
			http.Redirect(w, r, started.RedirectURL, http.StatusFound)
			return
		}
		s.renderAuthError(w, r, err)
		return
	}

	// Mock/dev mode completes the one-time attempt directly and lets the terminal continue.
	err := s.completeAttempt(r, attempt)
	if err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmplLoginSuccess.Execute(w, nil)
		return
	}

	switch {
	case errors.Is(err, auth.ErrAuthAttemptNotFound):
		s.renderError(w, "Auth attempt not found or expired. Please start a new SSH session.")
	case errors.Is(err, auth.ErrAuthAttemptAlreadyUsed):
		s.renderError(w, "Auth attempt has already been used. Please start a new SSH session.")
	default:
		s.logger.WarnContext(r.Context(), "complete auth attempt unexpected error", "error", err)
		s.renderError(w, "Something went wrong. Please try again.")
	}
}

func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	attempt := r.URL.Query().Get("state")
	if attempt == "" && s.provider == "mock" {
		attempt = r.URL.Query().Get("attempt")
	}
	if attempt == "" {
		s.renderError(w, "Missing auth state. Please start a new SSH session.")
		return
	}
	code := r.URL.Query().Get("code")
	if s.provider != "mock" && code == "" {
		s.renderError(w, "Missing Swiggy authorization code. Please start a new SSH session.")
		return
	}
	record, err := s.svc.GetAuthAttempt(r.Context(), attempt)
	if err != nil {
		s.renderAuthError(w, r, err)
		return
	}
	if record.Status != auth.AuthAttemptStatusPending {
		s.renderAuthError(w, r, auth.ErrAuthAttemptAlreadyUsed)
		return
	}
	if s.provider != "mock" {
		if s.auth == nil {
			s.renderAuthError(w, r, auth.ErrBrowserAuthProviderUnavailable)
			return
		}
		_, err = s.auth.ExecuteCallback(r.Context(), auth.BrowserAuthCallbackInput{
			State:       attempt,
			Code:        code,
			CallbackURL: s.publicBaseURL + "/auth/callback",
		})
		if err != nil {
			s.renderAuthError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmplLoginSuccess.Execute(w, nil)
		return
	}
	err = s.completeAttempt(r, attempt)
	if err != nil {
		s.renderAuthError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmplLoginSuccess.Execute(w, nil)
}

func (s *Server) startProviderAuth(r *http.Request, attempt string) (auth.StartBrowserAuthOutput, error) {
	if s.startAuth == nil {
		return auth.StartBrowserAuthOutput{}, auth.ErrBrowserAuthProviderUnavailable
	}
	return s.startAuth.Execute(r.Context(), auth.StartBrowserAuthInput{
		AttemptToken: attempt,
		CallbackURL:  s.publicBaseURL + "/auth/callback",
	})
}

func (s *Server) completeAttempt(r *http.Request, attempt string) error {
	if s.auth != nil {
		_, err := s.auth.Execute(r.Context(), attempt)
		return err
	}
	return s.svc.CompleteAuthAttempt(r.Context(), attempt)
}

func (s *Server) renderAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrAuthAttemptNotFound):
		s.renderError(w, "Auth attempt not found or expired. Please start a new SSH session.")
	case errors.Is(err, auth.ErrAuthAttemptAlreadyUsed):
		s.renderError(w, "Auth attempt has already been used. Please start a new SSH session.")
	case errors.Is(err, auth.ErrBrowserAuthProviderUnavailable):
		s.renderError(w, "Swiggy browser login is not configured yet. Please use mock provider for local development.")
	case errors.Is(err, auth.ErrBrowserAuthProviderCallback):
		s.renderError(w, "Swiggy login callback could not be completed. Please start a new SSH session.")
	case errors.Is(err, auth.ErrSSHIdentityRequired):
		s.renderError(w, "This auth link is not attached to a durable SSH identity. Reconnect with a known SSH key and try again.")
	default:
		s.logger.WarnContext(r.Context(), "browser auth unexpected error", "error", err)
		s.renderError(w, "Something went wrong. Please try again.")
	}
}

func (s *Server) renderError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmplLoginError.Execute(w, struct{ Message string }{Message: msg})
}

package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"swiggy-ssh/internal/application/auth"
	appfood "swiggy-ssh/internal/application/food"
	"swiggy-ssh/internal/application/identity"
	appinstamart "swiggy-ssh/internal/application/instamart"
	domainauth "swiggy-ssh/internal/domain/auth"
	domainfood "swiggy-ssh/internal/domain/food"
	domaininstamart "swiggy-ssh/internal/domain/instamart"
	"swiggy-ssh/internal/presentation/tui"

	"golang.org/x/crypto/ssh"
)

// Server is the SSH transport boundary for session handling.
type Server interface {
	Start(ctx context.Context) error
}

// SSHServer handles SSH listener and session mechanics.
type SSHServer struct {
	addr           string
	hostKeyPath    string
	logger         *slog.Logger
	resolver       *identity.ResolveSSHIdentityUseCase
	registrar      *identity.RegisterSSHIdentityUseCase
	startSession   *identity.StartTerminalSessionUseCase
	endSession     *identity.EndTerminalSessionUseCase
	authAttemptSvc auth.BrowserAuthAttemptService
	publicBaseURL  string
	authUseCase    *auth.EnsureValidAccountUseCase
	instamartSvc   *appinstamart.Service
	foodSvc        *appfood.Service
}

type activeConnTracker struct {
	mu           sync.Mutex
	conns        map[net.Conn]struct{}
	shuttingDown bool
}

type sessionAddressState struct {
	authenticated bool
	addresses     []domaininstamart.Address
	selectedIndex int
	addressStatus tui.HomeAddressStatus
}

func newActiveConnTracker() *activeConnTracker {
	return &activeConnTracker{conns: make(map[net.Conn]struct{})}
}

func (t *activeConnTracker) add(conn net.Conn) {
	t.mu.Lock()
	if t.shuttingDown {
		t.mu.Unlock()
		_ = conn.Close()
		return
	}
	t.conns[conn] = struct{}{}
	t.mu.Unlock()
}

func (t *activeConnTracker) remove(conn net.Conn) {
	t.mu.Lock()
	delete(t.conns, conn)
	t.mu.Unlock()
}

func (t *activeConnTracker) closeAll() {
	t.mu.Lock()
	t.shuttingDown = true
	conns := make([]net.Conn, 0, len(t.conns))
	for conn := range t.conns {
		conns = append(conns, conn)
	}
	t.mu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}

func New(addr, hostKeyPath string, logger *slog.Logger, resolver *identity.ResolveSSHIdentityUseCase, registrar *identity.RegisterSSHIdentityUseCase, startSession *identity.StartTerminalSessionUseCase, endSession *identity.EndTerminalSessionUseCase, authAttemptSvc auth.BrowserAuthAttemptService, publicBaseURL string, authUseCase *auth.EnsureValidAccountUseCase, instamartSvc *appinstamart.Service, foodSvc *appfood.Service) *SSHServer {
	return &SSHServer{
		addr:           addr,
		hostKeyPath:    hostKeyPath,
		logger:         logger,
		resolver:       resolver,
		registrar:      registrar,
		startSession:   startSession,
		endSession:     endSession,
		authAttemptSvc: authAttemptSvc,
		publicBaseURL:  publicBaseURL,
		authUseCase:    authUseCase,
		instamartSvc:   instamartSvc,
		foodSvc:        foodSvc,
	}
}

func (s *SSHServer) Start(ctx context.Context) error {
	hostSigner, err := LoadOrCreateHostKey(s.hostKeyPath)
	if err != nil {
		return fmt.Errorf("load or create host key: %w", err)
	}

	serverConfig := newServerConfig()
	serverConfig.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.addr, err)
	}
	defer listener.Close()

	s.logger.InfoContext(ctx, "ssh server listening",
		"addr", s.addr,
		"host_key_fingerprint", ssh.FingerprintSHA256(hostSigner.PublicKey()),
	)

	var wg sync.WaitGroup
	tracker := newActiveConnTracker()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
		tracker.closeAll()
	}()

	for {
		rawConn, acceptErr := listener.Accept()
		if acceptErr != nil {
			if ctx.Err() != nil {
				wg.Wait()
				return nil
			}

			if errors.Is(acceptErr, net.ErrClosed) {
				wg.Wait()
				return nil
			}

			s.logger.ErrorContext(ctx, "ssh accept failed", "error", acceptErr)
			continue
		}

		tracker.add(rawConn)
		wg.Add(1)
		go func(conn net.Conn) {
			defer wg.Done()
			defer tracker.remove(conn)
			s.handleConn(ctx, conn, serverConfig)
		}(rawConn)
	}
}

func newServerConfig() *ssh.ServerConfig {
	return &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return publicKeyPermissions(key), nil
		},
		// Accept keyboard-interactive without prompts for clients that have no SSH key.
		// Public-key clients still use PublicKeyCallback, preserving fingerprint metadata.
		KeyboardInteractiveCallback: func(conn ssh.ConnMetadata, client ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
}

func publicKeyPermissions(key ssh.PublicKey) *ssh.Permissions {
	return &ssh.Permissions{
		Extensions: map[string]string{
			"pubkey_fingerprint": ssh.FingerprintSHA256(key),
			"pubkey_type":        key.Type(),
			"pubkey_authorized":  strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))),
		},
	}
}

func (s *SSHServer) handleConn(ctx context.Context, netConn net.Conn, serverConfig *ssh.ServerConfig) {
	defer netConn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, serverConfig)
	if err != nil {
		s.logger.WarnContext(ctx, "ssh handshake failed",
			"remote_addr", netConn.RemoteAddr().String(),
			"error", err,
		)
		return
	}
	defer sshConn.Close()

	fingerprint := ""
	pubKeyType := ""
	publicKeyAuthorized := ""
	if sshConn.Permissions != nil {
		fingerprint = sshConn.Permissions.Extensions["pubkey_fingerprint"]
		pubKeyType = sshConn.Permissions.Extensions["pubkey_type"]
		publicKeyAuthorized = sshConn.Permissions.Extensions["pubkey_authorized"]
	}

	var resolvedIdentity *identity.SessionIdentity
	if s.resolver != nil && publicKeyAuthorized != "" {
		publicKey, _, _, _, parseErr := ssh.ParseAuthorizedKey([]byte(publicKeyAuthorized))
		if parseErr != nil {
			s.logger.WarnContext(ctx, "ssh public key parse failed", "error", parseErr)
		} else {
			resolved, resolveErr := s.resolver.Execute(ctx, identity.ResolveSSHIdentityInput{
				Client: identity.ClientProtocolSSH,
				Key:    publicKey,
			})
			if errors.Is(resolveErr, identity.ErrNotFound) {
				s.logger.InfoContext(ctx, "ssh identity unknown; starting guest session",
					"remote_addr", sshConn.RemoteAddr().String(),
					"pubkey_fingerprint", fingerprint,
				)
			} else if errors.Is(resolveErr, identity.ErrSSHIdentityRevoked) {
				s.logger.InfoContext(ctx, "ssh identity revoked; starting guest session",
					"remote_addr", sshConn.RemoteAddr().String(),
					"pubkey_fingerprint", fingerprint,
				)
			} else if resolveErr != nil {
				s.logger.WarnContext(ctx, "ssh identity resolution failed",
					"remote_addr", sshConn.RemoteAddr().String(),
					"pubkey_fingerprint", fingerprint,
					"error", resolveErr,
				)
			} else {
				resolvedIdentity = &resolved
			}
		}
	}

	var terminalSessionID string
	if s.startSession != nil {
		var sshFingerprint *string
		if fingerprint != "" {
			sshFingerprint = &fingerprint
		}
		trackedSession, trackErr := s.startSession.Execute(ctx, identity.StartTerminalSessionInput{
			Client:           identity.ClientProtocolSSH,
			ClientSessionID:  fmt.Sprintf("%x", sshConn.SessionID()),
			SSHFingerprint:   sshFingerprint,
			CurrentScreen:    identity.ScreenSSHSessionPlaceholder,
			ResolvedIdentity: resolvedIdentity,
		})
		if trackErr != nil {
			s.logger.WarnContext(ctx, "terminal session track start failed",
				"remote_addr", sshConn.RemoteAddr().String(),
				"pubkey_fingerprint", fingerprint,
				"error", trackErr,
			)
		} else {
			terminalSessionID = trackedSession.ID
			if s.endSession != nil {
				defer func() {
					endCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if endErr := s.endSession.Execute(endCtx, identity.EndTerminalSessionInput{SessionID: trackedSession.ID}); endErr != nil {
						s.logger.WarnContext(ctx, "terminal session track end failed",
							"terminal_session_id", trackedSession.ID,
							"error", endErr,
						)
					}
				}()
			}
		}
	}

	go ssh.DiscardRequests(reqs)

	var resolvedUserID string
	if resolvedIdentity != nil {
		resolvedUserID = resolvedIdentity.User.ID
	}

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, acceptErr := newChan.Accept()
		if acceptErr != nil {
			s.logger.WarnContext(ctx, "ssh session channel accept failed", "error", acceptErr)
			continue
		}

		go s.handleSessionChannel(ctx, sshConn, channel, requests, terminalSessionID, fingerprint, pubKeyType, publicKeyAuthorized, resolvedUserID)
	}
}

func (s *SSHServer) handleSessionChannel(ctx context.Context, conn *ssh.ServerConn, ch ssh.Channel, reqs <-chan *ssh.Request, terminalSessionID, fingerprint, pubKeyType, publicKeyAuthorized, resolvedUserID string) {
	defer ch.Close()

	started := false
	viewport := tui.Viewport{}
	for req := range reqs {
		switch req.Type {
		case "shell", "exec":
			started = true
			_ = req.Reply(true, nil)
		case "pty-req", "window-change":
			if parsed, ok := parseViewportRequest(req); ok {
				viewport = parsed
			}
			_ = req.Reply(true, nil)
		case "env":
			_ = req.Reply(true, nil)
		default:
			_ = req.Reply(false, nil)
		}

		if started {
			break
		}
	}

	if !started {
		return
	}
	go discardSessionRequests(reqs)

	fallbackMsg := fmt.Sprintf(
		"Welcome to swiggy-ssh (MVP skeleton).\nuser=%s\nremote=%s\nkey_type=%s\nkey_fingerprint=%s\n\n",
		conn.User(),
		conn.RemoteAddr().String(),
		pubKeyType,
		fingerprint,
	)

	s.runSession(tui.WithViewport(ctx, viewport), ch, fallbackMsg, terminalSessionID, fingerprint, publicKeyAuthorized, resolvedUserID)

	_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: 0}))

	s.logger.InfoContext(ctx, "ssh session handled",
		"user", conn.User(),
		"remote_addr", conn.RemoteAddr().String(),
		"pubkey_type", pubKeyType,
		"pubkey_fingerprint", fingerprint,
		"terminal_session_id", terminalSessionID,
		"handled_at", time.Now().UTC().Format(time.RFC3339),
	)
}

type ptyRequestPayload struct {
	Term          string
	Columns       uint32
	Rows          uint32
	WidthPixels   uint32
	HeightPixels  uint32
	TerminalModes string
}

type windowChangePayload struct {
	Columns      uint32
	Rows         uint32
	WidthPixels  uint32
	HeightPixels uint32
}

func parseViewportRequest(req *ssh.Request) (tui.Viewport, bool) {
	switch req.Type {
	case "pty-req":
		var payload ptyRequestPayload
		if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
			return tui.Viewport{}, false
		}
		return viewportFromCells(payload.Columns, payload.Rows)
	case "window-change":
		var payload windowChangePayload
		if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
			return tui.Viewport{}, false
		}
		return viewportFromCells(payload.Columns, payload.Rows)
	default:
		return tui.Viewport{}, false
	}
}

func viewportFromCells(columns, rows uint32) (tui.Viewport, bool) {
	if columns == 0 || rows == 0 {
		return tui.Viewport{}, false
	}
	return tui.Viewport{Width: int(columns), Height: int(rows)}, true
}

func discardSessionRequests(reqs <-chan *ssh.Request) {
	for req := range reqs {
		switch req.Type {
		case "window-change", "env":
			_ = req.Reply(true, nil)
		default:
			_ = req.Reply(false, nil)
		}
	}
}

// runSession drives the screen routing logic for an established SSH session channel.
// It is called after the request loop confirms a shell/exec was started.
func (s *SSHServer) runSession(ctx context.Context, ch ssh.Channel, fallbackMsg, terminalSessionID, fingerprint, publicKeyAuthorized, resolvedUserID string) {
	if s.authAttemptSvc == nil || terminalSessionID == "" {
		_, _ = io.WriteString(ch, fallbackMsg)
		_, _ = io.WriteString(ch, "Session handler placeholder complete. Goodbye.\n")
		return
	}
	render := func(renderCtx context.Context, view tui.View) {
		_ = tui.ClearScreen(ch)
		_ = view.Render(renderCtx, ch)
	}

	state := sessionAddressState{selectedIndex: -1, addressStatus: tui.HomeAddressRequired}
	if s.authUseCase != nil && resolvedUserID != "" {
		_, fastErr := s.authUseCase.Execute(ctx, auth.EnsureValidAccountInput{
			UserID:         resolvedUserID,
			AllowFirstAuth: false,
		})
		if fastErr == nil {
			state.authenticated = true
			s.loadSessionAddresses(ctx, resolvedUserID, &state)
		}
		if errors.Is(fastErr, auth.ErrAccountRevoked) {
			render(ctx, tui.RevokedView{})
			return
		}
	}

	showAddressPicker := false
	showMenu := false
	for {
		_ = tui.ClearScreen(ch)
		result, err := tui.HomeView{Session: state.homeSessionState(), StartAddressPicker: showAddressPicker, StartMenu: showMenu, In: ch}.RenderWithResult(ctx, ch)
		showAddressPicker = false
		showMenu = false
		if err != nil {
			return
		}
		switch result.Action {
		case tui.HomeActionAddressSelected:
			if result.AddressIndex >= 0 && result.AddressIndex < len(state.addresses) {
				state.selectedIndex = result.AddressIndex
				state.addressStatus = tui.HomeAddressSelected
			}
			showMenu = true
			continue
		case tui.HomeActionInstamart:
			if !state.authenticated {
				var ok bool
				ok, resolvedUserID = s.authenticateSessionForApp(ctx, ch, render, terminalSessionID, fingerprint, publicKeyAuthorized, resolvedUserID)
				if !ok {
					return
				}
				state.authenticated = true
				s.loadSessionAddresses(ctx, resolvedUserID, &state)
			}
			selectedAddress, ok := state.selectedAddress()
			if !ok {
				continue
			}
			_ = tui.ClearScreen(ch)
			instamartResult, renderErr := (tui.InstamartAppView{Service: s.instamartSvc, UserID: resolvedUserID, Addresses: state.addresses, SelectedAddress: selectedAddress, In: ch}).RenderWithResult(ctx, ch)
			if renderErr != nil {
				return
			}
			if instamartResult.Action == tui.InstamartActionBackToHome {
				state.selectAddressByID(instamartResult.SelectedAddress.ID)
				showMenu = true
				continue
			}
			return
		case tui.HomeActionTrackOrders:
			if !state.authenticated {
				var ok bool
				ok, resolvedUserID = s.authenticateSessionForApp(ctx, ch, render, terminalSessionID, fingerprint, publicKeyAuthorized, resolvedUserID)
				if !ok {
					return
				}
				state.authenticated = true
				s.loadSessionAddresses(ctx, resolvedUserID, &state)
			}
			selectedAddress, _ := state.selectedAddress()
			_ = tui.ClearScreen(ch)
			instamartResult, renderErr := (tui.InstamartAppView{Service: s.instamartSvc, UserID: resolvedUserID, Addresses: state.addresses, SelectedAddress: selectedAddress, StartTracking: true, In: ch}).RenderWithResult(ctx, ch)
			if renderErr != nil {
				return
			}
			if instamartResult.Action == tui.InstamartActionBackToHome {
				state.selectAddressByID(instamartResult.SelectedAddress.ID)
				showMenu = true
				continue
			}
			return
		case tui.HomeActionFood:
			if !state.authenticated {
				var ok bool
				ok, resolvedUserID = s.authenticateSessionForApp(ctx, ch, render, terminalSessionID, fingerprint, publicKeyAuthorized, resolvedUserID)
				if !ok {
					return
				}
				state.authenticated = true
				s.loadSessionAddresses(ctx, resolvedUserID, &state)
			}
			selectedAddress, ok := state.selectedAddress()
			if !ok {
				continue
			}
			foodAddress := selectedAddressToFoodAddress(selectedAddress)
			if s.foodSvc != nil {
				foodAddresses, err := s.foodSvc.GetAddresses(domainauth.ContextWithUserID(ctx, resolvedUserID))
				if err != nil {
					s.logger.WarnContext(ctx, "food address load failed", "error", err)
					render(ctx, tui.ErrorView{Message: "Food address lookup failed. Please reconnect and try again."})
					return
				}
				var foodAddressOK bool
				foodAddress, foodAddressOK = foodAddressForSelected(foodAddresses, selectedAddress)
				if !foodAddressOK {
					render(ctx, tui.ErrorView{Message: "Food could not match the selected delivery address. Switch addresses in Home or reconnect and try again."})
					showAddressPicker = true
					continue
				}
			}
			_ = tui.ClearScreen(ch)
			foodResult, renderErr := (tui.FoodAppView{
				Service:         s.foodSvc,
				UserID:          resolvedUserID,
				SelectedAddress: foodAddress,
				In:              ch,
			}).RenderWithResult(ctx, ch)
			if renderErr != nil {
				return
			}
			if foodResult.Action == tui.FoodActionBackToHome {
				showMenu = true
				continue
			}
			return
		default:
			return
		}
	}
}

func (s *SSHServer) authenticateSessionForApp(ctx context.Context, ch ssh.Channel, render func(context.Context, tui.View), terminalSessionID, fingerprint, publicKeyAuthorized, resolvedUserID string) (bool, string) {
	// BROWSER AUTH ATTEMPT FLOW

	durableUserID, identityErr := s.ensureDurableUserForBrowserAuth(ctx, resolvedUserID, publicKeyAuthorized)
	if identityErr != nil {
		s.logger.WarnContext(ctx, "failed to establish durable ssh identity", "error", identityErr, "pubkey_fingerprint", fingerprint)
		if errors.Is(identityErr, auth.ErrOAuthAccountUserRequired) || errors.Is(identityErr, identity.ErrMissingSSHPublicKey) {
			render(ctx, tui.ErrorView{Message: "Browser login needs an SSH public key. Reconnect with an SSH key and try again."})
			return false, resolvedUserID
		}
		render(ctx, tui.ErrorView{Message: "Login unavailable. Please try again later."})
		return false, resolvedUserID
	}
	resolvedUserID = durableUserID

	authRequired, issueErr := s.beginBrowserAuth(ctx, resolvedUserID, terminalSessionID)
	if issueErr != nil {
		s.logger.WarnContext(ctx, "failed to issue auth attempt", "error", issueErr)
		if errors.Is(issueErr, auth.ErrOAuthAccountUserRequired) {
			render(ctx, tui.ErrorView{Message: "Browser login needs an SSH public key. Reconnect with an SSH key and try again."})
			return false, resolvedUserID
		}
		render(ctx, tui.ErrorView{Message: "Login unavailable. Please try again later."})
		return false, resolvedUserID
	}
	if !authRequired.AuthRequired {
		render(ctx, tui.ErrorView{Message: "Login unavailable. Please try again later."})
		return false, resolvedUserID
	}
	completed, pollErr := s.renderLoginWaitingAndPoll(ctx, ch, authRequired.LoginURL, authRequired.AuthAttemptToken)
	if pollErr != nil {
		render(ctx, tui.ErrorView{Message: "Login polling error. Session ending."})
		return false, resolvedUserID
	}
	if !completed {
		render(ctx, tui.ErrorView{Message: "Login expired or cancelled. Please reconnect."})
		return false, resolvedUserID
	}

	// Login confirmed — check/establish account.
	if resolvedUserID == "" {
		render(ctx, tui.InstamartPlaceholderView{StatusMessage: "Guest session connected for this SSH session.", In: ch})
		return false, resolvedUserID
	}
	if s.authUseCase == nil {
		render(ctx, tui.LoginSuccessView{In: ch})
		return true, resolvedUserID
	}

	reauthFn := func(reauthCtx context.Context) error {
		newAttempt, _, reauthIssueErr := s.authAttemptSvc.IssueAuthAttempt(reauthCtx, resolvedUserID, terminalSessionID)
		if reauthIssueErr != nil {
			return fmt.Errorf("issue reauth auth attempt: %w", reauthIssueErr)
		}
		render(reauthCtx, tui.ReconnectView{LoginURL: authStartURL(s.publicBaseURL, newAttempt)})
		reauthCompleted, reauthPollErr := pollAuthAttempt(reauthCtx, s.authAttemptSvc, newAttempt, s.logger)
		if reauthPollErr != nil {
			return fmt.Errorf("poll reauth: %w", reauthPollErr)
		}
		if !reauthCompleted {
			return errors.New("reauth attempt expired or cancelled")
		}
		return nil
	}

	result, authErr := s.authUseCase.Execute(ctx, auth.EnsureValidAccountInput{
		UserID:         resolvedUserID,
		AllowFirstAuth: true,
		Reauth:         reauthFn,
	})
	switch {
	case errors.Is(authErr, auth.ErrAccountRevoked):
		render(ctx, tui.RevokedView{})
		return false, resolvedUserID
	case authErr != nil:
		s.logger.WarnContext(ctx, "ensure valid account failed", "error", authErr)
		render(ctx, tui.ErrorView{Message: "Auth check failed. Please reconnect."})
		return false, resolvedUserID
	default:
		render(ctx, tui.LoginSuccessView{
			IsFirstAuth: result.IsFirstAuth,
			WasReauth:   result.WasReauth,
			Account:     result.Account,
			In:          ch,
		})
		return true, resolvedUserID
	}
	return false, resolvedUserID
}

func (s *SSHServer) loadSessionAddresses(ctx context.Context, userID string, state *sessionAddressState) {
	if state == nil || !state.authenticated {
		return
	}
	if s.instamartSvc == nil || userID == "" {
		state.addressStatus = tui.HomeAddressUnavailable
		return
	}
	addresses, err := s.instamartSvc.GetAddresses(domainauth.ContextWithUserID(ctx, userID))
	if err != nil {
		s.logger.WarnContext(ctx, "session address load failed", "error", err)
		state.addresses = nil
		state.selectedIndex = -1
		state.addressStatus = tui.HomeAddressUnavailable
		return
	}
	state.addresses = addresses
	if len(addresses) == 0 {
		state.selectedIndex = -1
		state.addressStatus = tui.HomeAddressRequired
		return
	}
	if state.selectedIndex < 0 || state.selectedIndex >= len(addresses) {
		state.selectedIndex = 0
	}
	state.addressStatus = tui.HomeAddressSelected
}

func (s sessionAddressState) homeSessionState() tui.HomeSessionState {
	addresses := make([]tui.HomeAddressOption, 0, len(s.addresses))
	for _, address := range s.addresses {
		addresses = append(addresses, tui.HomeAddressOption{
			ID:          address.ID,
			Label:       address.Label,
			DisplayLine: address.DisplayLine,
			PhoneMasked: address.PhoneMasked,
			Category:    address.Category,
		})
	}
	return tui.HomeSessionState{
		Authenticated:        s.authenticated,
		AddressStatus:        s.addressStatus,
		SelectedAddressIndex: s.selectedIndex,
		Addresses:            addresses,
	}
}

func (s sessionAddressState) selectedAddress() (domaininstamart.Address, bool) {
	if s.addressStatus != tui.HomeAddressSelected || s.selectedIndex < 0 || s.selectedIndex >= len(s.addresses) {
		return domaininstamart.Address{}, false
	}
	if strings.TrimSpace(s.addresses[s.selectedIndex].ID) == "" {
		return domaininstamart.Address{}, false
	}
	return s.addresses[s.selectedIndex], true
}

func selectedAddressToFoodAddress(addr domaininstamart.Address) domainfood.Address {
	return domainfood.Address{
		ID:          addr.ID,
		Label:       addr.Label,
		DisplayLine: addr.DisplayLine,
		PhoneMasked: addr.PhoneMasked,
		Category:    addr.Category,
	}
}

func foodAddressForSelected(addresses []domainfood.Address, selected domaininstamart.Address) (domainfood.Address, bool) {
	if len(addresses) == 0 {
		return domainfood.Address{}, false
	}
	selectedID := strings.TrimSpace(selected.ID)
	if selectedID != "" {
		for _, address := range addresses {
			if strings.TrimSpace(address.ID) == selectedID {
				return address, true
			}
		}
	}

	selectedKey := comparableAddressKey(selected.Label, selected.DisplayLine, selected.Category, selected.PhoneMasked)
	if selectedKey != "" {
		for _, address := range addresses {
			if comparableAddressKey(address.Label, address.DisplayLine, address.Category, address.PhoneMasked) == selectedKey {
				return address, true
			}
		}
	}

	return domainfood.Address{}, false
}

func comparableAddressKey(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			normalized = append(normalized, part)
		}
	}
	return strings.Join(normalized, "|")
}

func (s *sessionAddressState) selectAddressByID(addressID string) {
	addressID = strings.TrimSpace(addressID)
	if addressID == "" {
		return
	}
	for i, address := range s.addresses {
		if strings.TrimSpace(address.ID) == addressID {
			s.selectedIndex = i
			s.addressStatus = tui.HomeAddressSelected
			return
		}
	}
}

func (s *SSHServer) renderLoginWaitingAndPoll(ctx context.Context, ch ssh.Channel, loginURL, rawAttempt string) (bool, error) {
	renderCtx, cancelRender := context.WithCancel(ctx)
	renderDone := make(chan error, 1)
	go func() {
		_ = tui.ClearScreen(ch)
		renderDone <- (tui.LoginWaitingView{LoginURL: loginURL, In: ch}).Render(renderCtx, ch)
	}()

	completed, pollErr := pollAuthAttempt(ctx, s.authAttemptSvc, rawAttempt, s.logger)
	cancelRender()
	if renderErr := <-renderDone; renderErr != nil {
		s.logger.WarnContext(ctx, "login waiting render failed", "error", renderErr)
	}

	return completed, pollErr
}

func authStartURL(publicBaseURL, rawAttempt string) string {
	return publicBaseURL + "/auth/start?attempt=" + url.QueryEscape(rawAttempt)
}

func (s *SSHServer) ensureDurableUserForBrowserAuth(ctx context.Context, resolvedUserID, publicKeyAuthorized string) (string, error) {
	if resolvedUserID != "" {
		return resolvedUserID, nil
	}
	if publicKeyAuthorized == "" || s.registrar == nil {
		return "", auth.ErrOAuthAccountUserRequired
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKeyAuthorized))
	if err != nil {
		return "", err
	}
	registered, err := s.registrar.Execute(ctx, identity.RegisterSSHIdentityInput{
		Client: identity.ClientProtocolSSH,
		Key:    publicKey,
	})
	if err != nil {
		return "", err
	}
	return registered.User.ID, nil
}

func (s *SSHServer) beginBrowserAuth(ctx context.Context, userID, terminalSessionID string) (auth.EnsureValidAccountOutput, error) {
	if userID == "" {
		return auth.EnsureValidAccountOutput{}, auth.ErrOAuthAccountUserRequired
	}
	if s.authUseCase != nil {
		return s.authUseCase.Execute(ctx, auth.EnsureValidAccountInput{
			UserID:             userID,
			AllowFirstAuth:     true,
			AuthAttemptService: s.authAttemptSvc,
			TerminalSessionID:  terminalSessionID,
			PublicBaseURL:      s.publicBaseURL,
		})
	}
	rawAttempt, _, err := s.authAttemptSvc.IssueAuthAttempt(ctx, userID, terminalSessionID)
	if err != nil {
		return auth.EnsureValidAccountOutput{}, err
	}
	return auth.EnsureValidAccountOutput{
		AuthRequired:     true,
		LoginURL:         authStartURL(s.publicBaseURL, rawAttempt),
		AuthAttemptToken: rawAttempt,
	}, nil
}

// pollAuthAttempt polls svc every 2 s until the attempt leaves the pending state.
// Returns (true, nil) on completion, (false, nil) on expiry/cancellation,
// (false, err) on unexpected error. Respects ctx cancellation.
func pollAuthAttempt(ctx context.Context, svc auth.BrowserAuthAttemptService, rawAttempt string, logger *slog.Logger) (completed bool, err error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// Treat SSH disconnect / server shutdown the same as expiry at the
			// caller level — avoids "polling error" message on clean disconnect.
			return false, nil
		case <-ticker.C:
			record, getErr := svc.GetAuthAttempt(ctx, rawAttempt)
			if errors.Is(getErr, auth.ErrAuthAttemptNotFound) {
				return false, nil // expired
			}
			if getErr != nil {
				logger.WarnContext(ctx, "poll auth attempt error", "error", getErr)
				return false, getErr
			}
			switch record.Status {
			case auth.AuthAttemptStatusCompleted:
				return true, nil
			case auth.AuthAttemptStatusCancelled:
				return false, nil
				// AuthAttemptStatusPending: keep polling
			}
		}
	}
}

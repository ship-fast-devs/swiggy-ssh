package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"os/signal"
	"strings"
	"syscall"

	"swiggy-ssh/internal/application/auth"
	appfood "swiggy-ssh/internal/application/food"
	"swiggy-ssh/internal/application/identity"
	appinstamart "swiggy-ssh/internal/application/instamart"
	domainfood "swiggy-ssh/internal/domain/food"
	domaininstamart "swiggy-ssh/internal/domain/instamart"
	cache "swiggy-ssh/internal/infrastructure/cache/redis"
	"swiggy-ssh/internal/infrastructure/crypto"
	store "swiggy-ssh/internal/infrastructure/persistence/postgres"
	mockprovider "swiggy-ssh/internal/infrastructure/provider/mock"
	swiggyprovider "swiggy-ssh/internal/infrastructure/provider/swiggy"
	"swiggy-ssh/internal/platform/config"
	"swiggy-ssh/internal/platform/logging"
	httpserver "swiggy-ssh/internal/presentation/http"
	sshserver "swiggy-ssh/internal/presentation/ssh"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	logger := logging.New(cfg.AppEnv)
	if err := cfg.Validate(); err != nil {
		logger.ErrorContext(ctx, "invalid configuration", "error", err)
		return
	}

	if cfg.AppEnv == "production" && cfg.TokenEncryptionKey == config.DefaultDevTokenEncryptionKey {
		logger.ErrorContext(ctx, "TOKEN_ENCRYPTION_KEY must be set in production; refusing to start with the dev default key")
		return
	}

	keyBytes, err := base64.RawURLEncoding.DecodeString(cfg.TokenEncryptionKey)
	if err != nil {
		logger.ErrorContext(ctx, "invalid TOKEN_ENCRYPTION_KEY: base64 decode failed", "error", err)
		return
	}
	encryptor, err := crypto.NewAESGCMEncryptor(keyBytes)
	if err != nil {
		logger.ErrorContext(ctx, "invalid TOKEN_ENCRYPTION_KEY", "error", err)
		return
	}

	postgresStore, err := store.NewPostgresStore(ctx, cfg.DatabaseURL, encryptor)
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize postgres store", "error", err)
		return
	}
	defer postgresStore.Close()

	redisClient, err := cache.NewRedisClient(ctx, cfg.RedisURL)
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize redis client", "error", err)
		return
	}
	defer redisClient.Close()

	authAttemptSvc := cache.NewRedisLoginCodeService(redisClient, cfg.LoginCodeTTL)
	logger.InfoContext(ctx, "browser auth attempt service ready", "ttl", cfg.LoginCodeTTL)

	swiggyBrowserAuth := swiggyprovider.NewBrowserAuthClient(swiggyprovider.BrowserAuthConfig{
		AuthorizeURL: cfg.SwiggyAuthAuthorizeURL,
		TokenURL:     cfg.SwiggyAuthTokenURL,
		ClientID:     cfg.SwiggyClientID,
		Scopes:       strings.Fields(strings.ReplaceAll(cfg.SwiggyAuthScopes, ",", " ")),
	})
	ensureValidAccount := auth.NewEnsureValidAccountUseCase(postgresStore)
	completeBrowserAuth := auth.NewCompleteBrowserAuthUseCase(postgresStore, authAttemptSvc, swiggyBrowserAuth)
	startBrowserAuth := auth.NewStartBrowserAuthUseCase(authAttemptSvc, swiggyBrowserAuth)

	var instamartProvider domaininstamart.Provider
	var foodProvider domainfood.Provider
	switch cfg.SwiggyProvider {
	case "mock":
		instamartProvider = mockprovider.NewInstamartProvider()
		foodProvider = nil // no mock food provider; food TUI shows unavailable message
	case "mcp", "swiggy":
		authorizer := swiggyprovider.NewOAuthAccountAuthorizer(postgresStore)
		instamartProvider = swiggyprovider.NewMCPInstamartClient(cfg.SwiggyMCPIMEndpoint, http.DefaultClient, authorizer)
		foodProvider = swiggyprovider.NewMCPFoodClient(cfg.SwiggyMCPFoodEndpoint, http.DefaultClient, authorizer)
	default:
		logger.ErrorContext(ctx, "invalid SWIGGY_PROVIDER; accepted values are mock, mcp, swiggy", "provider", cfg.SwiggyProvider)
		return
	}
	instamartService := appinstamart.NewService(instamartProvider)

	var foodService *appfood.Service
	if foodProvider != nil {
		foodService = appfood.NewService(foodProvider)
	}

	resolveSSHIdentity := identity.NewResolveSSHIdentityUseCase(postgresStore)
	registerSSHIdentity := identity.NewRegisterSSHIdentityUseCase(postgresStore)
	startTerminalSession := identity.NewStartTerminalSessionUseCase(postgresStore)
	endTerminalSession := identity.NewEndTerminalSessionUseCase(postgresStore)
	server := sshserver.New(cfg.SSHAddr, cfg.SSHHostKeyPath, logger, resolveSSHIdentity, registerSSHIdentity, startTerminalSession, endTerminalSession, authAttemptSvc, cfg.PublicBaseURL, ensureValidAccount, instamartService, foodService)
	httpSrv := httpserver.New(cfg.HTTPAddr, logger, authAttemptSvc, completeBrowserAuth, startBrowserAuth, cfg.PublicBaseURL, cfg.SwiggyProvider)

	logger.InfoContext(ctx, "swiggy-ssh scaffold startup",
		"app_env", cfg.AppEnv,
		"provider", cfg.SwiggyProvider,
		"swiggy_mcp_im_endpoint", cfg.SwiggyMCPIMEndpoint,
		"swiggy_mcp_food_endpoint", cfg.SwiggyMCPFoodEndpoint,
		"ssh_addr", cfg.SSHAddr,
		"ssh_host_key_path", cfg.SSHHostKeyPath,
		"http_addr", cfg.HTTPAddr,
		"public_base_url", cfg.PublicBaseURL,
	)

	// Run SSH and HTTP servers concurrently; stop both on signal or first error.
	errCh := make(chan error, 2)
	go func() { errCh <- server.Start(ctx) }()
	go func() { errCh <- httpSrv.Start(ctx) }()

	if err := <-errCh; err != nil {
		logger.ErrorContext(ctx, "server exited with error", "error", err)
	}
	// ctx cancellation (from signal) stops both servers.
}

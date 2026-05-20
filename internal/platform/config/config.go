package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultAppEnv                 = "local"
	defaultSwiggyProvider         = "mock"
	defaultSwiggyMCPIMEndpoint    = "https://mcp.swiggy.com/im"
	defaultSwiggyMCPFoodEndpoint  = "https://mcp.swiggy.com/food"
	defaultSwiggyAuthAuthorizeURL = "https://mcp.swiggy.com/auth/authorize"
	defaultSwiggyAuthTokenURL     = "https://mcp.swiggy.com/auth/token"
	defaultSwiggyClientID         = "swiggy-mcp"
	defaultSwiggyAuthScopes       = "mcp:tools"
	defaultSSHAddr                = ":2222"
	defaultSSHHostKeyPath         = ".local/ssh_host_ed25519_key"
	defaultHTTPAddr               = ":8080"
	defaultDatabaseURL            = "postgres://swiggy:swiggy@localhost:5432/swiggy_ssh?sslmode=disable"
	defaultRedisURL               = "redis://localhost:6379/0"
	defaultPublicBaseURL          = "http://localhost:8080"

	// DefaultDevTokenEncryptionKey is the LOCAL DEVELOPMENT ONLY fallback encryption key.
	// This key is publicly known from the source code.
	// TOKEN_ENCRYPTION_KEY must be set to a securely generated value in production.
	DefaultDevTokenEncryptionKey = "c3dpZ2d5LXNzaC1sb2NhbC1kZXYta2V5LTMyYnl0ZXM"
)

// Config holds process configuration loaded from env with local defaults.
type Config struct {
	AppEnv                 string
	SwiggyProvider         string
	SwiggyMCPIMEndpoint    string
	SwiggyMCPFoodEndpoint  string
	SwiggyLoginStartURL    string
	SwiggyAuthAuthorizeURL string
	SwiggyAuthTokenURL     string
	SwiggyClientID         string
	SwiggyAuthScopes       string
	SSHAddr                string
	SSHHostKeyPath         string
	HTTPAddr               string
	DatabaseURL            string
	RedisURL               string
	PublicBaseURL          string
	LoginCodeTTL           time.Duration
	TokenEncryptionKey     string // base64url-encoded 32-byte AES-256 key; required in production
}

// Load reads runtime configuration from environment variables.
func Load() Config {
	return Config{
		AppEnv:                 envOrDefault("APP_ENV", defaultAppEnv),
		SwiggyProvider:         envOrDefault("SWIGGY_PROVIDER", defaultSwiggyProvider),
		SwiggyMCPIMEndpoint:    envOrDefault("SWIGGY_MCP_IM_ENDPOINT", defaultSwiggyMCPIMEndpoint),
		SwiggyMCPFoodEndpoint:  envOrDefault("SWIGGY_MCP_FOOD_ENDPOINT", defaultSwiggyMCPFoodEndpoint),
		SwiggyLoginStartURL:    envOrDefault("SWIGGY_LOGIN_START_URL", ""),
		SwiggyAuthAuthorizeURL: envOrDefault("SWIGGY_AUTH_AUTHORIZE_URL", defaultSwiggyAuthAuthorizeURL),
		SwiggyAuthTokenURL:     envOrDefault("SWIGGY_AUTH_TOKEN_URL", defaultSwiggyAuthTokenURL),
		SwiggyClientID:         envOrDefault("SWIGGY_CLIENT_ID", defaultSwiggyClientID),
		SwiggyAuthScopes:       envOrDefault("SWIGGY_AUTH_SCOPES", defaultSwiggyAuthScopes),
		SSHAddr:                envOrDefault("SSH_ADDR", defaultSSHAddr),
		SSHHostKeyPath:         envOrDefault("SSH_HOST_KEY_PATH", defaultSSHHostKeyPath),
		HTTPAddr:               envOrDefault("HTTP_ADDR", defaultHTTPAddr),
		DatabaseURL:            envOrDefault("DATABASE_URL", defaultDatabaseURL),
		RedisURL:               envOrDefault("REDIS_URL", defaultRedisURL),
		PublicBaseURL:          envOrDefault("PUBLIC_BASE_URL", defaultPublicBaseURL),
		LoginCodeTTL:           envDuration("LOGIN_CODE_TTL", 10*time.Minute),
		TokenEncryptionKey:     envOrDefault("TOKEN_ENCRYPTION_KEY", DefaultDevTokenEncryptionKey),
	}
}

func (c Config) Validate() error {
	if c.SwiggyProvider == "mock" {
		return nil
	}
	if c.SwiggyProvider != "mcp" && c.SwiggyProvider != "swiggy" {
		return fmt.Errorf("SWIGGY_PROVIDER must be mock, mcp, or swiggy, got %q", c.SwiggyProvider)
	}
	mcpEndpoint := strings.TrimSpace(c.SwiggyMCPIMEndpoint)
	if mcpEndpoint == "" {
		mcpEndpoint = defaultSwiggyMCPIMEndpoint
	}
	if err := validateHTTPURL("SWIGGY_MCP_IM_ENDPOINT", mcpEndpoint); err != nil {
		return err
	}
	if strings.TrimSpace(c.SwiggyClientID) == "" {
		return errors.New("SWIGGY_CLIENT_ID is required when SWIGGY_PROVIDER is not mock")
	}
	foodEndpoint := strings.TrimSpace(c.SwiggyMCPFoodEndpoint)
	if foodEndpoint == "" {
		foodEndpoint = defaultSwiggyMCPFoodEndpoint
	}
	if err := validateHTTPURL("SWIGGY_MCP_FOOD_ENDPOINT", foodEndpoint); err != nil {
		return err
	}
	if err := validateHTTPURL("SWIGGY_AUTH_AUTHORIZE_URL", c.SwiggyAuthAuthorizeURL); err != nil {
		return err
	}
	return validateHTTPURL("SWIGGY_AUTH_TOKEN_URL", c.SwiggyAuthTokenURL)
}

func validateHTTPURL(name, value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid absolute http(s) URL: %w", name, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid absolute http(s) URL", name)
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}

	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

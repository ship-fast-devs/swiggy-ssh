CREATE TABLE IF NOT EXISTS ssh_identities (
    id UUID PRIMARY KEY,
    public_key_fingerprint TEXT NOT NULL UNIQUE,
    public_key TEXT NOT NULL,
    label TEXT,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS oauth_accounts (
    id UUID PRIMARY KEY,
    ssh_identity_id UUID NOT NULL REFERENCES ssh_identities(id),
    provider TEXT NOT NULL,
    provider_user_id TEXT,
    encrypted_access_token TEXT NOT NULL,
    token_expires_at TIMESTAMPTZ,
    scopes TEXT[],
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(ssh_identity_id, provider),
    CONSTRAINT oauth_accounts_status_check CHECK (status IN ('active', 'expired', 'reconnect_required', 'revoked'))
);

CREATE INDEX IF NOT EXISTS idx_oauth_accounts_ssh_identity_id ON oauth_accounts(ssh_identity_id);

CREATE TABLE IF NOT EXISTS terminal_sessions (
    id UUID PRIMARY KEY,
    ssh_identity_id UUID REFERENCES ssh_identities(id),
    ssh_fingerprint TEXT,
    client TEXT NOT NULL,
    client_session_id TEXT NOT NULL,
    current_screen TEXT,
    selected_address_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_terminal_sessions_ssh_identity_id ON terminal_sessions(ssh_identity_id);

CREATE TABLE IF NOT EXISTS audit_events (
    id UUID PRIMARY KEY,
    ssh_identity_id UUID NULL REFERENCES ssh_identities(id),
    terminal_session_id UUID NULL REFERENCES terminal_sessions(id),
    event_name TEXT NOT NULL,
    provider TEXT,
    status TEXT NOT NULL,
    error_code TEXT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_events_ssh_identity_id ON audit_events(ssh_identity_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_terminal_session_id ON audit_events(terminal_session_id);

CREATE TABLE IF NOT EXISTS oidc_login_flows (
    state_hash TEXT PRIMARY KEY,
    nonce TEXT NOT NULL,
    code_verifier TEXT NOT NULL,
    return_path TEXT NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_sessions (
    token_hash TEXT PRIMARY KEY,
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    email TEXT NOT NULL,
    csrf_token TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS auth_sessions_expires_at ON auth_sessions(expires_at);

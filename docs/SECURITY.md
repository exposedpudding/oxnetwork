# Security Model - oxnet.work

## 1. Threat model summary

oxnet.work can perform privileged infrastructure operations. Treat compromise of the control plane as materially more serious than compromise of a read-only dashboard.

Primary threats:

- stolen administrator session;
- leaked integration token;
- malicious or compromised node agent;
- control plane exposed publicly without adequate auth;
- command/action injection;
- SSRF through integration configuration;
- sensitive data in logs;
- cross-site request attacks;
- replayed agent/action requests;
- overly broad Docker/socket privileges.

## 2. Exposure model

Preferred:

```text
Browser -> Tailscale -> oxnet.work
```

Optional:

```text
Internet -> Pangolin -> authentication/access policy -> oxnet.work
```

The app must not assume that possession of a URL equals authorization.

## 3. Authentication

Target authentication is OIDC, with Authentik a preferred self-hosted provider. Keep auth implementation provider-neutral.

The server now supports OIDC authorization code login with PKCE, nonce and signed ID token validation. It stores hashed, revocable session tokens in SQLite, uses Secure/HttpOnly/SameSite cookies on HTTPS, and requires a CSRF token for sign out. The provider is enabled only when issuer, client ID, client secret file and public URL are all configured. The Home GW stack has no public ingress by default; live authorization and role checks are still required before action endpoints are introduced.

Bootstrap options before OIDC is configured must be conservative:

- bind only to loopback/private tailnet; and/or
- one-time admin bootstrap credential stored hashed; and
- force change/disable bootstrap after OIDC setup.

Do not ship a known default username/password.

## 4. Authorization

Even for a single-owner product, model permissions so actions can be separated later.

Suggested scopes:

- `read:inventory`
- `read:metrics`
- `read:logs`
- `run:safe_actions`
- `run:destructive_actions`
- `manage:workflows`
- `manage:integrations`
- `manage:security`

Roles can map to sets of scopes.

## 5. Sessions

- Secure, HttpOnly, SameSite cookies for browser sessions.
- Rotate session identifiers after authentication.
- Configurable idle/absolute expiry.
- CSRF protection for cookie-authenticated state changes.
- No auth token in URL/query strings.

## 6. Secrets

Never store plaintext integration secrets in ordinary SQLite fields.

v1 acceptable approaches, in preference order:

1. environment variable / Docker secret / mounted secret file references;
2. encrypted local secret store with key provided separately;
3. future Vaultwarden/other secret provider integration.

The DB may store a secret reference such as `env:TAILSCALE_API_KEY` but not the value.

## 7. Agent security

- unique agent identity;
- short-lived or rotatable scoped credentials;
- no shared universal agent password;
- action IDs are unique and not reusable;
- agent rejects actions outside declared capabilities;
- agent validates server identity;
- action parameters use typed schemas;
- no arbitrary shell command field;
- logs redact secrets/environment values.

Docker integration should prefer the local agent talking to Docker locally rather than mounting remote Docker sockets into the control plane.

## 8. Dangerous action UX

Actions such as shutdown, restart, drain with active jobs, container deletion, restore, and configuration mutation require:

- explicit descriptive confirmation;
- target name displayed;
- impact summary;
- optional typed confirmation for high-impact actions;
- audit record.

## 9. Network/provider clients

- strict URL allowlisting/config validation;
- explicit timeouts;
- TLS verification enabled by default;
- bounded response sizes;
- no following arbitrary redirects to private metadata endpoints from user-entered URLs;
- proxy behavior documented.

## 10. Logging

Log:

- request ID;
- authenticated actor ID;
- action type/target;
- provider health errors in sanitized form.

Do not log:

- passwords;
- API tokens;
- session cookies;
- authorization headers;
- full environment dumps;
- sensitive container env values.

## 11. Audit trail

Audit events are append-only at application level. Include:

- actor;
- action;
- target;
- result;
- request/action ID;
- source IP/tailnet identity where available;
- timestamp;
- sanitized context.

Provide export capability later; do not allow normal users to edit past audit entries.

## 12. Backup security

- SQLite backup may contain infrastructure metadata; protect it.
- Secret values should not be inside the DB where possible.
- Document restore validation.
- Backup success in UI does not mean recovery is proven; add periodic restore test workflow later.

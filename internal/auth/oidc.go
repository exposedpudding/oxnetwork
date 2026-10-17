package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/exposedpudding/oxnetwork/internal/config"
	"golang.org/x/oauth2"
)

const (
	sessionCookie = "oxnet_session"
	stateCookie   = "oxnet_oidc_state"
	loginTTL      = 10 * time.Minute
	idleTTL       = 30 * time.Minute
	absoluteTTL   = 12 * time.Hour
)

type Identity struct {
	Subject string `json:"subject"`
	Email   string `json:"email,omitempty"`
}

type session struct {
	Identity
	CSRFToken string
	LastSeen  time.Time
	Expires   time.Time
}

type contextKey struct{}

func FromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	return identity, ok
}

type Service struct {
	db       *sql.DB
	issuer   string
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	secure   bool
	now      func() time.Time
}

// New fetches the provider's OIDC discovery document. Failure prevents the
// authenticated server from starting, rather than silently falling back to
// unauthenticated mode.
func New(ctx context.Context, db *sql.DB, cfg config.Config) (*Service, error) {
	if !cfg.OIDCEnabled() {
		return nil, errors.New("OIDC is not configured")
	}
	secret, err := os.ReadFile(cfg.Auth.ClientSecretFile)
	if err != nil {
		return nil, fmt.Errorf("read OIDC client secret file: %w", err)
	}
	clientSecret := strings.TrimSpace(string(secret))
	if clientSecret == "" {
		return nil, errors.New("OIDC client secret file is empty")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	provider, err := oidc.NewProvider(ctx, cfg.Auth.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed: %w", err)
	}
	publicURL := strings.TrimRight(cfg.Auth.PublicURL, "/")
	return &Service{
		db:       db,
		issuer:   cfg.Auth.IssuerURL,
		oauth:    oauth2.Config{ClientID: cfg.Auth.ClientID, ClientSecret: clientSecret, RedirectURL: publicURL + "/auth/callback", Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"}},
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.Auth.ClientID}),
		secure:   strings.HasPrefix(publicURL, "https://"),
		now:      time.Now,
	}, nil
}

func (s *Service) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/readyz":
			next.ServeHTTP(w, r)
			return
		case "/auth/login":
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			s.login(w, r)
			return
		case "/auth/callback":
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			s.callback(w, r)
			return
		}
		current, token, err := s.currentSession(r)
		if err != nil {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/auth/") || r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "UNAUTHENTICATED", "message": "Sign in is required"}})
				return
			}
			http.Redirect(w, r, "/auth/login?return_to="+url.QueryEscape(safeReturnPath(r.URL.RequestURI())), http.StatusSeeOther)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		if r.URL.Path == "/auth/session" {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]any{"identity": current.Identity, "csrf_token": current.CSRFToken})
			return
		}
		if r.URL.Path == "/auth/logout" {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(current.CSRFToken)) != 1 {
				http.Error(w, "invalid CSRF token", http.StatusForbidden)
				return
			}
			_, _ = s.db.ExecContext(r.Context(), `DELETE FROM auth_sessions WHERE token_hash=?`, hash(token))
			s.clearCookie(w, sessionCookie, "/")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, current.Identity)))
	})
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	state, err := randomString()
	if err != nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	nonce, err := randomString()
	if err != nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	verifier := oauth2.GenerateVerifier()
	returnPath := safeReturnPath(r.URL.Query().Get("return_to"))
	_, _ = s.db.ExecContext(r.Context(), `DELETE FROM oidc_login_flows WHERE expires_at < ?`, s.now().Unix())
	_, _ = s.db.ExecContext(r.Context(), `DELETE FROM auth_sessions WHERE expires_at < ?`, s.now().Unix())
	_, err = s.db.ExecContext(r.Context(), `INSERT INTO oidc_login_flows(state_hash,nonce,code_verifier,return_path,expires_at) VALUES(?,?,?,?,?)`, hash(state), nonce, verifier, returnPath, s.now().Add(loginTTL).Unix())
	if err != nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	s.setCookie(w, stateCookie, state, "/auth/callback", s.now().Add(loginTTL), true)
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, s.oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), http.StatusSeeOther)
}

func (s *Service) callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie(stateCookie)
	s.clearCookie(w, stateCookie, "/auth/callback")
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookie.Value)) != 1 || r.URL.Query().Get("code") == "" {
		http.Error(w, "invalid login response", http.StatusBadRequest)
		return
	}
	var nonce, codeVerifier, returnPath string
	var expires int64
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(r.Context(), `SELECT nonce,code_verifier,return_path,expires_at FROM oidc_login_flows WHERE state_hash=?`, hash(state)).Scan(&nonce, &codeVerifier, &returnPath, &expires)
	if err != nil || s.now().Unix() > expires {
		http.Error(w, "login expired", http.StatusBadRequest)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM oidc_login_flows WHERE state_hash=?`, hash(state)); err != nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	token, err := s.oauth.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(codeVerifier))
	if err != nil {
		http.Error(w, "login exchange failed", http.StatusBadGateway)
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "identity token missing", http.StatusBadGateway)
		return
	}
	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil || idToken.Nonce != nonce {
		http.Error(w, "identity verification failed", http.StatusBadGateway)
		return
	}
	var claims struct {
		Subject string `json:"sub"`
		Email   string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" {
		http.Error(w, "identity claims invalid", http.StatusBadGateway)
		return
	}
	sessionToken, err := randomString()
	if err != nil {
		http.Error(w, "session unavailable", http.StatusServiceUnavailable)
		return
	}
	csrfToken, err := randomString()
	if err != nil {
		http.Error(w, "session unavailable", http.StatusServiceUnavailable)
		return
	}
	now := s.now()
	_, err = s.db.ExecContext(r.Context(), `INSERT INTO auth_sessions(token_hash,issuer,subject,email,csrf_token,created_at,last_seen_at,expires_at) VALUES(?,?,?,?,?,?,?,?)`, hash(sessionToken), s.issuer, claims.Subject, claims.Email, csrfToken, now.Unix(), now.Unix(), now.Add(absoluteTTL).Unix())
	if err != nil {
		http.Error(w, "session unavailable", http.StatusServiceUnavailable)
		return
	}
	s.setCookie(w, sessionCookie, sessionToken, "/", now.Add(absoluteTTL), true)
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, safeReturnPath(returnPath), http.StatusSeeOther)
}

func (s *Service) currentSession(r *http.Request) (session, string, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return session{}, "", errors.New("session missing")
	}
	var result session
	var lastSeen, expires int64
	err = s.db.QueryRowContext(r.Context(), `SELECT subject,email,csrf_token,last_seen_at,expires_at FROM auth_sessions WHERE token_hash=? AND issuer=?`, hash(cookie.Value), s.issuer).Scan(&result.Subject, &result.Email, &result.CSRFToken, &lastSeen, &expires)
	if err != nil {
		return session{}, "", err
	}
	now := s.now()
	result.LastSeen, result.Expires = time.Unix(lastSeen, 0), time.Unix(expires, 0)
	if !now.Before(result.Expires) || now.Sub(result.LastSeen) >= idleTTL {
		return session{}, "", errors.New("session expired")
	}
	if now.Sub(result.LastSeen) >= time.Minute {
		_, _ = s.db.ExecContext(r.Context(), `UPDATE auth_sessions SET last_seen_at=? WHERE token_hash=?`, now.Unix(), hash(cookie.Value))
	}
	return result, cookie.Value, nil
}

func (s *Service) setCookie(w http.ResponseWriter, name, value, path string, expires time.Time, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: path, Expires: expires, Secure: s.secure, HttpOnly: httpOnly, SameSite: http.SameSiteLaxMode})
}

func (s *Service) clearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{Name: name, Path: path, MaxAge: -1, Secure: s.secure, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func randomString() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func safeReturnPath(value string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.ContainsAny(value, "\\\r\n") {
		return "/"
	}
	u, err := url.ParseRequestURI(value)
	if err != nil || u.IsAbs() || u.Host != "" || u.Fragment != "" {
		return "/"
	}
	return value
}

package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/config"
	"github.com/exposedpudding/oxnetwork/internal/db"
)

func TestOIDCLoginSessionLogoutAndReplay(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var issuer, nonce string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/jwks", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "test", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
		case "/token":
			if err := r.ParseForm(); err != nil || r.Form.Get("code_verifier") == "" {
				t.Errorf("missing PKCE verifier: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-access", "token_type": "Bearer", "expires_in": 3600, "id_token": signedIDToken(t, key, issuer, nonce)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	issuer = provider.URL
	root := t.TempDir()
	secretPath := filepath.Join(root, "oidc-secret")
	if err := os.WriteFile(secretPath, []byte("test-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	cfg.Auth.IssuerURL = issuer
	cfg.Auth.ClientID = "oxnet-test"
	cfg.Auth.ClientSecretFile = secretPath
	cfg.Auth.PublicURL = "http://127.0.0.1:8080"
	database, err := db.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service, err := New(context.Background(), database, cfg)
	if err != nil {
		t.Fatal(err)
	}
	handler := service.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	request := func(path, method string, cookies []*http.Cookie, csrf string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, nil)
		for _, c := range cookies {
			r.AddCookie(c)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if got := request("/api/v1/nodes", "GET", nil, "").Code; got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated API: %d", got)
	}
	if got := request("/compute", "GET", nil, "").Code; got != http.StatusSeeOther {
		t.Fatalf("unauthenticated page: %d", got)
	}
	login := request("/auth/login?return_to=%2Fcompute", "GET", nil, "")
	if login.Code != http.StatusSeeOther {
		t.Fatalf("login: %d %s", login.Code, login.Body.String())
	}
	authURL := login.Header().Get("Location")
	if !strings.HasPrefix(authURL, issuer+"/authorize?") {
		t.Fatalf("authorization URL: %q", authURL)
	}
	query := mustURL(t, authURL).Query()
	nonce = query.Get("nonce")
	if nonce == "" || query.Get("code_challenge") == "" || query.Get("code_challenge_method") != "S256" {
		t.Fatalf("missing nonce or PKCE: %v", query)
	}
	var stateCookie *http.Cookie
	for _, c := range login.Result().Cookies() {
		if c.Name == stateCookieName() {
			stateCookie = c
		}
	}
	if stateCookie == nil || !stateCookie.HttpOnly {
		t.Fatal("missing HttpOnly login state cookie")
	}
	callbackPath := "/auth/callback?state=" + query.Get("state") + "&code=test-code"
	callback := request(callbackPath, "GET", []*http.Cookie{stateCookie}, "")
	if callback.Code != http.StatusSeeOther || callback.Header().Get("Location") != "/compute" {
		t.Fatalf("callback: %d %s %q", callback.Code, callback.Body.String(), callback.Header().Get("Location"))
	}
	var sessionCookie *http.Cookie
	for _, c := range callback.Result().Cookies() {
		if c.Name == "oxnet_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("missing protected session cookie")
	}
	if got := request("/api/v1/nodes", "GET", []*http.Cookie{sessionCookie}, "").Code; got != http.StatusOK {
		t.Fatalf("authenticated API: %d", got)
	}
	sessionResponse := request("/auth/session", "GET", []*http.Cookie{sessionCookie}, "")
	var sessionBody struct {
		Identity  Identity `json:"identity"`
		CSRFToken string   `json:"csrf_token"`
	}
	if err := json.Unmarshal(sessionResponse.Body.Bytes(), &sessionBody); err != nil {
		t.Fatal(err)
	}
	if sessionBody.Identity.Subject != "owner" || sessionBody.CSRFToken == "" {
		t.Fatalf("bad session: %+v", sessionBody)
	}
	if got := request("/auth/logout", "POST", []*http.Cookie{sessionCookie}, "").Code; got != http.StatusForbidden {
		t.Fatalf("logout without CSRF: %d", got)
	}
	if got := request("/auth/logout", "POST", []*http.Cookie{sessionCookie}, sessionBody.CSRFToken).Code; got != http.StatusNoContent {
		t.Fatalf("logout: %d", got)
	}
	if got := request("/api/v1/nodes", "GET", []*http.Cookie{sessionCookie}, "").Code; got != http.StatusUnauthorized {
		t.Fatalf("revoked session: %d", got)
	}
	if got := request(callbackPath, "GET", []*http.Cookie{stateCookie}, "").Code; got != http.StatusBadRequest {
		t.Fatalf("replayed callback: %d", got)
	}
}

func stateCookieName() string { return stateCookie }

func signedIDToken(t *testing.T, key *rsa.PrivateKey, issuer, nonce string) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": "test"})
	claims, _ := json.Marshal(map[string]any{"iss": issuer, "sub": "owner", "aud": "oxnet-test", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": nonce, "email": "owner@example.test"})
	message := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return message + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func mustURL(t *testing.T, value string) *url.URL {
	t.Helper()
	u, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestSafeReturnPath(t *testing.T) {
	for _, value := range []string{"//attacker.test", "https://attacker.test", "/\\attacker.test", ""} {
		if got := safeReturnPath(value); got != "/" {
			t.Fatalf("unsafe path %q became %q", value, got)
		}
	}
	if got := safeReturnPath("/network?kind=compute"); got != "/network?kind=compute" {
		t.Fatalf("safe path changed: %q", got)
	}
}

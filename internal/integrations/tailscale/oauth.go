package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const oauthTokenURL = "https://api.tailscale.com/api/v2/oauth/token"
const deviceReadScope = "devices:core:read"

// NewOAuthClient exchanges a read-only OAuth client credential for short-lived
// access tokens and renews them before expiry. Neither secret nor token is logged.
func NewOAuthClient(clientID, clientSecret string) *http.Client {
	return newOAuthClient(clientID, clientSecret, oauthTokenURL, &http.Client{Timeout: 10 * time.Second})
}

func newOAuthClient(clientID, clientSecret, tokenURL string, tokenHTTP *http.Client) *http.Client {
	source := oauth2.ReuseTokenSource(nil, oauthSource{
		clientID: clientID, clientSecret: clientSecret, tokenURL: tokenURL, http: tokenHTTP,
	})
	client := oauth2.NewClient(context.Background(), source)
	client.Timeout = 10 * time.Second
	return client
}

type oauthSource struct {
	clientID, clientSecret, tokenURL string
	http                             *http.Client
}

func (s oauthSource) Token() (*oauth2.Token, error) {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret},
		"scope":         {deviceReadScope},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, errors.New("Tailscale OAuth token request could not be created")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, errors.New("Tailscale OAuth token request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tailscale OAuth token request returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, errors.New("Tailscale OAuth token response was invalid")
	}
	if payload.AccessToken == "" || !strings.EqualFold(payload.TokenType, "Bearer") || payload.ExpiresIn <= 0 {
		return nil, errors.New("Tailscale OAuth token response was incomplete")
	}
	if payload.Scope != "" {
		found := false
		for _, scope := range strings.Fields(payload.Scope) {
			if scope == deviceReadScope {
				found = true
			}
		}
		if !found {
			return nil, errors.New("Tailscale OAuth token lacks device read scope")
		}
	}
	return &oauth2.Token{AccessToken: payload.AccessToken, TokenType: "Bearer", Expiry: time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)}, nil
}

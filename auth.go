package opensky

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	tokenURL           = "https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token"
	tokenRefreshMargin = 30 * time.Second
)

// tokenResponse represents the OAuth2 token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// tokenManager handles OAuth2 client credentials flow with automatic token refresh.
type tokenManager struct {
	mu           sync.Mutex
	clientID     string
	clientSecret string
	httpClient   *http.Client
	tokenURL     string

	accessToken string
	expiresAt   time.Time
}

// newTokenManager creates a new tokenManager for the given client credentials.
func newTokenManager(clientID, clientSecret string, httpClient *http.Client) *tokenManager {
	return &tokenManager{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   httpClient,
		tokenURL:     tokenURL,
	}
}

// getToken returns a valid access token, refreshing it automatically if expired or about to expire.
func (tm *tokenManager) getToken() (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.accessToken != "" && time.Now().Before(tm.expiresAt) {
		return tm.accessToken, nil
	}
	return tm.refresh()
}

// refresh fetches a new access token from the OpenSky authentication server.
func (tm *tokenManager) refresh() (string, error) {
	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {tm.clientID},
		"client_secret": {tm.clientSecret},
	}

	req, err := http.NewRequest(http.MethodPost, tm.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tm.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	var tokenResp tokenResponse
	if err = json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}

	tm.accessToken = tokenResp.AccessToken
	expiresIn := time.Duration(tokenResp.ExpiresIn) * time.Second
	tm.expiresAt = time.Now().Add(expiresIn - tokenRefreshMargin)

	return tm.accessToken, nil
}

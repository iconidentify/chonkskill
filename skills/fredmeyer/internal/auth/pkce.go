package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/iconidentify/chonkskill/skills/fredmeyer/internal/kroger"
)

type PKCEParams struct {
	CodeVerifier  string `json:"code_verifier"`
	CodeChallenge string `json:"code_challenge"`
	State         string `json:"state"`
}

type AuthManager struct {
	client    *kroger.Client
	tokenFile string

	mu         sync.Mutex
	pkceParams *PKCEParams
}

func NewAuthManager(client *kroger.Client, tokenFile string) *AuthManager {
	am := &AuthManager{
		client:    client,
		tokenFile: tokenFile,
	}
	am.loadToken()
	am.loadPKCE()
	return am
}

func (am *AuthManager) generatePKCE() (*PKCEParams, error) {
	verifier := make([]byte, 32)
	if _, err := rand.Read(verifier); err != nil {
		return nil, fmt.Errorf("generating verifier: %w", err)
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifier)

	h := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])

	state := make([]byte, 16)
	if _, err := rand.Read(state); err != nil {
		return nil, fmt.Errorf("generating state: %w", err)
	}

	return &PKCEParams{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		State:         base64.RawURLEncoding.EncodeToString(state),
	}, nil
}

func (am *AuthManager) pkceFile() string {
	dir := filepath.Dir(am.tokenFile)
	return filepath.Join(dir, "kroger_pkce_pending.json")
}

func (am *AuthManager) savePKCE() {
	if am.pkceParams == nil {
		os.Remove(am.pkceFile())
		return
	}
	data, err := json.Marshal(am.pkceParams)
	if err != nil {
		return
	}
	os.WriteFile(am.pkceFile(), data, 0600)
}

func (am *AuthManager) loadPKCE() {
	data, err := os.ReadFile(am.pkceFile())
	if err != nil {
		return
	}
	var params PKCEParams
	if err := json.Unmarshal(data, &params); err != nil {
		return
	}
	if params.CodeVerifier != "" && params.State != "" {
		am.pkceParams = &params
	}
}

// StartAuth generates PKCE params and returns the authorization URL.
func (am *AuthManager) StartAuth(scope string) (string, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	pkce, err := am.generatePKCE()
	if err != nil {
		return "", err
	}
	am.pkceParams = pkce
	am.savePKCE()

	authURL := am.client.AuthorizationURL(scope, pkce.State, pkce.CodeChallenge)
	return authURL, nil
}

// CompleteAuth exchanges the redirect URL for tokens.
func (am *AuthManager) CompleteAuth(redirectURL string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.pkceParams == nil {
		return fmt.Errorf("no pending auth flow, call StartAuth first")
	}

	u, err := url.Parse(redirectURL)
	if err != nil {
		return fmt.Errorf("parsing redirect URL: %w", err)
	}

	code := u.Query().Get("code")
	state := u.Query().Get("state")

	if state != am.pkceParams.State {
		return fmt.Errorf("state mismatch: expected %s, got %s", am.pkceParams.State, state)
	}

	if code == "" {
		errMsg := u.Query().Get("error")
		errDesc := u.Query().Get("error_description")
		return fmt.Errorf("no code in redirect URL, error: %s, description: %s", errMsg, errDesc)
	}

	token, err := am.client.ExchangeCode(code, am.pkceParams.CodeVerifier)
	if err != nil {
		return fmt.Errorf("exchanging code: %w", err)
	}

	am.pkceParams = nil
	am.savePKCE()
	return am.saveToken(token)
}

func (am *AuthManager) saveToken(token *kroger.Token) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling token: %w", err)
	}
	return os.WriteFile(am.tokenFile, data, 0600)
}

func (am *AuthManager) loadToken() {
	data, err := os.ReadFile(am.tokenFile)
	if err != nil {
		return
	}

	var token kroger.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return
	}

	if token.ExpiresAt.IsZero() {
		token.ExpiresAt = time.Now().Add(-1 * time.Second)
	}

	am.client.SetUserToken(&token)
}

// IsAuthenticated checks if we have a valid user token.
func (am *AuthManager) IsAuthenticated() bool {
	token := am.client.GetUserToken()
	if token == nil {
		return false
	}
	if token.IsExpired() {
		newToken, err := am.client.RefreshUserToken()
		if err != nil {
			return false
		}
		_ = am.saveToken(newToken)
		return true
	}
	return true
}

// ForceReauth invalidates the current user token.
func (am *AuthManager) ForceReauth() {
	am.client.SetUserToken(nil)
	os.Remove(am.tokenFile)
	am.pkceParams = nil
	am.savePKCE()
}

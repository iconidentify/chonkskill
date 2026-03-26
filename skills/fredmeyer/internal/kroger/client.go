package kroger

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Client struct {
	clientID     string
	clientSecret string
	redirectURI  string
	baseURL      string
	httpClient   *http.Client

	mu          sync.RWMutex
	clientToken *Token
	userToken   *Token
}

type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt.Add(-30 * time.Second))
}

type ClientConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	BaseURL      string
}

func NewClient(cfg ClientConfig) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.kroger.com/v1"
	}
	return &Client{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURI:  cfg.RedirectURI,
		baseURL:      cfg.BaseURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) basicAuth() string {
	creds := c.clientID + ":" + c.clientSecret
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

func (c *Client) GetClientToken(scope string) (*Token, error) {
	c.mu.RLock()
	if c.clientToken != nil && !c.clientToken.IsExpired() {
		t := c.clientToken
		c.mu.RUnlock()
		return t, nil
	}
	c.mu.RUnlock()

	data := url.Values{
		"grant_type": {"client_credentials"},
		"scope":      {scope},
	}

	req, err := http.NewRequest("POST", c.baseURL+"/connect/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", c.basicAuth())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token request returned %d: %s", resp.StatusCode, string(body))
	}

	var token Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decoding token: %w", err)
	}
	token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	c.mu.Lock()
	c.clientToken = &token
	c.mu.Unlock()

	return &token, nil
}

func (c *Client) SetUserToken(token *Token) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userToken = token
}

func (c *Client) GetUserToken() *Token {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userToken
}

func (c *Client) RefreshUserToken() (*Token, error) {
	c.mu.RLock()
	ut := c.userToken
	c.mu.RUnlock()

	if ut == nil || ut.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available, user must re-authenticate")
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {ut.RefreshToken},
	}

	req, err := http.NewRequest("POST", c.baseURL+"/connect/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", c.basicAuth())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh request returned %d: %s", resp.StatusCode, string(body))
	}

	var token Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decoding refresh token: %w", err)
	}
	token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	c.mu.Lock()
	c.userToken = &token
	c.mu.Unlock()

	return &token, nil
}

func (c *Client) doAuthenticatedRequest(method, path string, body interface{}, useUserToken bool) (*http.Response, error) {
	var token *Token
	var err error

	if useUserToken {
		token = c.GetUserToken()
		if token == nil || token.IsExpired() {
			token, err = c.RefreshUserToken()
			if err != nil {
				return nil, fmt.Errorf("user not authenticated: %w", err)
			}
		}
	} else {
		token, err = c.GetClientToken("product.compact")
		if err != nil {
			return nil, err
		}
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized && useUserToken {
		resp.Body.Close()
		token, err = c.RefreshUserToken()
		if err != nil {
			return nil, fmt.Errorf("re-auth failed: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		return c.httpClient.Do(req)
	}

	return resp, nil
}

// SearchLocations finds Kroger stores near a zip code.
func (c *Client) SearchLocations(zipCode string, radiusMiles int, limit int, chain string) (json.RawMessage, error) {
	params := url.Values{
		"filter.zipCode.near":   {zipCode},
		"filter.radiusInMiles":  {fmt.Sprintf("%d", radiusMiles)},
		"filter.limit":          {fmt.Sprintf("%d", limit)},
	}
	if chain != "" {
		params.Set("filter.chain", chain)
	}

	resp, err := c.doAuthenticatedRequest("GET", "/locations?"+params.Encode(), nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("locations search returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// GetLocationDetails returns details for a specific location.
func (c *Client) GetLocationDetails(locationID string) (json.RawMessage, error) {
	resp, err := c.doAuthenticatedRequest("GET", "/locations/"+locationID, nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("location details returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// SearchProducts searches for products at a specific location.
func (c *Client) SearchProducts(term string, locationID string, limit int, brand string, fulfillment string) (json.RawMessage, error) {
	params := url.Values{
		"filter.term":  {term},
		"filter.limit": {fmt.Sprintf("%d", limit)},
	}
	if locationID != "" {
		params.Set("filter.locationId", locationID)
	}
	if brand != "" {
		params.Set("filter.brand", brand)
	}
	if fulfillment != "" {
		params.Set("filter.fulfillment", fulfillment)
	}

	resp, err := c.doAuthenticatedRequest("GET", "/products?"+params.Encode(), nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("product search returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// GetProductDetails returns details for a specific product.
func (c *Client) GetProductDetails(productID string, locationID string) (json.RawMessage, error) {
	path := "/products/" + productID
	if locationID != "" {
		path += "?filter.locationId=" + locationID
	}

	resp, err := c.doAuthenticatedRequest("GET", path, nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("product details returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// AddToCart adds items to the authenticated user's Kroger cart.
type CartItem struct {
	UPC      string `json:"upc"`
	Quantity int    `json:"quantity"`
}

func (c *Client) AddToCart(items []CartItem) error {
	payload := map[string]interface{}{
		"items": items,
	}

	resp, err := c.doAuthenticatedRequest("PUT", "/cart/add", payload, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add to cart returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetProfile returns the authenticated user's profile.
func (c *Client) GetProfile() (json.RawMessage, error) {
	resp, err := c.doAuthenticatedRequest("GET", "/identity/profile", nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("profile returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// GetChains returns all Kroger-owned chains.
func (c *Client) GetChains() (json.RawMessage, error) {
	resp, err := c.doAuthenticatedRequest("GET", "/chains", nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chains returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// GetDepartments returns all departments.
func (c *Client) GetDepartments() (json.RawMessage, error) {
	resp, err := c.doAuthenticatedRequest("GET", "/departments", nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("departments returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// AuthorizationURL returns the URL the user must visit to authorize the app.
func (c *Client) AuthorizationURL(scope string, state string, codeChallenge string) string {
	params := url.Values{
		"scope":                 {scope},
		"response_type":        {"code"},
		"client_id":            {c.clientID},
		"redirect_uri":         {c.redirectURI},
		"state":                {state},
		"code_challenge":       {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	return c.baseURL + "/connect/oauth2/authorize?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens.
func (c *Client) ExchangeCode(code string, codeVerifier string) (*Token, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.redirectURI},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequest("POST", c.baseURL+"/connect/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", c.basicAuth())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("exchange returned %d: %s", resp.StatusCode, string(body))
	}

	var token Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decoding token: %w", err)
	}
	token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	c.SetUserToken(&token)
	return &token, nil
}

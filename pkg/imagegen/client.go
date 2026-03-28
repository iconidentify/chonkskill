// Package imagegen provides an image generation client that talks to LiteLLM's
// OpenAI-compatible /v1/images/generations endpoint. This routes image generation
// through the same LiteLLM proxy as text completions, allowing any backend
// (Gemini Imagen, Flux, DALL-E, Stable Diffusion, fal.ai models, etc.) to be
// configured in the LiteLLM model list without code changes.
package imagegen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const DefaultImageModel = "gemini-2.0-flash-preview-image-generation"

// Client generates images via LiteLLM's /v1/images/generations endpoint.
type Client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewClient creates an image generation client.
// apiKey is the LiteLLM API key (Bearer token).
// baseURL is the LiteLLM proxy URL (e.g. http://localhost:4000).
// model is the image model name configured in LiteLLM.
func NewClient(apiKey, baseURL, model string) *Client {
	if model == "" {
		model = DefaultImageModel
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

// GenerateParams configures image generation.
type GenerateParams struct {
	Prompt string
	Size   string // e.g. "1024x1024", "1024x1536", "1536x1024"
	N      int    // Number of images, default 1
}

// GenerateResult holds the generated image URL.
type GenerateResult struct {
	ImageURL string `json:"image_url"`
}

// Generate creates an image from a text prompt.
func (c *Client) Generate(params GenerateParams) (*GenerateResult, error) {
	if params.N == 0 {
		params.N = 1
	}
	if params.Size == "" {
		params.Size = "1024x1024"
	}

	payload := map[string]any{
		"model":  c.model,
		"prompt": params.Prompt,
		"n":      params.N,
		"size":   params.Size,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := c.baseURL + "/v1/images/generations"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image generation API returned %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	// OpenAI image generation response format.
	var apiResp struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	result := &GenerateResult{}
	if len(apiResp.Data) > 0 {
		result.ImageURL = apiResp.Data[0].URL
	}

	if result.ImageURL == "" {
		return nil, fmt.Errorf("no image URL in response")
	}

	return result, nil
}

// DownloadImage downloads an image from a URL to a local file.
func DownloadImage(url, destPath string) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return 0, err
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("downloading image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download returned %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return io.Copy(f, resp.Body)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

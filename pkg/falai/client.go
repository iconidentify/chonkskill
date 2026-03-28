// Package falai provides an HTTP client for the fal.ai image generation API.
// Used for cover art, chapter ornaments, maps, and scene break decorations.
package falai

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

const (
	DefaultGenerateURL = "https://fal.run/fal-ai/nano-banana-2"
	DefaultEditURL     = "https://fal.run/fal-ai/nano-banana-2/edit"
)

// Client talks to the fal.ai API.
type Client struct {
	apiKey      string
	generateURL string
	editURL     string
	httpClient  *http.Client
}

// NewClient creates a fal.ai client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:      apiKey,
		generateURL: DefaultGenerateURL,
		editURL:     DefaultEditURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

// GenerateParams configures image generation.
type GenerateParams struct {
	Prompt      string
	Resolution  string // e.g. "1024x1024"
	AspectRatio string // e.g. "16:9", "1:1"
	Seed        int
}

// GenerateResult holds the response from an image generation call.
type GenerateResult struct {
	ImageURL    string `json:"image_url"`
	Description string `json:"description,omitempty"`
}

// Generate creates a new image from a prompt.
func (c *Client) Generate(params GenerateParams) (*GenerateResult, error) {
	payload := map[string]any{
		"prompt":            params.Prompt,
		"num_images":        1,
		"output_format":     "png",
		"safety_tolerance":  "6",
		"limit_generations": true,
		"thinking_level":    "high",
	}
	if params.Resolution != "" {
		payload["resolution"] = params.Resolution
	}
	if params.AspectRatio != "" {
		payload["aspect_ratio"] = params.AspectRatio
	}
	if params.Seed > 0 {
		payload["seed"] = params.Seed
	}

	return c.post(c.generateURL, payload)
}

// EditParams configures image editing with a reference image.
type EditParams struct {
	Prompt      string
	ImageURLs   []string
	Resolution  string
	AspectRatio string
	Seed        int
}

// Edit creates an image using reference images for style consistency.
func (c *Client) Edit(params EditParams) (*GenerateResult, error) {
	payload := map[string]any{
		"prompt":            params.Prompt,
		"image_urls":        params.ImageURLs,
		"num_images":        1,
		"output_format":     "png",
		"safety_tolerance":  "6",
		"limit_generations": true,
		"thinking_level":    "high",
	}
	if params.Resolution != "" {
		payload["resolution"] = params.Resolution
	}
	if params.AspectRatio != "" {
		payload["aspect_ratio"] = params.AspectRatio
	}
	if params.Seed > 0 {
		payload["seed"] = params.Seed
	}

	return c.post(c.editURL, payload)
}

// DownloadImage downloads an image from a URL to a local file.
func (c *Client) DownloadImage(url, destPath string) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Get(url)
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

func (c *Client) post(url string, payload map[string]any) (*GenerateResult, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Key "+c.apiKey)

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
		return nil, fmt.Errorf("fal.ai API returned %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	var apiResp struct {
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	result := &GenerateResult{}
	if len(apiResp.Images) > 0 {
		result.ImageURL = apiResp.Images[0].URL
	}

	return result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

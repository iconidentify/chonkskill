// Package xai provides an HTTP client for the xAI Responses API with x_search
// tool support. It calls Grok with x_search enabled, which gives real-time
// access to X (Twitter) posts, trends, and discussions.
package xai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.x.ai/v1/responses"

// Client talks to the xAI Responses API.
type Client struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates an xAI client. Model defaults to grok-3-latest if empty.
func NewClient(apiKey, model string) *Client {
	if model == "" {
		model = "grok-4-0709"
	}
	return &Client{
		apiKey:  apiKey,
		model:   model,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Grok searches + reasoning can take a while
		},
	}
}

// SearchParams configures the x_search tool.
type SearchParams struct {
	Query                    string
	AllowedHandles           []string
	ExcludedHandles          []string
	FromDate                 string // YYYY-MM-DD
	ToDate                   string // YYYY-MM-DD
	EnableImageUnderstanding bool
	EnableVideoUnderstanding bool
}

// SearchResult holds the structured response from an x_search call.
type SearchResult struct {
	Text      string     `json:"text"`
	Citations []Citation `json:"citations,omitempty"`
	Model     string     `json:"model,omitempty"`
	Usage     *Usage     `json:"usage,omitempty"`
}

// Citation is a reference to an X post or web source.
type Citation struct {
	Type  string `json:"type,omitempty"`
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

// Search executes an x_search via the xAI Responses API.
func (c *Client) Search(params SearchParams) (*SearchResult, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Build the x_search tool definition.
	tool := map[string]any{
		"type": "x_search",
	}

	if len(params.AllowedHandles) > 0 {
		if len(params.AllowedHandles) > 10 {
			params.AllowedHandles = params.AllowedHandles[:10]
		}
		// Strip @ prefixes if present.
		handles := make([]string, len(params.AllowedHandles))
		for i, h := range params.AllowedHandles {
			handles[i] = strings.TrimPrefix(h, "@")
		}
		tool["x_handles"] = handles
	}

	if len(params.ExcludedHandles) > 0 {
		if len(params.ExcludedHandles) > 10 {
			params.ExcludedHandles = params.ExcludedHandles[:10]
		}
		handles := make([]string, len(params.ExcludedHandles))
		for i, h := range params.ExcludedHandles {
			handles[i] = strings.TrimPrefix(h, "@")
		}
		tool["excluded_x_handles"] = handles
	}

	if params.FromDate != "" {
		tool["from_date"] = params.FromDate
	}
	if params.ToDate != "" {
		tool["to_date"] = params.ToDate
	}
	if params.EnableImageUnderstanding {
		tool["enable_image_understanding"] = true
	}
	if params.EnableVideoUnderstanding {
		tool["enable_video_understanding"] = true
	}

	payload := map[string]any{
		"model": c.model,
		"tools": []any{tool},
		"input": []map[string]string{
			{"role": "user", "content": params.Query},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewReader(body))
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
		return nil, fmt.Errorf("xAI API returned %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	// Parse the response.
	var apiResp responsesAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	result := &SearchResult{
		Model: apiResp.Model,
	}

	if apiResp.Usage != nil {
		result.Usage = &Usage{
			InputTokens:  apiResp.Usage.InputTokens,
			OutputTokens: apiResp.Usage.OutputTokens,
		}
	}

	// Extract text from output messages.
	var textParts []string
	for _, item := range apiResp.Output {
		if item.Type == "message" {
			for _, block := range item.Content {
				if block.Type == "output_text" {
					textParts = append(textParts, block.Text)
				}
			}
		}
	}
	result.Text = strings.Join(textParts, "\n")

	// Extract citations from annotations within output_text blocks,
	// and also from any top-level citations field.
	seen := make(map[string]bool)
	for _, item := range apiResp.Output {
		if item.Type == "message" {
			for _, block := range item.Content {
				for _, ann := range block.Annotations {
					if ann.URL != "" && !seen[ann.URL] {
						seen[ann.URL] = true
						result.Citations = append(result.Citations, Citation{
							Type:  ann.Type,
							URL:   ann.URL,
							Title: ann.Title,
						})
					}
				}
			}
		}
	}

	return result, nil
}

// Internal types for parsing the xAI Responses API JSON.

type responsesAPIResponse struct {
	Model  string       `json:"model"`
	Output []outputItem `json:"output"`
	Usage  *apiUsage    `json:"usage,omitempty"`
}

type apiUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type outputItem struct {
	Type    string         `json:"type"`
	Content []contentBlock `json:"content,omitempty"`
}

type contentBlock struct {
	Type        string       `json:"type"`
	Text        string       `json:"text,omitempty"`
	Annotations []annotation `json:"annotations,omitempty"`
}

type annotation struct {
	Type  string `json:"type,omitempty"`
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

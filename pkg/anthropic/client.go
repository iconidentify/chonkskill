// Package anthropic provides an HTTP client for LLM completion via the
// Anthropic Messages API format (/v1/messages). Designed to talk to a LiteLLM
// proxy which exposes the same endpoint with standard Bearer auth.
//
// The request body and response format match the Anthropic Messages API exactly,
// which LiteLLM mirrors at /v1/messages. Auth uses Authorization: Bearer.
package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	// DefaultWriter is the default model for creative generation tasks.
	DefaultWriter = "claude-sonnet-4-6"
	// DefaultJudge is the default model for evaluation tasks.
	DefaultJudge = "claude-opus-4-6"
	// DefaultReview is the default model for deep manuscript review.
	DefaultReview = "claude-opus-4-6"
)

// Client talks to an LLM via the Anthropic Messages API format.
// Connects to a LiteLLM proxy using Bearer token auth.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates an LLM client that posts to {baseURL}/v1/messages.
// apiKey is the LiteLLM API key (sent as Bearer token).
// baseURL is the LiteLLM proxy URL (e.g. http://localhost:4000).
func NewClient(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 600 * time.Second,
		},
	}
}

// Request configures a single API call.
type Request struct {
	Model       string
	System      string
	Prompt      string
	MaxTokens   int
	Temperature float64
}

// Response holds the parsed API response.
type Response struct {
	Text       string
	InputToks  int
	OutputToks int
	StopReason string
}

// Message sends a single-turn message and returns the text response.
func (c *Client) Message(req Request) (*Response, error) {
	if req.MaxTokens == 0 {
		req.MaxTokens = 8000
	}

	payload := map[string]any{
		"model":       req.Model,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
		"messages":    []map[string]string{{"role": "user", "content": req.Prompt}},
	}
	if req.System != "" {
		payload["system"] = req.System
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	var apiResp messagesResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	result := &Response{
		StopReason: apiResp.StopReason,
	}
	if apiResp.Usage != nil {
		result.InputToks = apiResp.Usage.InputTokens
		result.OutputToks = apiResp.Usage.OutputTokens
	}

	var parts []string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	result.Text = strings.Join(parts, "")

	return result, nil
}

// ParseJSON extracts a JSON object from LLM output text.
// Handles markdown code fences, leading text, and malformed JSON with
// bracket-matching fallback. This replicates the robust parse_json_response
// from the original evaluate.py.
func ParseJSON(text string) (map[string]any, error) {
	// Strip markdown code fences.
	cleaned := text
	cleaned = regexp.MustCompile("(?s)```(?:json)?\\s*\n?").ReplaceAllString(cleaned, "")
	cleaned = strings.ReplaceAll(cleaned, "```", "")
	cleaned = strings.TrimSpace(cleaned)

	// Find the first '{'.
	idx := strings.IndexByte(cleaned, '{')
	if idx < 0 {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	cleaned = cleaned[idx:]

	// Try standard JSON parse.
	var result map[string]any
	if err := json.Unmarshal([]byte(cleaned), &result); err == nil {
		return result, nil
	}

	// Bracket-matching fallback for malformed JSON.
	depth := 0
	inString := false
	escaped := false
	end := -1
	for i, ch := range cleaned {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if end > 0 {
		if err := json.Unmarshal([]byte(cleaned[:end]), &result); err == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("failed to parse JSON from response")
}

// ParseJSONArray extracts a JSON array from LLM output text.
func ParseJSONArray(text string) ([]map[string]any, error) {
	cleaned := text
	cleaned = regexp.MustCompile("(?s)```(?:json)?\\s*\n?").ReplaceAllString(cleaned, "")
	cleaned = strings.ReplaceAll(cleaned, "```", "")
	cleaned = strings.TrimSpace(cleaned)

	idx := strings.IndexByte(cleaned, '[')
	if idx < 0 {
		return nil, fmt.Errorf("no JSON array found in response")
	}
	cleaned = cleaned[idx:]

	var result []map[string]any
	if err := json.Unmarshal([]byte(cleaned), &result); err == nil {
		return result, nil
	}

	// Bracket-matching fallback.
	depth := 0
	inString := false
	escaped := false
	end := -1
	for i, ch := range cleaned {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '[' {
			depth++
		} else if ch == ']' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if end > 0 {
		if err := json.Unmarshal([]byte(cleaned[:end]), &result); err == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("failed to parse JSON array from response")
}

// ParseScore extracts a score value from YAML-like "key: N.N" output.
// This replicates parse_score from run_pipeline.py.
func ParseScore(text, key string) (float64, bool) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+":") {
			val := strings.TrimSpace(strings.TrimPrefix(line, key+":"))
			var score float64
			if _, err := fmt.Sscanf(val, "%f", &score); err == nil {
				return score, true
			}
		}
	}
	return 0, false
}

type messagesResponse struct {
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      *apiUsage      `json:"usage,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type apiUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// CountWords counts words in text, matching Python's split() behavior.
func CountWords(text string) int {
	return len(strings.Fields(text))
}

// TruncateWords returns the first n words of text.
func TruncateWords(text string, n int) string {
	words := strings.Fields(text)
	if len(words) <= n {
		return text
	}
	return strings.Join(words[:n], " ")
}

// SanitizeForPrompt removes control characters that could confuse the model.
func SanitizeForPrompt(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

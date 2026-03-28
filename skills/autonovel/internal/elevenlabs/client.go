// Package elevenlabs provides an HTTP client for the ElevenLabs Text to Dialogue API.
// Used for generating multi-voice audiobook narration from parsed chapter scripts.
package elevenlabs

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
	DefaultBaseURL   = "https://api.elevenlabs.io/v1"
	MaxCharsPerCall  = 4500
	PauseBetweenMs   = 3000
)

// Client talks to the ElevenLabs API.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates an ElevenLabs client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

// Segment represents one narration segment in a dialogue script.
type Segment struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

// Voice maps a character name to an ElevenLabs voice ID.
type Voice struct {
	Name    string `json:"name"`
	VoiceID string `json:"voice_id"`
}

// DialogueInput is the API input for text-to-dialogue conversion.
type DialogueInput struct {
	Text    string `json:"text"`
	VoiceID string `json:"voice_id"`
}

// TextToDialogue converts dialogue segments to audio.
// Returns raw MP3 audio bytes.
func (c *Client) TextToDialogue(inputs []DialogueInput) ([]byte, error) {
	payload := map[string]any{
		"inputs": inputs,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/text-to-dialogue", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ElevenLabs API returned %d: %s", resp.StatusCode, truncate(string(errBody), 500))
	}

	return io.ReadAll(resp.Body)
}

// ListVoices returns all available voices.
func (c *Client) ListVoices() ([]Voice, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/voices", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var result struct {
		Voices []struct {
			VoiceID string `json:"voice_id"`
			Name    string `json:"name"`
		} `json:"voices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	voices := make([]Voice, len(result.Voices))
	for i, v := range result.Voices {
		voices[i] = Voice{Name: v.Name, VoiceID: v.VoiceID}
	}
	return voices, nil
}

// ChunkSegments splits segments into API-call-sized chunks.
func ChunkSegments(segments []Segment, voices map[string]string, defaultVoiceID string, maxChars int) [][]DialogueInput {
	if maxChars == 0 {
		maxChars = MaxCharsPerCall
	}

	var chunks [][]DialogueInput
	var currentChunk []DialogueInput
	currentChars := 0

	for _, seg := range segments {
		voiceID := defaultVoiceID
		if vid, ok := voices[seg.Speaker]; ok {
			voiceID = vid
		}

		segChars := len(seg.Text)
		if currentChars+segChars > maxChars && len(currentChunk) > 0 {
			chunks = append(chunks, currentChunk)
			currentChunk = nil
			currentChars = 0
		}

		currentChunk = append(currentChunk, DialogueInput{
			Text:    seg.Text,
			VoiceID: voiceID,
		})
		currentChars += segChars
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// SaveAudio writes audio bytes to a file.
func SaveAudio(data []byte, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ConcatAudioFiles concatenates MP3 files by simple byte concatenation.
// This works for MP3 but not for other formats.
func ConcatAudioFiles(paths []string, outputPath string) error {
	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("reading %s: %w", p, err)
		}
		if _, err := out.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

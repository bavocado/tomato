package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// AnthropicProvider implements Provider using Anthropic's native Messages API.
// Configured via environment variables:
//
//	ANTHROPIC_BASE_URL    — API endpoint (default: https://api.anthropic.com)
//	ANTHROPIC_AUTH_TOKEN  — API key sent as x-api-key header
//	ANTHROPIC_MODEL       — model name sent in the request body
type AnthropicProvider struct {
	BaseURL   string
	AuthToken string
	ModelName string
}

// anthropicRequest is the Messages API request body.
type anthropicRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []anthropicMsg  `json:"messages"`
	Stream    bool            `json:"stream"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicStreamEvent matches a single SSE event line.
type anthropicStreamEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index,omitempty"`
	Delta *anthropicDelta `json:"delta,omitempty"`
}

type anthropicDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (p *AnthropicProvider) Model() string {
	return p.ModelName
}

func (p *AnthropicProvider) Stream(messages []Message, onChunk func(string)) error {
	// Convert generic Messages to Anthropic format
	anthroMsgs := make([]anthropicMsg, len(messages))
	for i, m := range messages {
		role := m.Role
		// Anthropic uses "assistant" for assistant messages; "system" is not in the messages array
		// but is sent separately — we skip system role here and prepend it as system prompt
		if role == "system" {
			// We'll handle system via a separate system parameter below
			role = "user"
		}
		anthroMsgs[i] = anthropicMsg{Role: role, Content: m.Content}
	}

	// Find the system message and separate it
	var systemPrompt string
	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			break
		}
	}

	body := anthropicRequest{
		Model:     p.ModelName,
		MaxTokens: 4096,
		Messages:  anthroMsgs,
		Stream:    true,
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling anthropic request: %w", err)
	}

	req, err := http.NewRequest("POST", p.BaseURL+"/v1/messages", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("creating anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.AuthToken)
	req.Header.Set("anthropic-version", "2023-06-01")
	if systemPrompt != "" {
		req.Header.Set("anthropic-system-prompt", systemPrompt)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling Anthropic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Anthropic returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Anthropic SSE format:
	//   event: content_block_delta
	//   data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}
	scanner := bufio.NewScanner(resp.Body)
	var currentEvent string
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if currentEvent != "content_block_delta" {
				continue
			}

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if event.Delta != nil && event.Delta.Type == "text_delta" {
				onChunk(event.Delta.Text)
			}
		}
	}

	return scanner.Err()
}

// NewAnthropicProvider creates an AnthropicProvider from environment variables.
func NewAnthropicProvider(modelID string) (*AnthropicProvider, error) {
	authToken := getEnv("ANTHROPIC_AUTH_TOKEN", "")
	if authToken == "" {
		return nil, fmt.Errorf("ANTHROPIC_AUTH_TOKEN environment variable is required")
	}

	baseURL := getEnv("ANTHROPIC_BASE_URL", "https://api.anthropic.com")
	// Strip trailing slash
	baseURL = strings.TrimRight(baseURL, "/")

	modelName := getEnv("ANTHROPIC_MODEL", "")
	if modelName == "" {
		// Fall back to the model part of modelID (e.g. "anthropic/claude-sonnet-4-20250514")
		parts := strings.SplitN(modelID, "/", 2)
		if len(parts) == 2 {
			modelName = parts[1]
		} else {
			modelName = "claude-sonnet-4-20250514"
		}
	}

	return &AnthropicProvider{
		BaseURL:   baseURL,
		AuthToken: authToken,
		ModelName: modelName,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
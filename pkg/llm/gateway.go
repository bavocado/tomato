package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAIProvider implements Provider for the OpenAI-compatible protocol.
type OpenAIProvider struct {
	BaseURL   string
	APIKey    string
	modelName string
}

// chatRequest is the OpenAI chat completion request body.
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// chatStreamChunk is a single SSE chunk from the streaming response.
type chatStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
	} `json:"choices"`
}

func (p *OpenAIProvider) Model() string {
	return p.modelName
}

func (p *OpenAIProvider) Stream(messages []Message, onChunk func(string)) error {
	body := chatRequest{
		Model:    p.Model(),
		Messages: messages,
		Stream:   true,
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", p.BaseURL+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling LLM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk chatStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed chunks
		}
		for _, choice := range chunk.Choices {
			onChunk(choice.Delta.Content)
		}
	}

	return scanner.Err()
}

// NewProvider creates a Provider from a model identifier string.
// Format: "provider/model", e.g., "openai/gpt-5", "glm/glm-5.2", "deepseek/deepseek-4pro".
// All use the OpenAI-compatible protocol; only base_url differs.
func NewProvider(modelID, apiKey string) (Provider, error) {
	parts := strings.SplitN(modelID, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid model format %q, expected provider/model", modelID)
	}

	providerName := parts[0]
	modelName := parts[1]

	baseURL := defaultBaseURL(providerName)
	return &OpenAIProvider{
		BaseURL: baseURL,
		APIKey:  apiKey,
		modelName: modelName,
	}, nil
}

// baseURLs maps provider names to their API endpoints.
var baseURLs = map[string]string{
	"openai":   "https://api.openai.com/v1",
	"glm":      "https://open.bigmodel.cn/api/paas/v4",
	"deepseek": "https://api.deepseek.com",
}

func defaultBaseURL(provider string) string {
	if url, ok := baseURLs[provider]; ok {
		return url
	}
	return "https://api.openai.com/v1"
}

// EnvKeyName returns the environment variable name for a provider's API key.
func EnvKeyName(provider string) string {
	return fmt.Sprintf("%s_API_KEY", strings.ToUpper(strings.ReplaceAll(provider, "-", "_")))
}

// ResolveModel picks the model for a step, falling back to the default.
func ResolveModel(stepName string, config map[string]string) string {
	if m, ok := config[stepName]; ok {
		return m
	}
	return config["default"]
}
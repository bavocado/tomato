package llm

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIStream(t *testing.T) {
	// Mock server that streams an OpenAI-compatible response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing or wrong auth header")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		body := `data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-5","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"}}]}

data: [DONE]

`
		io.WriteString(w, body)
	}))
	defer server.Close()

	provider := &OpenAIProvider{
		BaseURL:   server.URL,
		APIKey:    "test-key",
		modelName: "gpt-5",
	}

	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Say hello"},
	}

	var result strings.Builder
	err := provider.Stream(messages, func(chunk string) {
		result.WriteString(chunk)
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.String() != "Hello world" {
		t.Errorf("expected 'Hello world', got '%s'", result.String())
	}
}

func TestModelFromConfig(t *testing.T) {
	config := map[string]string{
		"default": "deepseek/deepseek-4pro",
		"impl":    "glm/glm-5.2",
		"spec":    "openai/gpt-5",
		"review":  "glm/glm-5.2",
		"test":    "deepseek/deepseek-4pro",
	}
	stepName := "impl"
	expected := "glm/glm-5.2"

	model := ResolveModel(stepName, config)
	if model != expected {
		t.Errorf("for step %s, expected model %s, got %s", stepName, expected, model)
	}

	// Fallback to default
	model = ResolveModel("unknown-step", config)
	if model != "deepseek/deepseek-4pro" {
		t.Errorf("expected fallback to default, got %s", model)
	}
}

func TestNewProvider(t *testing.T) {
	tests := []struct {
		modelID  string
		wantErr  bool
		wantType string
	}{
		{"openai/gpt-5", false, "openai"},
		{"glm/glm-5.2", false, "openai"},
		{"deepseek/deepseek-4pro", false, "openai"},
		{"anthropic/claude-sonnet-5", false, "claude-cli"},
		{"invalid", true, ""},
	}

	for _, tt := range tests {
		p, err := NewProvider(ProviderConfig{
			ModelID: tt.modelID,
		})
		if tt.wantErr {
			if err == nil {
				t.Errorf("expected error for %s", tt.modelID)
			}
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for %s: %v", tt.modelID, err)
			continue
		}
		if p.Model() != strings.SplitN(tt.modelID, "/", 2)[1] {
			t.Errorf("expected model %s, got %s", strings.SplitN(tt.modelID, "/", 2)[1], p.Model())
		}
		switch tt.wantType {
		case "claude-cli":
			if _, ok := p.(*ClaudeCLIProvider); !ok {
				t.Errorf("expected %s to use ClaudeCLIProvider, got %T", tt.modelID, p)
			}
		case "openai":
			if _, ok := p.(*OpenAIProvider); !ok {
				t.Errorf("expected %s to use OpenAIProvider, got %T", tt.modelID, p)
			}
		}
	}
}

func TestNewProviderUsesProviderConfigForOpenAI(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		ModelID:   "glm/glm-5.2",
		BaseURL:   "http://127.0.0.1:1980",
		AuthToken: "sk-ai-router",
		Model:     "glm:glm-5.2",
	})
	if err != nil {
		t.Fatal(err)
	}

	prov, ok := p.(*OpenAIProvider)
	if !ok {
		t.Fatalf("expected OpenAIProvider, got %T", p)
	}
	if prov.BaseURL != "http://127.0.0.1:1980" {
		t.Errorf("expected base url from config, got %s", prov.BaseURL)
	}
	if prov.APIKey != "sk-ai-router" {
		t.Errorf("expected auth token from config, got %s", prov.APIKey)
	}
	if prov.Model() != "glm:glm-5.2" {
		t.Errorf("expected model glm:glm-5.2, got %s", prov.Model())
	}
}

func TestChatCompletionsURL(t *testing.T) {
	tests := []struct {
		base string
		want string
	}{
		// ai-router-style bare host:port gets /v1 prepended (otherwise 404).
		{"http://127.0.0.1:1980", "http://127.0.0.1:1980/v1/chat/completions"},
		// Default provider URLs already carry a version segment.
		{"https://api.openai.com/v1", "https://api.openai.com/v1/chat/completions"},
		{"https://open.bigmodel.cn/api/paas/v4", "https://open.bigmodel.cn/api/paas/v4/chat/completions"},
		// Trailing path with no version segment still gets /v1.
		{"https://example.com/proxy", "https://example.com/proxy/v1/chat/completions"},
	}
	for _, tt := range tests {
		if got := chatCompletionsURL(tt.base); got != tt.want {
			t.Errorf("chatCompletionsURL(%q) = %q, want %q", tt.base, got, tt.want)
		}
	}
}

func TestEnvKeyName(t *testing.T) {
	if EnvKeyName("openai") != "OPENAI_API_KEY" {
		t.Errorf("expected OPENAI_API_KEY, got %s", EnvKeyName("openai"))
	}
	if EnvKeyName("deepseek") != "DEEPSEEK_API_KEY" {
		t.Errorf("expected DEEPSEEK_API_KEY, got %s", EnvKeyName("deepseek"))
	}
}

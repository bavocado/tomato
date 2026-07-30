package llm

import (
	"strings"
	"testing"
)

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
		modelID string
		wantErr bool
	}{
		{"openai/gpt-5", false},
		{"glm/glm-5.2", false},
		{"deepseek/deepseek-4pro", false},
		{"anthropic/claude-sonnet-5", false},
		{"invalid", true},
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
		// Every provider now routes through the claude CLI.
		if _, ok := p.(*ClaudeCLIProvider); !ok {
			t.Errorf("expected %s to use ClaudeCLIProvider, got %T", tt.modelID, p)
		}
		if p.Model() != strings.SplitN(tt.modelID, "/", 2)[1] {
			t.Errorf("expected model %s, got %s", strings.SplitN(tt.modelID, "/", 2)[1], p.Model())
		}
	}
}

func TestNewProviderPassesProviderConnectionForCLI(t *testing.T) {
	// A provider configured with base_url + auth_token (e.g. an ai-router
	// fronting glm) still routes through the claude CLI, with the connection
	// injected via ANTHROPIC_BASE_URL/AUTH_TOKEN env. The model name comes from
	// the model ID's segment after the provider, so one provider serves many
	// models.
	p, err := NewProvider(ProviderConfig{
		ModelID:   "glm/glm:glm-5.2",
		BaseURL:   "http://127.0.0.1:1980",
		AuthToken: "sk-ai-router",
	})
	if err != nil {
		t.Fatal(err)
	}

	prov, ok := p.(*ClaudeCLIProvider)
	if !ok {
		t.Fatalf("expected ClaudeCLIProvider, got %T", p)
	}
	if prov.BaseURL != "http://127.0.0.1:1980" {
		t.Errorf("expected base url from config, got %s", prov.BaseURL)
	}
	if prov.AuthToken != "sk-ai-router" {
		t.Errorf("expected auth token from config, got %s", prov.AuthToken)
	}
	if prov.Model() != "glm:glm-5.2" {
		t.Errorf("expected model glm:glm-5.2 from id, got %s", prov.Model())
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

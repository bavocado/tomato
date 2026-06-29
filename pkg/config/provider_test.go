package config

import "testing"

func TestResolveProviderConfigFromProviders(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConnectionConfig{
			"glm": {
				BaseURL:   "https://glm.example.com",
				AuthToken: "glm-token",
				Model:     "glm-5.2",
			},
		},
	}

	p := cfg.ResolveProviderConfig("glm/glm-5.2")
	if p.BaseURL != "https://glm.example.com" {
		t.Errorf("expected glm provider base_url, got %s", p.BaseURL)
	}
	if p.AuthToken != "glm-token" {
		t.Errorf("expected glm provider auth token")
	}
	if p.Model != "glm-5.2" {
		t.Errorf("expected glm provider model, got %s", p.Model)
	}
}

func TestResolveProviderConfigFallsBackToAnthropic(t *testing.T) {
	cfg := &Config{
		Anthropic: AnthropicConfig{
			BaseURL:   "https://anthropic.example.com",
			AuthToken: "anthropic-token",
			Model:     "claude-test",
		},
	}

	p := cfg.ResolveProviderConfig("anthropic/claude-test")
	if p.BaseURL != "https://anthropic.example.com" {
		t.Errorf("expected legacy anthropic base_url, got %s", p.BaseURL)
	}
	if p.AuthToken != "anthropic-token" {
		t.Errorf("expected legacy anthropic auth token")
	}
	if p.Model != "claude-test" {
		t.Errorf("expected legacy anthropic model, got %s", p.Model)
	}
}

func TestDefaultConfigHasProviders(t *testing.T) {
	cfg := Default()
	if cfg.Providers == nil {
		t.Fatal("default config should include providers map")
	}
	if _, ok := cfg.Providers["glm"]; !ok {
		t.Error("default config should include glm provider")
	}
	if _, ok := cfg.Providers["deepseek"]; !ok {
		t.Error("default config should include deepseek provider")
	}
}
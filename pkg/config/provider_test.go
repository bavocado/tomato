package config

import "testing"

func TestResolveProviderConfigFromProviders(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConnectionConfig{
			"glm": {
				BaseURL:   "https://glm.example.com",
				AuthToken: "glm-token",
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
}

// TestResolveProviderConfigAnthropicFromProviders verifies anthropic is looked
// up as an ordinary provider — there is no longer a separate top-level
// anthropic field.
func TestResolveProviderConfigAnthropicFromProviders(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConnectionConfig{
			"anthropic": {
				BaseURL:   "https://anthropic.example.com",
				AuthToken: "anthropic-token",
			},
		},
	}

	p := cfg.ResolveProviderConfig("anthropic/claude-test")
	if p.BaseURL != "https://anthropic.example.com" {
		t.Errorf("expected anthropic provider base_url, got %s", p.BaseURL)
	}
	if p.AuthToken != "anthropic-token" {
		t.Errorf("expected anthropic provider auth token")
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
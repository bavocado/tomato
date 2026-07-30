package llm

import (
	"fmt"
	"strings"
)

// NewProvider creates a Provider from a ProviderConfig.
//
// All providers route through the claude CLI (ClaudeCLIProvider). ai-router
// fronts non-Anthropic models (glm/ark/deepseek) as Anthropic-protocol
// endpoints (/v1/messages), which the claude CLI speaks natively. Going through
// the CLI (rather than OpenAI-compatible HTTP) lets the CLI manage max_tokens
// and handle thinking blocks - so thinking models like ark's glm-5.2 don't get
// truncated to an empty response when the upstream's default max_tokens is
// consumed entirely by reasoning. A provider configured with base_url +
// auth_token is injected via ANTHROPIC_BASE_URL/AUTH_TOKEN env; one without
// uses the claude CLI's own defaults.
func NewProvider(cfg ProviderConfig) (Provider, error) {
	parts := strings.SplitN(cfg.ModelID, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid model format %q, expected provider/model", cfg.ModelID)
	}

	baseURL := firstNonEmpty(cfg.BaseURL, cfg.AnthropicURL)
	authToken := firstNonEmpty(cfg.AuthToken, cfg.AnthropicKey)
	// The model name always comes from the model ID's segment after the
	// provider ("/"). A provider carries no model of its own, so one provider
	// can serve many models (e.g. an ai-router fronting glm and ark).
	return NewClaudeCLIProvider(cfg.ModelID, baseURL, authToken, parts[1], cfg.SessionID, cfg.RepoDir)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// EnvKeyName returns the environment variable name for a provider's API key/token.
func EnvKeyName(provider string) string {
	if provider == "anthropic" {
		return "ANTHROPIC_AUTH_TOKEN"
	}
	return fmt.Sprintf("%s_API_KEY", strings.ToUpper(strings.ReplaceAll(provider, "-", "_")))
}

// ResolveModel picks the model for a step, falling back to the default.
func ResolveModel(stepName string, config map[string]string) string {
	if m, ok := config[stepName]; ok {
		return m
	}
	return config["default"]
}

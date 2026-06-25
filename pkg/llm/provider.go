package llm

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Provider is the interface for LLM providers.
type Provider interface {
	// Stream sends messages and calls onChunk for each token.
	Stream(messages []Message, onChunk func(string)) error
	// Model returns the model identifier (e.g., "gpt-5").
	Model() string
}

// ProviderConfig carries connection parameters for all supported providers.
type ProviderConfig struct {
	ModelID       string // e.g. "openai/gpt-5", "anthropic/claude-sonnet-4"
	APIKey        string // used by OpenAI-compatible providers (openai/glm/deepseek)
	AnthropicURL  string // ANTHROPIC_BASE_URL equivalent, from yaml
	AnthropicKey  string // ANTHROPIC_AUTH_TOKEN equivalent, from yaml
	AnthropicModel string // ANTHROPIC_MODEL equivalent, from yaml
}
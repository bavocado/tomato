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
	ModelID   string // e.g. "glm/glm-5.2", "deepseek/deepseek-v4-pro"
	APIKey    string // fallback for direct OpenAI-compatible HTTP providers
	BaseURL   string // passed to claude as ANTHROPIC_BASE_URL
	AuthToken string // passed to claude as ANTHROPIC_AUTH_TOKEN
	Model     string // passed to claude as ANTHROPIC_MODEL

	// SessionID, when non-empty, resumes an existing claude CLI session so
	// prior conversation context is reused across steps in one workflow run.
	SessionID string

	// RepoDir is the project root. When it contains a .codegraph/ index, the
	// claude CLI is launched with --mcp-config mounting the codegraph MCP
	// server so the LLM can query the code knowledge graph.
	RepoDir string

	// Deprecated: kept for older call sites during migration.
	AnthropicURL   string
	AnthropicKey   string
	AnthropicModel string
}

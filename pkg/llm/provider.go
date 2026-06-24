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
package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// CacheKey uniquely identifies a prompt invocation.
type CacheKey struct {
	TemplateVersion string
	PromptContent   string
	ModelID         string
	Params          string // e.g., "temperature=0.7,max_tokens=2000"
}

// Cache stores LLM responses on disk.
type Cache struct {
	dir string
	mu  sync.RWMutex
}

// NewCache creates a cache directory.
func NewCache(cacheDir string) (*Cache, error) {
	dir := filepath.Join(cacheDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}
	return &Cache{dir: dir}, nil
}

// keyPath returns the file path for a cache key.
func (c *Cache) keyPath(key CacheKey) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s", key.TemplateVersion, key.PromptContent, key.ModelID, key.Params)))
	return filepath.Join(c.dir, hex.EncodeToString(h[:]))
}

// Get returns the cached response and true, or empty string and false on miss.
func (c *Cache) Get(key CacheKey) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(c.keyPath(key))
	if err != nil {
		return "", false
	}

	var entry struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		return "", false
	}

	return entry.Response, true
}

// Set stores a response for a cache key.
func (c *Cache) Set(key CacheKey, response string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := struct {
		Response string `json:"response"`
	}{Response: response}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(c.keyPath(key), data, 0644)
}
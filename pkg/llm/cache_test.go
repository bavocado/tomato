package llm

import (
	"os"
	"testing"
)

func TestResponseCache(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	key := CacheKey{
		TemplateVersion: "v1",
		PromptContent:   "Write hello world in Go",
		ModelID:         "gpt-5",
		Params:          "temperature=0.7",
	}

	// Miss
	_, hit := cache.Get(key)
	if hit {
		t.Error("expected cache miss on first get")
	}

	// Set
	response := "package main\nfunc main() { println(\"hello\") }"
	if err := cache.Set(key, response); err != nil {
		t.Fatal(err)
	}

	// Hit
	val, hit := cache.Get(key)
	if !hit {
		t.Error("expected cache hit after set")
	}
	if val != response {
		t.Errorf("expected %q, got %q", response, val)
	}

	// Different params = different key (miss)
	key2 := CacheKey{
		TemplateVersion: "v1",
		PromptContent:   "Write hello world in Go",
		ModelID:         "gpt-5",
		Params:          "temperature=0.1",
	}
	_, hit = cache.Get(key2)
	if hit {
		t.Error("expected cache miss for different params")
	}
}

func TestCachePersistence(t *testing.T) {
	dir := t.TempDir()

	// Create cache, set a value, then close
	cache, err := NewCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	key := CacheKey{
		TemplateVersion: "v1",
		PromptContent:   "persist test",
		ModelID:         "gpt-5",
		Params:          "",
	}
	if err := cache.Set(key, "cached-response"); err != nil {
		t.Fatal(err)
	}

	// Create a new cache instance pointing to the same dir
	cache2, err := NewCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	val, hit := cache2.Get(key)
	if !hit {
		t.Error("expected cache hit from new instance")
	}
	if val != "cached-response" {
		t.Errorf("expected 'cached-response', got '%s'", val)
	}
}

func TestCacheDir(t *testing.T) {
	dir := t.TempDir()
	cacheDir := dir + "/subdir"
	cache, err := NewCache(cacheDir)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Error("cache directory should have been created")
	}

	_ = cache
}
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCommandShowsAnthropicYamlStatus(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// Isolate from ambient ANTHROPIC_* env so the yaml values are what render.
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	t.Setenv("ANTHROPIC_BASE_URL", "")
	t.Setenv("ANTHROPIC_MODEL", "")

	yaml := `
models:
  default: openai/gpt-5
workflows:
  default:
    steps: [spec]
anthropic:
  base_url: https://api.anthropic.com
  auth_token: sk-ant-abcdef123456
  model: claude-sonnet-4-20250514
`
	if err := os.WriteFile(filepath.Join(tempDir, "tomato.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	cmd := NewConfigCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Anthropic (legacy):") {
		t.Fatalf("expected Anthropic legacy section, got:\n%s", out)
	}
	if !strings.Contains(out, "base_url: ✓ https://api.anthropic.com") {
		t.Errorf("expected configured base_url, got:\n%s", out)
	}
	if !strings.Contains(out, "auth_token: ✓ configured (sk-ant-a..., from yaml)") {
		t.Errorf("expected masked auth_token from yaml, got:\n%s", out)
	}
	if strings.Contains(out, "sk-ant-abcdef123456") {
		t.Errorf("full token leaked in output:\n%s", out)
	}
	if !strings.Contains(out, "model: ✓ claude-sonnet-4-20250514") {
		t.Errorf("expected configured model, got:\n%s", out)
	}
}

func TestConfigCommandShowsMissingAnthropicYamlStatus(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// Isolate from ambient ANTHROPIC_* env.
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	t.Setenv("ANTHROPIC_BASE_URL", "")
	t.Setenv("ANTHROPIC_MODEL", "")

	yaml := `
models:
  default: openai/gpt-5
workflows:
  default:
    steps: [spec]
anthropic:
  base_url: ""
  auth_token: ""
  model: ""
`
	if err := os.WriteFile(filepath.Join(tempDir, "tomato.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	cmd := NewConfigCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Anthropic (legacy):") {
		t.Fatalf("expected Anthropic legacy section, got:\n%s", out)
	}
	if !strings.Contains(out, "base_url: ✗ not set") {
		t.Errorf("expected missing base_url, got:\n%s", out)
	}
	if !strings.Contains(out, "auth_token: ✗ not set") {
		t.Errorf("expected missing auth_token, got:\n%s", out)
	}
	if !strings.Contains(out, "model: ✗ not set") {
		t.Errorf("expected missing model, got:\n%s", out)
	}
}

func TestConfigCommandShowsProvidersStatus(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	yaml := `
models:
  default: glm/glm-5.2
workflows:
  default:
    steps: [spec]
providers:
  glm:
    base_url: https://glm.example.com
    auth_token: glm-secret-token
    model: glm-5.2
  deepseek:
    base_url: https://deepseek.example.com
    auth_token: deepseek-secret-token
    model: deepseek-v4-pro
`
	if err := os.WriteFile(filepath.Join(tempDir, "tomato.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	cmd := NewConfigCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Providers:") {
		t.Fatalf("expected Providers section, got:\n%s", out)
	}
	if !strings.Contains(out, "glm:") || !strings.Contains(out, "deepseek:") {
		t.Errorf("expected glm and deepseek provider sections, got:\n%s", out)
	}
	if !strings.Contains(out, "auth_token: ✓ configured (glm-secr..., from yaml)") {
		t.Errorf("expected masked glm auth token, got:\n%s", out)
	}
	if strings.Contains(out, "glm-secret-token") || strings.Contains(out, "deepseek-secret-token") {
		t.Errorf("provider token leaked in output:\n%s", out)
	}
}

func TestMaskSecretShortToken(t *testing.T) {
	masked := maskSecret("abc")
	if masked != "abc..." {
		t.Errorf("expected abc..., got %q", masked)
	}
}

func TestMaskSecretLongToken(t *testing.T) {
	masked := maskSecret("sk-ant-abcdef123456")
	if masked != "sk-ant-a..." {
		t.Errorf("expected sk-ant-a..., got %q", masked)
	}
}

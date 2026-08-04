package compress

import (
	"strings"
	"testing"
)

func TestRTKToolOutputTrimsLongSuccessfulOutput(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 160; i++ {
		b.WriteString("ok line\n")
	}
	b.WriteString("final line\n")

	res := RTKToolOutput("Bash", b.String(), false)
	if !res.Truncated {
		t.Fatal("expected long output to be truncated")
	}
	if strings.Count(res.Text, "ok line") > 20 {
		t.Fatalf("expected repeated success output to be folded, got:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "final line") {
		t.Fatalf("expected tail to be preserved, got:\n%s", res.Text)
	}
	if res.RawBytes <= res.KeptBytes {
		t.Fatalf("expected compression, raw=%d kept=%d", res.RawBytes, res.KeptBytes)
	}
}

func TestRTKToolOutputKeepsErrorsAndPaths(t *testing.T) {
	raw := strings.Join([]string{
		"setup",
		"FAIL: TestAuth",
		"/tmp/project/internal/auth/auth_test.go:42: expected encrypted key",
		"extra",
	}, "\n")

	res := RTKToolOutput("Bash", raw, true)
	for _, want := range []string{"FAIL: TestAuth", "/tmp/project/internal/auth/auth_test.go:42"} {
		if !strings.Contains(res.Text, want) {
			t.Fatalf("expected %q in compressed output:\n%s", want, res.Text)
		}
	}
}

func TestRTKToolOutputLimitsHugeSingleLine(t *testing.T) {
	res := RTKToolOutput("Bash", strings.Repeat("x", 20000)+"tail", false)
	if !res.Truncated {
		t.Fatal("expected huge single line to be truncated")
	}
	if !strings.Contains(res.Text, "output truncated") || !strings.Contains(res.Text, "tail") {
		t.Fatalf("expected truncation marker and tail, got:\n%s", res.Text)
	}
}

func TestCavemanDropsFillerButPreservesCodeBlocks(t *testing.T) {
	raw := strings.Join([]string{
		"I inspected the repository and found that the fast workflow is currently using the normal cache path, which means it can skip Claude.",
		"",
		"```go",
		"func main() {",
		"    println(\"keep me\")",
		"}",
		"```",
		"",
		"I think the next action is to run go test ./... and commit the result.",
	}, "\n")

	res := Caveman(raw)
	if strings.Contains(res.Text, "I inspected the repository and found that") {
		t.Fatalf("expected filler phrase to be trimmed:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "```go\nfunc main()") {
		t.Fatalf("expected code block to be preserved:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "go test ./...") {
		t.Fatalf("expected test command to be preserved:\n%s", res.Text)
	}
}

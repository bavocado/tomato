package compress

import (
	"strconv"
	"strings"
)

const maxRTKLines = 80
const maxRTKBytes = 12 * 1024

type Result struct {
	Text      string
	RawBytes  int
	KeptBytes int
	Truncated bool
}

func RTKToolOutput(tool, text string, isError bool) Result {
	rawBytes := len(text)
	lines := foldRepeats(strings.Split(text, "\n"))
	if isError {
		out := limitBytes(strings.Join(keepErrorLines(lines), "\n"))
		return Result{Text: out, RawBytes: rawBytes, KeptBytes: len(out), Truncated: len(out) < rawBytes}
	}
	if len(lines) > maxRTKLines {
		lines = append(append([]string{}, lines[:30]...), append([]string{"... output truncated ..."}, lines[len(lines)-30:]...)...)
	}
	out := limitBytes(strings.Join(lines, "\n"))
	return Result{Text: out, RawBytes: rawBytes, KeptBytes: len(out), Truncated: len(out) < rawBytes}
}

func Caveman(text string) Result {
	rawBytes := len(text)
	lines := strings.Split(text, "\n")
	var out []string
	inCode := false
	lastBlank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			out = append(out, line)
			lastBlank = false
			continue
		}
		if inCode {
			out = append(out, line)
			continue
		}
		if trimmed == "" {
			if !lastBlank && len(out) > 0 {
				out = append(out, "")
				lastBlank = true
			}
			continue
		}
		trimmed = cavemanLine(trimmed)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
		lastBlank = false
	}
	result := strings.TrimSpace(strings.Join(out, "\n"))
	return Result{Text: result, RawBytes: rawBytes, KeptBytes: len(result), Truncated: len(result) < rawBytes}
}

func foldRepeats(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		j := i + 1
		for j < len(lines) && lines[j] == lines[i] {
			j++
		}
		out = append(out, lines[i])
		if n := j - i; n > 1 {
			out = append(out, "... repeated "+strconv.Itoa(n-1)+" more times ...")
		}
		i = j
	}
	return out
}

func keepErrorLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	last := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "exit status") || strings.Contains(lower, "exit code") || looksPathy(line) {
			start := max(0, i-1)
			end := min(len(lines), i+2)
			if start <= last {
				start = last + 1
			}
			out = append(out, lines[start:end]...)
			last = end - 1
		}
	}
	if len(out) == 0 && len(lines) > 0 {
		out = lines[:min(len(lines), 20)]
	}
	return foldRepeats(out)
}

func cavemanLine(s string) string {
	for _, prefix := range []string{
		"I inspected the repository and found that ",
		"I think ",
		"I found that ",
		"Currently, ",
	} {
		s = strings.TrimPrefix(s, prefix)
	}
	if idx := strings.Index(s, " which means "); idx >= 0 {
		s = s[:idx] + "."
	}
	return strings.TrimSpace(s)
}

func looksPathy(s string) bool {
	return strings.Contains(s, "/") && strings.Contains(s, ".")
}

func limitBytes(s string) string {
	if len(s) <= maxRTKBytes {
		return s
	}
	r := []rune(s)
	keep := min(len(r), maxRTKBytes/2)
	return string(r[:keep]) + "\n... output truncated ...\n" + string(r[len(r)-keep:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

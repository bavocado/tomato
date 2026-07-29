package decompose

import (
	"os"
	"path/filepath"
	"strings"
)

// ParentFeatureFromDesign derives the parent feature name from a design
// document's first "# " heading. Sub-feature dirs become <parent>-f001, so the
// parent name must reflect the design itself, not the git branch tomato happens
// to be running on.
//
// The slug is ASCII-only: only [a-z0-9._-] survive, everything else (spaces,
// punctuation, CJK) is dropped and adjacent runs collapse to a single "-".
// So "# grape 平台设计文档" → "grape". A pure-ASCII name keeps dirs portable
// across shells and tools; headings that contain no ASCII at all fall back to
// "source". Trailing boilerplate suffixes (design/spec) are trimmed first.
func ParentFeatureFromDesign(design string) string {
	title := firstH1(design)
	title = trimBoilerplate(title)
	slug := slugify(title)
	if slug == "" {
		return "source"
	}
	return slug
}

// firstH1 returns the text of the first "# " line, trimmed, with the leading
// "#" and surrounding spaces removed. Markdown levels ("## ") are ignored.
func firstH1(md string) string {
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		}
	}
	return ""
}

// trimBoilerplate drops trailing suffixes that carry no feature meaning so the
// dir name stays compact (e.g. "grape 平台设计文档" → "grape 平台").
var boilerplateSuffixes = []string{
	"设计文档", "设计方案", "设计", "规格文档", "规格", "需求文档", "需求",
	"design document", "design doc", "design", "specification", "spec",
}

func trimBoilerplate(s string) string {
	trimmed := strings.TrimSpace(s)
	lower := strings.ToLower(trimmed)
	for _, suf := range boilerplateSuffixes {
		if strings.HasSuffix(lower, suf) {
			return strings.TrimSpace(trimmed[:len(trimmed)-len(suf)])
		}
	}
	return trimmed
}

// slugify produces an ASCII-only slug: [a-z0-9._-] survive (letters lowercased),
// every other rune (spaces, punctuation, CJK) is dropped and adjacent drops
// collapse to a single "-". Leading/trailing "-" are stripped.
func slugify(s string) string {
	var b strings.Builder
	prevSep := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '.', c == '_', c == '-':
			b.WriteByte(c)
			prevSep = false
		case c >= 'A' && c <= 'Z':
			b.WriteByte(c + 'a' - 'A')
			prevSep = false
		default:
			// Non-ASCII (multi-byte runes land here byte-by-byte) and ASCII
			// separators both collapse to a single "-".
			if !prevSep {
				b.WriteByte('-')
				prevSep = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// FindDecomposeParent scans <specsDir>/*/decomposition.md and returns the parent
// feature names (the immediate sub-directory names) that already hold a
// decomposition. apply uses this to locate a prior generate run regardless of
// the parent name the design title would currently derive — breaking the cycle
// where generate writes under a derived name but apply starts from the
// branch-inferred name and cannot find it.
func FindDecomposeParent(specsDir string) []string {
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil
	}
	var parents []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(specsDir, e.Name(), "decomposition.md")); err == nil {
			parents = append(parents, e.Name())
		}
	}
	return parents
}

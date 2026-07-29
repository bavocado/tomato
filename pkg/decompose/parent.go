package decompose

import (
	"strings"
	"unicode"
)

// ParentFeatureFromDesign derives the parent feature name from a design
// document's first "# " heading. Sub-feature dirs become <parent>-f001, so the
// parent name must reflect the design itself, not the git branch tomato happens
// to be running on.
//
// The first top-level heading is used (e.g. "# grape 平台设计文档" →
// "grape-平台设计文档"). Trailing boilerplate suffixes like "设计文档"/
// "design"/"spec" are dropped. Whitespace and punctuation collapse to a single
// "-", and CJK characters are kept (a pure-ASCII slug would erase Chinese
// titles). Empty headings fall back to "source".
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

// slugify keeps letters (incl. CJK), digits, ".", "_", "-"; every other run
// (spaces, punctuation) becomes a single "-", and leading/trailing "-" are
// stripped. CJK runes pass unicode.IsLetter, so a Chinese title survives.
func slugify(s string) string {
	var b strings.Builder
	prevSep := true
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-':
			// Lowercase ASCII letters for tidy dir names; CJK/other letters
			// are unchanged (unicode.ToLower is a no-op for them).
			if r >= 'A' && r <= 'Z' {
				r = r - 'A' + 'a'
			}
			b.WriteRune(r)
			prevSep = false
		default:
			if !prevSep {
				b.WriteByte('-')
				prevSep = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

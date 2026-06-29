package steps

import (
	"path/filepath"
	"regexp"
	"strings"
)

var featureSanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// ResolveFeature determines the feature name that groups artifacts under
// docs/specs/<feature>/ (design §2.6). Precedence:
//  1. an explicit value (e.g. the --feature flag), sanitized
//  2. the configured value (tomato.yaml `feature:`), sanitized
//  3. the current git branch's last path segment (feature/login → login)
//  4. "current-feature" (fallback: nothing set, detached HEAD, or no repo)
func ResolveFeature(explicit, configured, repoDir string) string {
	if f := sanitizeFeature(explicit); f != "" {
		return f
	}
	if f := sanitizeFeature(configured); f != "" {
		return f
	}
	branch := getCurrentBranch(repoDir)
	if branch != "" && branch != "HEAD" {
		seg := branch
		if i := strings.LastIndex(branch, "/"); i >= 0 {
			seg = branch[i+1:]
		}
		if f := sanitizeFeature(seg); f != "" {
			return f
		}
	}
	return "current-feature"
}

// sanitizeFeature reduces an arbitrary string to a safe path segment.
func sanitizeFeature(s string) string {
	s = strings.TrimSpace(s)
	s = featureSanitizeRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// FeatureDir returns the artifact directory for a feature under repoDir.
func FeatureDir(repoDir, feature string) string {
	return filepath.Join(repoDir, "docs", "specs", feature)
}

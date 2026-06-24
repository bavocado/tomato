package runid

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Generate creates a short human-readable run ID from UUID prefix.
func Generate() string {
	short := uuid.New().String()[:8]
	date := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("%s-%s", date, short)
}

// RunDir returns the filesystem path for a run's data directory.
func RunDir(baseDir, runID string) string {
	return fmt.Sprintf("%s/runs/%s", baseDir, runID)
}

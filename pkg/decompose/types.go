// Package decompose parses a decomposition.md manifest, validates it against
// the Definition-of-Ready, orders sub-features by dependency/priority, and
// materializes them as independent feature directories.
package decompose

// Decomposition is the parsed authoritative yaml manifest from decomposition.md.
type Decomposition struct {
	Source        string       `yaml:"source"`
	TotalFeatures int          `yaml:"total_features"`
	DagCheck      string       `yaml:"dag_check"`
	CriticalPath  []string     `yaml:"critical_path"`
	Spikes        []string     `yaml:"spikes"`
	Features      []SubFeature `yaml:"features"`
}

// SubFeature is one independently-implementable slice of the parent design.
type SubFeature struct {
	ID                 string   `yaml:"id"`
	Title              string   `yaml:"title"`
	Goal               string   `yaml:"goal"`
	UserValue          string   `yaml:"user_value"`
	SliceType          string   `yaml:"slice_type"`
	C4Level            string   `yaml:"c4_level"`
	Priority           string   `yaml:"priority"`
	DependsOn          []string `yaml:"depends_on"`
	AcceptanceCriteria []string `yaml:"acceptance_criteria"`
	Complexity         string   `yaml:"complexity"`
	IsSpike            bool     `yaml:"is_spike"`
	Timebox            string   `yaml:"timebox,omitempty"`
	OutOfScope         []string `yaml:"out_of_scope"`
	OpenQuestions      []string `yaml:"open_questions,omitempty"`
}
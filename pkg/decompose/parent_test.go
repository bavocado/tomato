package decompose

import "testing"

func TestParentFeatureFromDesign(t *testing.T) {
	tests := []struct {
		name   string
		design string
		want   string
	}{
		{
			name:   "mixed cjk+ascii keeps ascii drops cjk",
			design: "# grape 平台设计文档\n\n正文",
			want:   "grape",
		},
		{
			name:   "english design doc suffix",
			design: "# User Auth Design Document\nbody",
			want:   "user-auth",
		},
		{
			name:   "plain title no boilerplate",
			design: "# Payment Gateway\n\n## details",
			want:   "payment-gateway",
		},
		{
			name:   "ignores h2 picks first h1",
			design: "## not this\n\n# Real Title\nbody",
			want:   "real-title",
		},
		{
			name:   "empty heading falls back",
			design: "#\nbody",
			want:   "source",
		},
		{
			name:   "no heading at all falls back",
			design: "just body, no heading",
			want:   "source",
		},
		{
			name:   "pure cjk no ascii falls back",
			design: "# 平台设计文档\nbody",
			want:   "source",
		},
		{
			name:   "punctuation collapses to single dash",
			design: "# Foo!!!  Bar??? Baz",
			want:   "foo-bar-baz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParentFeatureFromDesign(tt.design); got != tt.want {
				t.Errorf("ParentFeatureFromDesign() = %q, want %q", got, tt.want)
			}
		})
	}
}

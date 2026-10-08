package judge

import "testing"

// Every fixture compared by the check engine needs a writer so a deliberate
// output change cannot leave one of its byte locks behind.
func TestRegenerateGoldensCoversCheckConsumers(t *testing.T) {
	t.Parallel()
	writers := make(map[string]bool)
	for _, tt := range engineGoldens {
		writers[tt.golden] = true
	}
	for _, tt := range schemaGoldens {
		writers[tt.golden] = true
	}
	for _, tt := range coverageGoldens {
		writers[tt.golden] = true
	}
	for _, tt := range checkGoldens {
		if !writers[tt.golden] {
			t.Errorf("compared golden has no regeneration writer: %s", tt.golden)
		}
	}
}

package judge

import (
	"bytes"
	"os"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestRegenerateGoldens rewrites every JSONL golden from the current engine
// output, mirroring exactly how each golden's own test produces its bytes. It
// is an opt-in maintenance tool, not a check: without the environment guard it
// skips, so an ordinary test run can never rewrite the locks it runs against.
func TestRegenerateGoldens(t *testing.T) {
	if os.Getenv("YOMIHON_REGEN_GOLDENS") == "" {
		t.Skip("set YOMIHON_REGEN_GOLDENS=1 to rewrite the JSONL goldens")
	}

	engine := []struct {
		fixture string
		golden  string
		private []string
	}{
		{fixture: "testdata/vault", golden: "testdata/golden/check.jsonl"},
		{fixture: "testdata/vault-maps", golden: "testdata/golden/maps.jsonl"},
		{fixture: "testdata/vault-escapes", golden: "testdata/golden/escapes.jsonl"},
		{fixture: "testdata/vault-lines", golden: "testdata/golden/lines.jsonl"},
		{fixture: "testdata/vault-planned", golden: "testdata/golden/planned.jsonl"},
		{fixture: "testdata/vault-diskref", golden: "testdata/golden/diskref.jsonl"},
		{fixture: "testdata/vault-scope", golden: "testdata/golden/scope.jsonl"},
		{fixture: "testdata/vault-mapmismatch", golden: "testdata/golden/mapmismatch.jsonl"},
		{fixture: "testdata/vault-paths", golden: "testdata/golden/paths.jsonl"},
		{fixture: "testdata/vault-course", golden: "testdata/golden/course.jsonl"},
		{fixture: "testdata/vault-edges", golden: "testdata/golden/edges.jsonl"},
		{fixture: "testdata/vault-fragments", golden: "testdata/golden/fragments.jsonl"},
		{fixture: "testdata/vault-knowledge-scope", golden: "testdata/golden/knowledge-scope.jsonl"},
		{fixture: "testdata/vault-namecollision", golden: "testdata/golden/namecollision.jsonl"},
		{fixture: "testdata/vault-report", golden: "testdata/golden/report.jsonl"},
		{fixture: "testdata/vault-supersession", golden: "testdata/golden/supersession.jsonl"},
		{fixture: "testdata/vault-comment-scope", golden: "testdata/golden/comment-scope.jsonl"},
		{fixture: "testdata/vault-namecollision-privacy", golden: "testdata/golden/namecollision-privacy.jsonl", private: []string{"Private"}},
		{fixture: "testdata/vault-diary", golden: "testdata/golden/diary.jsonl", private: []string{"Diary"}},
		{fixture: "testdata/vault-titlecollision", golden: "testdata/golden/titlecollision.jsonl"},
		{fixture: "testdata/vault-titlecollision", golden: "testdata/golden/titlecollision-privacy.jsonl", private: []string{"Private"}},
	}
	for _, tt := range engine {
		root := judgeFixtureRootWithPrivacy(t, tt.fixture, tt.private...)
		findings, err := Check(t.Context(), root)
		if err != nil {
			t.Fatalf("Check(%q): %v", tt.fixture, err)
		}
		var buf bytes.Buffer
		if err := WriteJSONL(&buf, findings); err != nil {
			t.Fatalf("WriteJSONL(%q): %v", tt.fixture, err)
		}
		if err := os.WriteFile(tt.golden, buf.Bytes(), 0o600); err != nil {
			t.Fatalf("write %s: %v", tt.golden, err)
		}
		t.Logf("rewrote %s (%d bytes)", tt.golden, buf.Len())
	}

	schemaOnly := []struct {
		fixture string
		golden  string
	}{
		{fixture: "testdata/vault-schema", golden: "testdata/golden/schema.jsonl"},
		{fixture: "testdata/vault-coercion", golden: "testdata/golden/coercion.jsonl"},
		{fixture: "testdata/vault-strictness", golden: "testdata/golden/strictness.jsonl"},
	}
	for _, tt := range schemaOnly {
		notes, err := collectNotes(t.Context(), tt.fixture)
		if err != nil {
			t.Fatalf("collectNotes(%q): %v", tt.fixture, err)
		}
		s, err := schema.Load(tt.fixture)
		if err != nil {
			t.Fatalf("schema.Load(%q): %v", tt.fixture, err)
		}
		findings, err := checkSchema(notes, s)
		if err != nil {
			t.Fatalf("checkSchema(%q): %v", tt.fixture, err)
		}
		sortFindings(findings)
		var buf bytes.Buffer
		if err := WriteJSONL(&buf, findings); err != nil {
			t.Fatalf("WriteJSONL(%q): %v", tt.fixture, err)
		}
		if err := os.WriteFile(tt.golden, buf.Bytes(), 0o600); err != nil {
			t.Fatalf("write %s: %v", tt.golden, err)
		}
		t.Logf("rewrote %s (%d bytes)", tt.golden, buf.Len())
	}
}

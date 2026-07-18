package evalset

import (
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoadSuiteHasFortyCasesAcrossRequiredClasses(t *testing.T) {
	t.Parallel()

	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	if suite.FormatVersion != 1 ||
		suite.SyntheticVaultRoot != syntheticVaultRoot ||
		suite.ChunkTokenCap != syntheticChunkTokenCap {
		t.Errorf(
			"LoadSuite() metadata = (%d, %q, %d), want (1, %q, %d)",
			suite.FormatVersion,
			suite.SyntheticVaultRoot,
			suite.ChunkTokenCap,
			syntheticVaultRoot,
			syntheticChunkTokenCap,
		)
	}
	got := make(map[queryClass]int)
	for _, testCase := range suite.Cases {
		got[testCase.Class]++
	}
	want := map[queryClass]int{
		classTraditionalChineseJapanese: 8,
		classJapaneseTerm:               8,
		classCodeTerm:                   8,
		classCrossLingualParaphrase:     8,
		classFilterMixed:                8,
	}
	if len(suite.Cases) != 40 {
		t.Errorf("LoadSuite() case count = %d, want 40", len(suite.Cases))
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("LoadSuite() class counts mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateSuiteRejectsDuplicateCaseID(t *testing.T) {
	t.Parallel()

	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	suite.Cases[1].ID = suite.Cases[0].ID
	if err := validateSuite(&suite); !errors.Is(err, errInvalidSuite) {
		t.Errorf("validateSuite(duplicate ID) error = %v, want errInvalidSuite", err)
	}
}

func TestValidateSuiteRejectsMalformedCorpusAndCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*evalSuite)
	}{
		{name: "unmarked origin", mutate: func(s *evalSuite) { s.Cases[0].Origin = "local" }},
		{name: "non CJK query", mutate: func(s *evalSuite) { s.Cases[0].Query = "only ascii" }},
		{name: "wrong denominator", mutate: func(s *evalSuite) { s.Cases[0].RecallDenominator = 2 }},
		{name: "no required positive", mutate: func(s *evalSuite) { s.Cases[0].RequiredPositives = nil }},
		{name: "positive also contrastive", mutate: func(s *evalSuite) {
			s.Cases[0].ContrastiveSiblings = []string{s.Cases[0].RequiredPositives[0]}
		}},
		{name: "unknown tie rule", mutate: func(s *evalSuite) { s.Cases[0].Rank5TieRule = "provider-order" }},
		{name: "unknown class", mutate: func(s *evalSuite) { s.Cases[0].Class = "other" }},
		{name: "mixed query without filters", mutate: func(s *evalSuite) { s.Cases[32].Filters = nil }},
		{name: "unknown filter", mutate: func(s *evalSuite) { s.Cases[32].Filters[0].Key = "source_kind" }},
		{name: "positive outside corpus", mutate: func(s *evalSuite) {
			s.Cases[0].RequiredPositives[0] = "Writing/lessons/japanese/not-in-corpus.md"
		}},
		{name: "acceptable positive outside corpus", mutate: func(s *evalSuite) {
			s.Cases[0].AcceptablePositives = []string{"Writing/lessons/japanese/not-in-corpus.md"}
		}},
		{name: "acceptable positive also required", mutate: func(s *evalSuite) {
			s.Cases[0].AcceptablePositives = []string{s.Cases[0].RequiredPositives[0]}
		}},
		{name: "acceptable positive also contrastive", mutate: func(s *evalSuite) {
			s.Cases[0].AcceptablePositives = []string{s.Cases[0].ContrastiveSiblings[0]}
		}},
		{name: "private corpus path", mutate: func(s *evalSuite) {
			s.Corpus[0].RelPath = "Diary/__synthetic_eval_private.md"
		}},
		{name: "unmarked corpus path", mutate: func(s *evalSuite) {
			s.Corpus[0].RelPath = "Writing/lessons/japanese/copied-note.md"
		}},
		{name: "positive excluded by allowed paths", mutate: func(s *evalSuite) {
			s.Cases[32].AllowedPaths = []string{s.Cases[32].ContrastiveSiblings[0]}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			suite, err := loadSuite()
			if err != nil {
				t.Fatal(err)
			}
			tt.mutate(&suite)
			if err := validateSuite(&suite); !errors.Is(err, errInvalidSuite) {
				t.Errorf("validateSuite(%s) error = %v, want errInvalidSuite", tt.name, err)
			}
		})
	}
}

func TestLoadSuiteReturnsOnlyMarkedSyntheticCorpusPaths(t *testing.T) {
	t.Parallel()

	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Corpus) != 16 {
		t.Fatalf("LoadSuite() corpus count = %d, want 16", len(suite.Corpus))
	}
	for _, note := range suite.Corpus {
		base := path.Base(note.RelPath)
		if !strings.HasPrefix(base, "__synthetic_eval_") || !strings.HasSuffix(base, ".md") {
			t.Errorf("LoadSuite() corpus path = %q, want marked synthetic Markdown path", note.RelPath)
		}
		if strings.Contains(note.RelPath, "Diary/") {
			t.Errorf("LoadSuite() corpus path = %q, want non-private synthetic path", note.RelPath)
		}
	}
}

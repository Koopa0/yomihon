package evalset

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/search/evalset/recording"
	"github.com/koopa0/yomihon/internal/vault"
)

// errInvalidSuite means the committed synthetic corpus/query contract is not
// internally self-consistent and must not be evaluated.
var errInvalidSuite = errors.New("semantic eval suite is invalid")

// queryClass names one required retrieval shape in the committed suite.
type queryClass string

const (
	classTraditionalChineseJapanese queryClass = "traditional-chinese-japanese"
	classJapaneseTerm               queryClass = "japanese-term"
	classCodeTerm                   queryClass = "code-term"
	classCrossLingualParaphrase     queryClass = "cross-lingual-paraphrase"
	classFilterMixed                queryClass = "filter-mixed"
)

const (
	syntheticOrigin        = "generated-synthetic-v1"
	syntheticVaultRoot     = "/yomihon-synthetic-semantic-eval-v1"
	syntheticChunkTokenCap = 1024
	rank5TieRule           = "score-desc-path-asc"
)

// filter is one Part I structured constraint recorded beside a mixed query.
type filter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// evalCase is one retrieval oracle. RecallDenominator is explicit rather than
// inferred so a fixture edit cannot silently change the reported metric.
type evalCase struct {
	ID                  string     `json:"id"`
	Origin              string     `json:"origin"`
	Class               queryClass `json:"class"`
	Query               string     `json:"query"`
	Filters             []filter   `json:"filters"`
	AllowedPaths        []string   `json:"allowed_paths"`
	RequiredPositives   []string   `json:"required_positives"`
	AcceptablePositives []string   `json:"acceptable_positives,omitempty"`
	ContrastiveSiblings []string   `json:"contrastive_siblings"`
	Rank5TieRule        string     `json:"rank_5_tie_rule"`
	RecallDenominator   int        `json:"recall_denominator"`
}

// evalSuite is the immutable committed corpus/query contract.
type evalSuite struct {
	FormatVersion      int          `json:"format_version"`
	SyntheticVaultRoot string       `json:"synthetic_vault_root"`
	ChunkTokenCap      int          `json:"chunk_token_cap"`
	Corpus             []corpusNote `json:"-"`
	Cases              []evalCase   `json:"cases"`
}

// corpusNote is one owned synthetic Markdown source in vault-relative form.
type corpusNote struct {
	RelPath string
	Data    []byte
}

//go:embed testdata/cases.json
var casesJSON []byte

//go:embed all:testdata/vault
var corpusFS embed.FS

// loadSuite decodes the committed synthetic query oracle.
func loadSuite() (evalSuite, error) {
	suite, err := decodeSuite(casesJSON)
	if err != nil {
		return evalSuite{}, err
	}
	if err := fs.WalkDir(corpusFS, "testdata/vault", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !vault.IsMarkdown(name) {
			return nil
		}
		data, err := corpusFS.ReadFile(name)
		if err != nil {
			return err
		}
		relPath := strings.TrimPrefix(name, "testdata/vault/")
		suite.Corpus = append(suite.Corpus, corpusNote{RelPath: relPath, Data: data})
		return nil
	}); err != nil {
		return evalSuite{}, fmt.Errorf("read semantic eval corpus: %w", err)
	}
	if err := validateSuite(&suite); err != nil {
		return evalSuite{}, err
	}
	return suite, nil
}

func decodeSuite(data []byte) (evalSuite, error) {
	var suite evalSuite
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&suite); err != nil {
		return evalSuite{}, fmt.Errorf("%w: decode query cases: %w", errInvalidSuite, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return evalSuite{}, fmt.Errorf("%w: trailing query-case JSON value", errInvalidSuite)
	}
	return suite, nil
}

func validateSuite(suite *evalSuite) error {
	if suite == nil || len(suite.Cases) != 40 {
		return fmt.Errorf("%w: expected exactly 40 cases", errInvalidSuite)
	}
	if suite.FormatVersion != 1 ||
		suite.SyntheticVaultRoot != syntheticVaultRoot ||
		suite.ChunkTokenCap != syntheticChunkTokenCap {
		return fmt.Errorf("%w: recording metadata changed", errInvalidSuite)
	}
	corpus, err := validateCorpus(suite.Corpus)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(suite.Cases))
	classCounts := make(map[queryClass]int)
	for position := range suite.Cases {
		testCase := &suite.Cases[position]
		wantID := fmt.Sprintf("q%02d", position+1)
		if testCase.ID != wantID {
			return fmt.Errorf("%w: case %d ID is not %s", errInvalidSuite, position+1, wantID)
		}
		if _, exists := seen[testCase.ID]; exists {
			return fmt.Errorf("%w: duplicate case ID", errInvalidSuite)
		}
		seen[testCase.ID] = struct{}{}
		if err := validateCase(testCase, corpus); err != nil {
			return fmt.Errorf("%w: case %s: %w", errInvalidSuite, testCase.ID, err)
		}
		classCounts[testCase.Class]++
	}
	for _, class := range []queryClass{
		classTraditionalChineseJapanese,
		classJapaneseTerm,
		classCodeTerm,
		classCrossLingualParaphrase,
		classFilterMixed,
	} {
		if classCounts[class] != 8 {
			return fmt.Errorf("%w: class %s has %d cases, want 8", errInvalidSuite, class, classCounts[class])
		}
	}
	return nil
}

func validateCorpus(notes []corpusNote) (map[string]struct{}, error) {
	if len(notes) == 0 {
		return nil, fmt.Errorf("%w: synthetic corpus is empty", errInvalidSuite)
	}
	paths := make(map[string]struct{}, len(notes))
	for _, note := range notes {
		if !recording.IsSyntheticPath(note.RelPath) {
			return nil, fmt.Errorf("%w: corpus path is not marked synthetic", errInvalidSuite)
		}
		if !utf8.Valid(note.Data) || !strings.Contains(string(note.Data), "title: 合成評測：") {
			return nil, fmt.Errorf("%w: corpus note lacks the synthetic content marker", errInvalidSuite)
		}
		if _, exists := paths[note.RelPath]; exists {
			return nil, fmt.Errorf("%w: duplicate corpus path", errInvalidSuite)
		}
		paths[note.RelPath] = struct{}{}
	}
	return paths, nil
}

func validateCase(testCase *evalCase, corpus map[string]struct{}) error {
	if testCase.Origin != syntheticOrigin {
		return errors.New("origin is not the synthetic marker")
	}
	if !knownClass(testCase.Class) {
		return errors.New("query class is unknown")
	}
	if cjkRunes(testCase.Query) < 2 || leaksVaultPath(testCase.Query) {
		return errors.New("query is not CJK-heavy synthetic text")
	}
	if testCase.Rank5TieRule != rank5TieRule {
		return errors.New("rank-5 tie rule is not deterministic score/path order")
	}
	if len(testCase.RequiredPositives) == 0 || testCase.RecallDenominator != len(testCase.RequiredPositives) {
		return errors.New("recall denominator does not name every required positive")
	}
	if len(testCase.ContrastiveSiblings) == 0 {
		return errors.New("contrastive siblings are empty")
	}
	if err := validateFilters(testCase); err != nil {
		return err
	}
	if err := validateOraclePaths(testCase, corpus); err != nil {
		return err
	}
	return nil
}

func validateFilters(testCase *evalCase) error {
	if testCase.Class == classFilterMixed {
		if len(testCase.Filters) == 0 || len(testCase.AllowedPaths) == 0 {
			return errors.New("filter-mixed query lacks filters or allowed paths")
		}
	} else if len(testCase.Filters) != 0 || len(testCase.AllowedPaths) != 0 {
		return errors.New("non-filter query carries filter state")
	}
	known := map[string]struct{}{
		"type": {}, "status": {}, "domain": {}, "topic": {}, "slug": {}, "folder": {},
	}
	for _, filter := range testCase.Filters {
		if _, ok := known[filter.Key]; !ok || filter.Value == "" {
			return errors.New("filter is outside the Part I grammar")
		}
		if !strings.Contains(testCase.Query, filter.Key+":"+filter.Value) {
			return errors.New("recorded filter does not occur in the raw query")
		}
	}
	return nil
}

func validateOraclePaths(testCase *evalCase, corpus map[string]struct{}) error {
	positive, err := validatePathSet(testCase.RequiredPositives, corpus, nil, "required positive")
	if err != nil {
		return err
	}
	acceptable, err := validatePathSet(testCase.AcceptablePositives, corpus, positive, "acceptable positive")
	if err != nil {
		return err
	}
	maps.Copy(positive, acceptable)
	if _, contrastErr := validatePathSet(testCase.ContrastiveSiblings, corpus, positive, "contrastive sibling"); contrastErr != nil {
		return contrastErr
	}
	if testCase.Class != classFilterMixed {
		return nil
	}
	allowed, err := validatePathSet(testCase.AllowedPaths, corpus, nil, "allowed path")
	if err != nil {
		return err
	}
	for relPath := range positive {
		if _, ok := allowed[relPath]; !ok {
			return errors.New("required positive is excluded by the recorded filters")
		}
	}
	return nil
}

func validatePathSet(
	paths []string,
	corpus map[string]struct{},
	disjoint map[string]struct{},
	label string,
) (map[string]struct{}, error) {
	seen := make(map[string]struct{}, len(paths))
	for _, relPath := range paths {
		if _, ok := corpus[relPath]; !ok {
			return nil, fmt.Errorf("%s is outside the corpus", label)
		}
		if _, overlap := disjoint[relPath]; overlap {
			return nil, errors.New("one path has conflicting oracle roles")
		}
		if _, duplicate := seen[relPath]; duplicate {
			return nil, fmt.Errorf("%s is duplicated", label)
		}
		seen[relPath] = struct{}{}
	}
	return seen, nil
}

func knownClass(class queryClass) bool {
	switch class {
	case classTraditionalChineseJapanese, classJapaneseTerm, classCodeTerm, classCrossLingualParaphrase, classFilterMixed:
		return true
	default:
		return false
	}
}

func cjkRunes(value string) int {
	var count int
	for _, r := range value {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			count++
		}
	}
	return count
}

func leaksVaultPath(query string) bool {
	lower := strings.ToLower(query)
	return strings.Contains(query, "/Users/") || strings.Contains(query, "~/") ||
		strings.Contains(lower, "obsidian/") || strings.Contains(lower, ".md")
}

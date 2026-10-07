package note_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestNoteExplainsUnsupportedYAML keeps the parser's implementation failure
// out of the reading page and names the authored forms a reader can change.
func TestNoteExplainsUnsupportedYAML(t *testing.T) {
	t.Parallel()
	for _, authored := range []struct {
		name, body string
	}{
		{"top-level list key", "---\n<<: {a: 1}\n? [1, 2]\n: 3\n---\nReadable body.\n"},
		{"nested list key", "---\nbase: &b {x: 1}\nm:\n  <<: *b\n  ? [1]\n  : 2\n---\nReadable body.\n"},
		{"root key type error before nested merge", "---\n? [0]\n: ignored\nbase: &b {x: 1}\nm:\n  <<: *b\n  ? [1]\n  : 2\n---\nReadable body.\n"},
		{"deep list key", "---\nbase: &b {x: 1}\nouter:\n  middle:\n    inner:\n      <<: *b\n      ? [1]\n      : 2\n---\nReadable body.\n"},
		{"nested mapping key", "---\nbase: &b {x: 1}\nm:\n  <<: *b\n  ? {a: 1}\n  : 2\n---\nReadable body.\n"},
		{"mapping inside list", "---\nbase: &b {x: 1}\nitems:\n  - <<: *b\n    ? [1]\n    : 2\n---\nReadable body.\n"},
	} {
		for _, governed := range []bool{false, true} {
			for _, chrome := range []struct {
				lang wording.Lang
				want string
			}{
				{wording.ZhHant, "frontmatter 使用了 yomihon 讀不進來的 YAML 寫法：合併鍵（<<），或把串列、對應表當作鍵。請直接編輯 frontmatter，改用一般的文字鍵。"},
				{wording.En, "The frontmatter uses a YAML form yomihon cannot read: a merge key (<<), or a list or mapping used as a key. Edit the frontmatter directly to use ordinary text keys."},
			} {
				name := authored.name + "/" + string(chrome.lang) + "/ungoverned"
				if governed {
					name = authored.name + "/" + string(chrome.lang) + "/governed"
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					root := writeNotes(t, map[string]string{"Writing/Bad.md": authored.body})
					contract := loadContract(t)
					governance := schema.Ungoverned()
					if governed {
						governance = contract.Governance()
					}
					server := newServerWithGovernance(t, root, contract, governance)
					page := frontmatterNoticePage(t, server, chrome.lang, "Writing/Bad.md")
					if !strings.Contains(page, strings.ReplaceAll(chrome.want, "<<", "&lt;&lt;")) {
						t.Errorf("caught: note explanation omitted %q", chrome.want)
					}
					for _, unwanted := range []string{"runtime error", "interface {}"} {
						if strings.Contains(page, unwanted) {
							t.Errorf("caught: note explanation leaks %q", unwanted)
						}
					}
				})
			}
		}
	}
}

// TestNotePreservesOtherYAMLErrors keeps a non-scalar key's parser evidence
// when its own mapping has no merge, even if a sibling mapping does.
func TestNotePreservesOtherYAMLErrors(t *testing.T) {
	t.Parallel()
	for _, authored := range []struct {
		name, body string
	}{
		{"no merge", "---\nm:\n  ? [1]\n  : 2\n---\nReadable body.\n"},
		{"merge in sibling mapping", "---\nbase: &b {x: 1}\nmerged:\n  <<: *b\nm:\n  ? [1]\n  : 2\n---\nReadable body.\n"},
		{"ordinary error before unsupported mapping", "---\nbase: &b {x: 1}\nordinary:\n  ? [1]\n  : 2\nunsupported:\n  <<: *b\n  ? [2]\n  : 3\n---\nReadable body.\n"},
		{"quoted merge key", "---\nm:\n  '<<': {x: 1}\n  ? [1]\n  : 2\n---\nReadable body.\n"},
	} {
		for _, governed := range []bool{false, true} {
			for _, chrome := range []struct {
				lang wording.Lang
				want string
			}{
				{wording.ZhHant, "frontmatter 不是有效的 YAML。"},
				{wording.En, "The frontmatter is not valid YAML."},
			} {
				name := authored.name + "/" + string(chrome.lang) + "/ungoverned"
				if governed {
					name = authored.name + "/" + string(chrome.lang) + "/governed"
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					root := writeNotes(t, map[string]string{"Writing/Bad.md": authored.body})
					contract := loadContract(t)
					governance := schema.Ungoverned()
					if governed {
						governance = contract.Governance()
					}
					server := newServerWithGovernance(t, root, contract, governance)
					page := frontmatterNoticePage(t, server, chrome.lang, "Writing/Bad.md")
					for _, want := range []string{chrome.want, "frontmatter is not valid YAML: yaml: invalid map key: []interface {}{1}", "Readable body."} {
						if !strings.Contains(page, want) {
							t.Errorf("caught: ordinary YAML explanation omitted %q", want)
						}
					}
					for _, unwanted := range []string{"frontmatter 使用了 yomihon 讀不進來的 YAML 寫法", "The frontmatter uses a YAML form yomihon cannot read"} {
						if strings.Contains(page, unwanted) {
							t.Errorf("caught: ordinary YAML explanation was replaced by %q", unwanted)
						}
					}
				})
			}
		}
	}
}

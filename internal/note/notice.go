package note

import (
	"strings"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// folderNotices words what the pages owe their reader about the folder as a
// whole, in the reader's language. Independent identity and collision problems
// coexist, so neither notice displaces the other.
func folderNotices(collision []string, root *snapshot.RootNotice, rules *schema.VersionError, lang wording.Lang) []pages.FolderNotice {
	var notices []pages.FolderNotice
	if len(collision) != 0 {
		notices = append(notices, pages.FolderNotice{
			Title:   wording.NoticeNamesCollideTitle.In(lang),
			Summary: wording.NoticeNamesCollide.In(lang),
			Detail:  noticeFiles(collision),
		})
	}
	if root != nil {
		title, summary := wording.NoticeRootChangedTitle, wording.NoticeRootChanged
		detail := wording.RootDetail(lang, vault.Spelled(root.SelectedPath), vault.Spelled(root.OpenedName))
		if root.Unconfirmed {
			title, summary = wording.NoticeRootUnconfirmedTitle, wording.NoticeRootUnconfirmed
		}
		notices = append(notices, pages.FolderNotice{Title: title.In(lang), Summary: summary.In(lang), Detail: detail})
	}
	if rules != nil {
		notices = append(notices, rulesVersionNotice(rules, lang))
	}
	return notices
}

// rulesVersionNotice says what differs between the rules file and the version
// this one reads, and what to do about it: which line to change, or for a file
// in the older format, where the upgrade guide is, since the line alone does not
// make that file readable. The declared value is the vault's own text, so a long
// one is cut rather than allowed to fill the notice.
func rulesVersionNotice(rules *schema.VersionError, lang wording.Lang) pages.FolderNotice {
	const longest = 24
	declared := rules.Declared
	if runes := []rune(declared); len(runes) > longest {
		declared = string(runes[:longest]) + "…"
	}
	if rules.Older() {
		return pages.FolderNotice{
			Title:   wording.NoticeRulesVersionTitle.In(lang),
			Summary: wording.RulesVersionOlder(lang, declared, rules.Supported),
		}
	}
	return pages.FolderNotice{
		Title:   wording.NoticeRulesVersionTitle.In(lang),
		Summary: wording.RulesVersionSummary(lang, declared, rules.Supported),
		Detail:  wording.RulesVersionDetail(lang, rules.Line, rules.Supported),
	}
}

// noticeFiles names the files a notice is about as the filesystem spells them.
// Two names that fold to one path print alike, so each is written with its
// escapes beside it, which is the only place the pair differs.
func noticeFiles(paths []string) string {
	spelled := make([]string, 0, len(paths))
	for _, raw := range paths {
		spelled = append(spelled, vault.Spelled(raw))
	}
	return strings.Join(spelled, "; ")
}

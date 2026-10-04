package note

import (
	"strings"

	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// folderNotices words what the pages owe their reader about the folder as a
// whole, in the reader's language. Independent identity and collision problems
// coexist, so neither notice displaces the other.
func folderNotices(collision []string, root *snapshot.RootNotice, lang wording.Lang) []pages.FolderNotice {
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
		detail := wording.NoticeRootSelected.In(lang) + ": " + vault.Spelled(root.SelectedPath) + "; " +
			wording.NoticeRootOpened.In(lang) + ": " + vault.Spelled(root.OpenedName)
		if root.Unconfirmed != "" {
			title, summary = wording.NoticeRootUnconfirmedTitle, wording.NoticeRootUnconfirmed
			detail += "; " + root.Unconfirmed
		}
		notices = append(notices, pages.FolderNotice{Title: title.In(lang), Summary: summary.In(lang), Detail: detail})
	}
	return notices
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

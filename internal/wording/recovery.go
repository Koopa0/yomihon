package wording

// The write face's refusals. Each is a pair: what happened, and what the reader
// can do about it. Nothing here asks them to press the same button again —
// where a retry is the wrong move the sentence says so, because a refusal a
// reader cannot act on is a wall with no door in it.

// NonInstanceReason says why a resource carries no status control: the folder
// it sits in declares no lifecycle over it, so there is no vocabulary to offer.
var NonInstanceReason = both("不屬於生命週期治理範圍", "Outside lifecycle governance")

// The submitted form could not be read at all.
var (
	FormUnreadable     = both("表單內容無法解析。", "The form could not be read.")
	FormUnreadableNext = both(
		"返回原本的筆記頁面，重新載入後再選擇一次狀態轉換。",
		"Go back to the note, reload it, and choose the transition again.",
	)
)

// The form arrived without the three fields a transition is made of.
var (
	FieldsRequired = both("path、from 與 to 都是必填欄位。", "path, from and to are all required.")
)

// The form arrived without the version the write binds itself to.
var (
	IdentityRequired = both(
		"content_identity 是必填欄位：頁面表單附帶的內容識別值，寫入會綁定送出者讀到的版本。",
		"content_identity is required: the page's form carries the identity of the bytes it showed, and a write binds itself to the version its sender read.",
	)
	IdentityRequiredNext = both(
		"返回筆記並重新載入，從目前頁面的表單重新開始這次操作。",
		"Go back to the note, reload it, and start again from that page's form.",
	)
)

// The path was not one this vault can address.
var (
	PathNotRelative = both(
		"path 必須是 vault 內的相對 slash 路徑。",
		"path must be a slash-separated path relative to the vault.",
	)
	PathNotRelativeNext = both(
		"返回首頁，從導覽重新開啟筆記後再操作。",
		"Go back to the home page and open the note from the navigation again.",
	)
)

// The contract that declares the vocabulary could not be read, so the write
// face is closed rather than guessing.
var (
	ContractUnavailable = both(
		"vault contract 無法使用；生命週期寫入已關閉（fail-closed）。",
		"The vault contract is unavailable; lifecycle writes are closed.",
	)
	ContractUnavailableNext = both(
		"修正 vault contract 後重新啟動 yomihon，再從筆記頁面操作。",
		"Fix the vault contract, restart yomihon, and try again from the note.",
	)
	ArtifactPolicyUnavailable = both(
		"vault artifact policy 無法使用；生命週期寫入已關閉。",
		"The vault artifact policy is unavailable; lifecycle writes are closed.",
	)
	ArtifactPolicyUnavailableNext = both(
		"修正或還原 vault-schema.toml 的 artifact policy，重新啟動 yomihon 後再操作。",
		"Fix or restore the artifact policy in vault-schema.toml, restart yomihon, and try again.",
	)
)

// NotGoverned is for a target that is readable and carries no lifecycle to
// move through.
var NotGoverned = both(
	"這個資源仍可閱讀，但 status 只能在生命週期治理的筆記上變更。",
	"This resource is still readable, but status can only change on a note the lifecycle governs.",
)

// The page's idea of the status is older than the file's.
var (
	PageStale = both(
		"這個頁面已過期；磁碟上的狀態已經不同。",
		"This page is out of date; the status on disk has moved on.",
	)
	PageStaleNext = both(
		"返回筆記並重新載入目前狀態，確認後再選擇一次轉換。",
		"Go back to the note, reload it to see the status it carries now, and choose again.",
	)
)

// The note's bytes changed after the page that offered this transition was
// built, so the write is bound to a version that is no longer there.
var (
	ContentMoved = both(
		"筆記內容在這個頁面載入之後被改過；這次操作綁定的是當時讀到的版本。",
		"The note changed after this page was loaded, and this write is bound to the version it showed.",
	)
	ContentMovedNext = both(
		"重新載入筆記，確認目前的內容後再選擇一次轉換。",
		"Reload the note, read what it says now, and choose again.",
	)
	ContentRaced = both(
		"檔案在讀取與寫入之間遭到修改。",
		"The file changed between being read and being written.",
	)
	ContentRacedNext = both(
		"先檢查 Obsidian 或其他工具的最新變更，再重新載入筆記；不要直接重送。",
		"Check what Obsidian or another tool wrote most recently, then reload the note. Do not resend this.",
	)
)

// PlatformUnsupportedNext is the way on where the platform has no durable
// rename to install a rewrite with.
var PlatformUnsupportedNext = both(
	"改用目前支援 status 寫入的 macOS 或 Linux；閱讀與搜尋仍可在此平台使用。",
	"Use macOS or Linux, where status writes are supported; reading and search work here either way.",
)

// The published status records something that happened outside the vault.
var (
	PublishedRefused = both(
		"published 記錄的是一次已完成的發布；沒有任何發布器能為這次寫入作證，yomihon 不會設定這個值。",
		"published records a publication that already happened. Nothing here can attest to one, so yomihon does not set it.",
	)
	PublishedRefusedNext = both(
		"若發布確實已完成，請直接編輯筆記的 frontmatter 記下這個事實。",
		"If the publication did happen, write it into the note's frontmatter by hand.",
	)
)

// The named path does not lead to a regular file the surgical rewrite
// could bind to. A symbolic link is deliberately never followed: the write
// face rewrites exactly the entry the path names, and a link's target can
// sit outside the vault entirely.
var (
	TargetNotRegular = both(
		"這個路徑不是一般檔案：筆記本身或途中的目錄是 symlink 或其他特殊項目，狀態寫入不跟隨 symlink。",
		"This path is not a regular file: the note itself, or a directory on the way, is a symlink or another special entry, and the status write does not follow symlinks.",
	)
	TargetNotRegularNext = both(
		"請直接用編輯器修改 symlink 指向的實際檔案；yomihon 只改寫 vault 內的一般檔案。",
		"Edit the actual file the link points at in your editor; yomihon only rewrites regular files inside the vault.",
	)
)

// The note named by the form is not where it was.
var (
	NoteGone = both(
		"找不到這篇筆記；它可能已被刪除或移動。",
		"This note cannot be found; it may have been deleted or moved.",
	)
	NoteGoneNext = both(
		"返回首頁，從目前的導覽重新找到筆記；這次操作沒有寫入任何內容。",
		"Go back to the home page and find the note in the navigation. Nothing was written.",
	)
)

// Something failed that the page cannot name.
var (
	WriteFailed     = both("yomihon 無法完成狀態變更。", "yomihon could not complete the status change.")
	WriteFailedNext = both(
		"查看 yomihon 日誌並確認 vault 狀態；在原因不明時不要反覆重送。",
		"Read yomihon's log and check the vault's state. Do not keep resending while the cause is unknown.",
	)
)

// Two versions of the note are on disk and yomihon cannot say which is the one
// the author meant.
var (
	ConcurrentWriteLeftBoth = both(
		"有其他程式在寫入途中改了這個檔案，yomihon 無法確定筆記現在是哪一份內容。兩份都留在磁碟上，沒有刪除任何內容。",
		"Another program changed this file mid-write, and yomihon cannot tell which version the note is now. Both are on disk; nothing was deleted.",
	)
	ConcurrentWriteLeftBothNext = both(
		"不要重送。請依下方訊息比對筆記與旁邊保留的那一份，手動留下正確的內容後再操作。",
		"Do not resend. Compare the note with the copy kept beside it, named below, and keep the right one by hand.",
	)
	DurabilityUnconfirmed = both(
		"筆記已改寫，但無法確認資料已耐久保存。",
		"The note was rewritten, but the write could not be confirmed as durable.",
	)
	DurabilityUnconfirmedNext = both(
		"不要重送。請直接在 vault 中檢查筆記內容與檔案狀態，確認後再手動收尾。",
		"Do not resend. Check the note and the file in the vault, and finish by hand.",
	)
)

// The frontmatter is shaped in a way the surgical rewrite cannot honour.
var (
	StatusFieldInvalid = both(
		"frontmatter 的 status 欄位違反 schema。",
		"The frontmatter's status field does not satisfy the schema.",
	)
	StatusFieldInvalidNext = both(
		"直接編輯筆記，讓 frontmatter 恰好包含一個合法的 status 欄位，再重新載入。",
		"Edit the note so its frontmatter carries exactly one legal status field, then reload.",
	)
	StatusFieldUnsupportedYAML = both(
		"frontmatter 的 status 欄位使用了 yomihon 狀態改寫不支援的 YAML 寫法。",
		"The frontmatter's status field is written in a YAML form the status rewrite does not support.",
	)
	StatusFieldUnsupportedYAMLNext = both(
		"直接編輯筆記，把 status 欄位改成單行的「status: 值」形式（不使用引號鍵、flow mapping 或 YAML 錨點），再重新載入。",
		"Edit the note so the field is a single \"status: value\" line — no quoted key, no flow mapping, no YAML anchor — then reload.",
	)
)

// The schema declined the move itself.
var (
	TransitionRefused = both(
		"vault schema 拒絕這個狀態轉換。",
		"The vault schema refuses this status transition.",
	)
	StatusOutsideEnum = both(
		"狀態值不在 vault schema 的允許清單中。",
		"That status is not in the vault schema's declared list.",
	)
	TransitionNotAllowed = both(
		"vault schema 不允許這個狀態轉換。",
		"The vault schema does not allow this transition.",
	)
	// The repair is a hand edit, and the page's own actions carry the door to
	// where editing happens — so the way on names that door instead of asking
	// the reader to correct something this interface offers no control for.
	SchemaRefusalNext = both(
		"用下方的「在 Obsidian 開啟」直接編輯 frontmatter，依 vault schema 修正狀態值或轉換，再重新載入筆記。",
		"Edit the frontmatter through \"Open in Obsidian\" below, correct the status or the transition to match the vault schema, then reload the note.",
	)
)

// The recovery page itself could not be rendered, so this is all that is left
// to say. What it must carry is whether the note on disk was changed.
var (
	RecoveryRenderFailedUnchanged = both(
		"狀態尚未變更；復原頁面顯示失敗。\n",
		"The status was not changed; the recovery page could not be rendered.\n",
	)
	RecoveryRenderFailedChanged = both(
		"狀態已寫入，需要手動收尾；復原頁面顯示失敗，請勿重送，請直接檢查 vault。\n",
		"The status was written and needs finishing by hand; the recovery page could not be rendered. Do not resend — check the vault.\n",
	)
)

// RecoveryStartOver is what a write refused for a malformed request offers as
// the way on. It names the two pages a reader can get back to rather than
// asking them to fix the request, because the request was the page's to build
// and there is nothing on this one to correct.
var RecoveryStartOver = both(
	"返回筆記或首頁，從目前頁面重新開始這次操作。",
	"Go back to the note or to the home page, and start this again from the page you were on.",
)

// DurabilityUnsupported is refused before anything is written, on a platform
// with no rename this code can trust: a status change that cannot be installed
// durably is one the reader would be told happened and might not find later.
var DurabilityUnsupported = both(
	"此平台無法確認狀態檔案的耐久寫入；生命週期寫入已關閉（fail-closed）。",
	"This platform cannot confirm a durable write of the status file; lifecycle writes are closed.",
)

// NoteStatusUnreadable is shown where the note's own status line could not be
// read for this request. Whatever blocked the read blocks the write too, so
// every control derived from that value would be refused on arrival.
var NoteStatusUnreadable = both(
	"無法讀取這個筆記目前的狀態。狀態操作暫時關閉，重新載入頁面可以再試一次。",
	"This note's current status could not be read. Status controls are closed for now; reloading the page tries again.",
)

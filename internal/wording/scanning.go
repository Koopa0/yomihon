package wording

import (
	"fmt"
	"strconv"
)

// What every page says until the first read of the vault has finished.

// ScanningKicker is the page's whole claim: the vault is being read. It is also
// the body of the refusal a write gets in that time, so a stale tab posting a
// mark meets the same word the page carries.
var ScanningKicker = both("讀取中", "Reading")

// What the address of a vault says when its folder cannot be read: gone since
// the reader last opened it, or not readable by this account. The reasons are
// words for a reader, never an error code.
var (
	VaultReason        = both("原因", "Reason")
	VaultPlace         = both("位置", "Location")
	VaultFolderMissing = both("找不到資料夾", "The folder cannot be found")
	VaultFolderDenied  = both("沒有讀取權限", "No permission to read it")
	VaultFolderOther   = both("資料夾讀不進來", "The folder cannot be read")
	ReadAgain          = both("重新讀取", "Read again")
)

// ScanningFiles is the line that says how far the read has got, with the two
// counts already grouped.
var ScanningFiles = both("%s / %s 個檔案", "%s / %s files")

// ScanningProgress writes how many files of the vault have been read.
func ScanningProgress(lang Lang, done, total int) string {
	return fmt.Sprintf(ScanningFiles.In(lang), groupDigits(done), groupDigits(total))
}

// groupDigits writes n with a comma between each three digits, the way a count
// of files is read at a glance.
func groupDigits(n int) string {
	digits := strconv.Itoa(n)
	if n < 0 || len(digits) <= 3 {
		return digits
	}
	head := len(digits) % 3
	if head == 0 {
		head = 3
	}
	grouped := []byte(digits[:head])
	for i := head; i < len(digits); i += 3 {
		grouped = append(grouped, ',')
		grouped = append(grouped, digits[i:i+3]...)
	}
	return string(grouped)
}

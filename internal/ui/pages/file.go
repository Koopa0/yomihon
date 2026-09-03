package pages

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/wording"
)

// FileKind is how a vault file that is not a note is presented: highlighted
// source, a picture, the browser's document viewer, or — when nothing can show
// it honestly — a page that says what it is and where its bytes are.
type FileKind string

// File presentation kinds select the least surprising native reading surface.
const (
	FileSource FileKind = "source"
	FileImage  FileKind = "image"
	FilePDF    FileKind = "pdf"
	FileInfo   FileKind = "info"
)

// FileView is everything the file page needs. There is no status, transition or
// diagnostic: the write face has no opinion about a file that is not a note.
// SourceHTML is set only for FileSource and holds already-escaped output.
type FileView struct {
	Kind    FileKind
	Title   string
	RelPath string

	// Size and ContentType describe the bytes and are shown on the
	// information page.
	Size        int64
	ContentType string

	SourceHTML string

	Sidebar Sidebar
}

// byteUnits are the steps humanSize climbs; a vault never needs one above a
// gigabyte.
var byteUnits = []string{"KB", "MB", "GB"}

// humanSize renders a byte count the way a person reads one, keeping the exact
// figure alongside it: "2.4 MB" on its own is a rounding, not a fact.
func humanSize(n int64, lang wording.Lang) string {
	if n == 1 {
		return wording.ByteSingular.In(lang)
	}
	if n < 1024 {
		return withThousands(n) + wording.BytesSuffix.In(lang)
	}
	value := float64(n)
	unit := byteUnits[0]
	for _, u := range byteUnits {
		value /= 1024
		unit = u
		if value < 1024 {
			break
		}
	}
	return fmt.Sprintf(wording.ByteSizeFmt.In(lang), value, unit, withThousands(n))
}

func fileKindLabel(kind FileKind, lang wording.Lang) string {
	switch kind {
	case FileSource:
		return wording.FileKindSource.In(lang)
	case FileImage:
		return wording.FileKindImage.In(lang)
	case FilePDF:
		return "PDF"
	case FileInfo, "":
		return wording.FileInfoLabel.In(lang)
	default:
		panic("pages: unknown FileKind: " + string(kind))
	}
}

// withThousands groups a byte count in threes.
func withThousands(n int64) string {
	digits := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

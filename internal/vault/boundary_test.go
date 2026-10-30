package vault

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestScanSkipsExcludedDirectoriesAsItSkipsHiddenOnes holds the boundary to
// the dot-directory's own behaviour: nothing under an excluded directory is
// listed, held as a file or as a directory, and a sibling that matches no
// pattern is read as before.
func TestScanSkipsExcludedDirectoriesAsItSkipsHiddenOnes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "courses/C1/lab/main.go", "package main")
	writeReaderFixture(t, root, "courses/C1/lab/notes/inside.md", "inside")
	writeReaderFixture(t, root, "courses/C1/modules/lesson.md", "lesson")
	writeReaderFixture(t, root, "courses/C2/lab/main.go", "package main")
	writeReaderFixture(t, root, "courses/lab/main.go", "package main")
	writeReaderFixture(t, root, "courses/C1/.hidden/x.md", "hidden")

	reader := openTestReader(t, root)
	reader.SetBoundary(NewBoundary([]string{"courses/*/lab"}))
	available, err := reader.ScanAvailable(t.Context())
	if err != nil {
		t.Fatalf("ScanAvailable() error = %v", err)
	}
	for name, scan := range map[string]Scan{
		"complete":  scanCompleteFixture(t, reader),
		"available": available,
	} {
		var got []string
		for _, entry := range scan.Files() {
			got = append(got, entry.Path())
		}
		want := []string{"courses/C1/modules/lesson.md", "courses/lab/main.go"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s scan files mismatch (-want +got):\n%s", name, diff)
		}
		for _, excluded := range []string{"courses/C1/lab", "courses/C1/lab/main.go", "courses/C1/lab/notes", "courses/C2/lab"} {
			if scan.Contains(excluded) {
				t.Errorf("%s scan Contains(%q) = true, want false", name, excluded)
			}
			if !scan.OutsideScan(excluded + "/anything") {
				t.Errorf("%s scan OutsideScan(%q) = false, want true", name, excluded+"/anything")
			}
		}
		for _, kept := range []string{"courses/C1", "courses/C1/modules", "courses/lab"} {
			if !scan.Contains(kept) {
				t.Errorf("%s scan Contains(%q) = false, want true", name, kept)
			}
		}
	}
}

// TestScanWithoutBoundaryReadsEveryDirectory holds the default: a reader that
// was never given a boundary scans as it always has.
func TestScanWithoutBoundaryReadsEveryDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "courses/C1/lab/main.go", "package main")

	scan := scanCompleteFixture(t, openTestReader(t, root))
	if !scan.Contains("courses/C1/lab/main.go") {
		t.Error("Contains(courses/C1/lab/main.go) = false, want true")
	}
	if scan.OutsideScan("courses/C1/lab/main.go") {
		t.Error("OutsideScan(courses/C1/lab/main.go) = true, want false")
	}
	if !scan.OutsideScan("courses/.git/config") {
		t.Error("OutsideScan(courses/.git/config) = false, want true")
	}
}

func TestBoundaryOutsideScan(t *testing.T) {
	t.Parallel()
	boundary := NewBoundary([]string{"System/templates", "courses/*/lab"})
	tests := []struct {
		relPath string
		want    bool
	}{
		{relPath: "System/templates/L10.md", want: true},
		{relPath: "System/templates/deep/L10.md", want: true},
		{relPath: "System/schemas/vault-schema.toml", want: false},
		{relPath: "courses/C1/lab/main.go", want: true},
		{relPath: "courses/C1/lab", want: false},
		{relPath: "courses/C1/labs/main.go", want: false},
		{relPath: "other/courses/C1/lab/main.go", want: false},
		{relPath: "Notes/.trash/old.md", want: true},
		{relPath: "Notes/visible.md", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			t.Parallel()
			if got := boundary.OutsideScan(tt.relPath); got != tt.want {
				t.Errorf("OutsideScan(%q) = %v, want %v", tt.relPath, got, tt.want)
			}
		})
	}
	if (Boundary{}).OutsideScan("System/templates/L10.md") {
		t.Error("the zero Boundary excludes System/templates/L10.md")
	}
}

func TestValidExcludePattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		valid   bool
	}{
		{pattern: "courses/*/lab", valid: true},
		{pattern: "lab", valid: true},
		{pattern: "a/[bc]/d?", valid: true},
		{pattern: "", valid: false},
		{pattern: ".", valid: false},
		{pattern: "..", valid: false},
		{pattern: "../lab", valid: false},
		{pattern: "a/../b", valid: false},
		{pattern: "/lab", valid: false},
		{pattern: "lab/", valid: false},
		{pattern: "a//b", valid: false},
		{pattern: "a/[b", valid: false},
		{pattern: "a\x00b", valid: false},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			t.Parallel()
			if fault := ValidExcludePattern(tt.pattern); (fault == "") != tt.valid {
				t.Errorf("ValidExcludePattern(%q) = %q, want valid=%v", tt.pattern, fault, tt.valid)
			}
		})
	}
}

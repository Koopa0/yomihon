package sourcebytes

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestScanRejectsDangerousBytesBuiltFromNumericCodes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	data := append([]byte("before"), 0)
	data = append(data, 0xe2, 0x80, 0xa8, 0xe2, 0x80, 0xa9)
	if err := os.WriteFile(filepath.Join(root, "unsafe.md"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []finding{
		{path: "unsafe.md", kind: "NUL"},
		{path: "unsafe.md", kind: "U+2028"},
		{path: "unsafe.md", kind: "U+2029"},
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(finding{})); diff != "" {
		t.Fatalf("dangerous-byte findings mismatch (-want +got):\n%s", diff)
	}
}

func TestScanCoversFrozenTextContracts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, name := range []string{"contract.jsonl", "response.golden"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte{'x', 0}, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []finding{
		{path: "contract.jsonl", kind: "NUL"},
		{path: "response.golden", kind: "NUL"},
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(finding{})); diff != "" {
		t.Fatalf("frozen-contract findings mismatch (-want +got):\n%s", diff)
	}
}

func TestScanExemptsOnlyTheNamedByteClass(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	data := append([]byte{'x', 0}, 0xe2, 0x80, 0xa8)
	if err := os.WriteFile(filepath.Join(root, "fixture.md"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := scan(root, map[finding]struct{}{
		{path: "fixture.md", kind: "U+2028"}: {},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []finding{{path: "fixture.md", kind: "NUL"}}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(finding{})); diff != "" {
		t.Fatalf("class-scoped exemption mismatch (-want +got):\n%s", diff)
	}
}

func TestRepositorySourceBytes(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	fixtures := []struct {
		path  string
		u2028 int
		u2029 int
	}{
		{path: "internal/judge/testdata/golden/escapes.jsonl", u2028: 2, u2029: 2},
		{path: "internal/judge/testdata/vault-escapes/note.md", u2028: 1, u2029: 1},
	}
	allowed := make(map[finding]struct{}, len(fixtures)*2)
	for _, fixture := range fixtures {
		fixturePath := filepath.Join(root, filepath.FromSlash(fixture.path))
		data, err := os.ReadFile(fixturePath) // #nosec G304 -- fixed repository fixtures prove the detector recognizes known raw separators
		if err != nil {
			t.Fatal(err)
		}
		if got := bytes.Count(data, []byte{0xe2, 0x80, 0xa8}); got != fixture.u2028 {
			t.Fatalf("known escape fixture %q has %d raw U+2028 separators, want %d", fixture.path, got, fixture.u2028)
		}
		if got := bytes.Count(data, []byte{0xe2, 0x80, 0xa9}); got != fixture.u2029 {
			t.Fatalf("known escape fixture %q has %d raw U+2029 separators, want %d", fixture.path, got, fixture.u2029)
		}
		allowed[finding{path: fixture.path, kind: "U+2028"}] = struct{}{}
		allowed[finding{path: fixture.path, kind: "U+2029"}] = struct{}{}
	}
	findings, err := scan(root, allowed)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("repository contains dangerous source bytes: %v", findings)
	}
}

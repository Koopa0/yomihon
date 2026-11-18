package archlock

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoProbeNamesTheAddressOfAYomihonInUse holds the browser probes to the
// server the harness started for them. A probe may write a bookmark or a mark,
// so one that falls back to a fixed address drives whatever listens there, which
// on a reader's machine is their own yomihon. Port 9610 is the one a yomihon
// serves on when nothing says otherwise.
func TestNoProbeNamesTheAddressOfAYomihonInUse(t *testing.T) {
	t.Parallel()

	const defaultPort = "9610"
	root := filepath.Join(repoRoot, ".github")
	var probes []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext == ".mjs" || ext == ".sh" {
			probes = append(probes, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	checked := len(probes)
	for _, path := range probes {
		data, err := os.ReadFile(path) // #nosec G304 -- a path the walk of this repository's probes produced
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), defaultPort) {
			t.Errorf("%s names %s, the address a yomihon in use serves on; a probe takes its server from YOMIHON_BASE and has no default", path, defaultPort)
		}
	}
	if checked == 0 {
		t.Fatal("no probe was read, so this check proved nothing")
	}
}

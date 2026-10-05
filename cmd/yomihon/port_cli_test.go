package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestServeRejectsInvalidPortBeforeScanning(t *testing.T) {
	t.Parallel()
	binary := buildYomihonBinary(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "reading.md"), []byte("# Reading\n\nAuthored words.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"abc", "9714abc", "-1", "65536", "70000", "99999999999999999999999999",
		"9924 ", "1.2", "0x26c4", "９９２４",
	} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			exit, stdout, stderr := runYomihonBinary(t, binary, []string{"serve", root}, append(isolatedUserEnv(home), "YOMIHON_PORT="+value))
			want := fmt.Sprintf("yomihon: YOMIHON_PORT %q must be an integer from 0 to 65535\n", value)
			t.Log("invoked: real invalid-port process")
			if exit != 2 || stdout != "" || stderr != want {
				t.Errorf("caught: invalid port %q = exit %d, stdout %q, stderr %q; want 2, empty, %q without scanning", value, exit, stdout, stderr, want)
			}
			assertHomeUntouched(t, home)
		})
	}
}

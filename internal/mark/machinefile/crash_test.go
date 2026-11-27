package machinefile_test

import (
	"bufio"
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/machinefile"
	"github.com/koopa0/yomihon/internal/vault"
)

// A test that needs a writer that dies runs this test binary again as one,
// over the vault this variable names.
const crashVaultVar = "YOMIHON_TEST_MACHINEFILE_CRASH_VAULT"

func TestMain(m *testing.M) {
	if root := os.Getenv(crashVaultVar); root != "" {
		writeUntilKilled(root)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// crashData is what the dying writer writes, by turns: two contents each long
// enough that a kill often lands inside a write, and different all the way
// through, so a file that is part of one and part of the other is told apart
// from either.
var crashData = [2][]byte{
	bytes.Repeat([]byte("{\"turn\":\"a\"}\n"), 1<<16),
	bytes.Repeat([]byte("{\"turn\":\"b\"}\n"), 1<<16),
}

// writeUntilKilled replaces the machine file by turns with each of crashData,
// saying ready after the first write is kept, until it is killed. It returns
// only on a failure, which it has said.
func writeUntilKilled(root string) {
	r, err := vault.Open(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	f, err := machinefile.Open(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	for turn := 0; ; turn++ {
		if _, err = f.Replace(thisMachine, crashData[turn%2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if turn == 0 {
			if _, err = os.Stdout.WriteString("ready\n"); err != nil {
				return
			}
		}
	}
}

// TestAWriterKilledAtAnyMomentLeavesTheOldFileOrTheNew kills a process that is
// replacing the machine file over and over, at a moment nobody chose, again
// and again. Whatever step it was in, writing the temporary file, syncing it,
// renaming it or syncing the folder, what is left under the machine file's name
// is one write whole, and at most the one temporary file it was writing beside
// it. A kill is a process dying and not the power going: what the disk holds
// after a power cut is not something a test can make happen.
func TestAWriterKilledAtAnyMomentLeavesTheOldFileOrTheNew(t *testing.T) {
	t.Parallel()
	root := aVault(t)
	// Kills go on until one has landed between making a temporary file and
	// renaming it, and at least minKills have, up to maxKills: a run in which no
	// kill reached the window it exists for has shown nothing.
	const minKills, maxKills = 12, 200
	caughtWriting, kills := 0, 0
	for kill := 0; kill < maxKills && (kill < minKills || caughtWriting == 0); kill++ {
		kills++
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$") // #nosec G204 G702 -- this test binary, run again as the writer
		cmd.Env = append(os.Environ(), crashVaultVar+"="+root)
		cmd.Stderr = os.Stderr
		out, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		if line, readErr := bufio.NewReader(out).ReadString('\n'); readErr != nil || line != "ready\n" {
			_ = cmd.Process.Kill() //nolint:errcheck // the failure below is the report
			t.Fatalf("the writer said %q, %v; want ready", line, readErr)
		}
		// The moment of the kill is the point of the test, so it is left to chance
		// within a few writes' time rather than waited for.
		time.Sleep(time.Duration(rand.IntN(40)) * time.Millisecond) // #nosec G404 -- the moment of a kill, not a secret
		if err = cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		_ = cmd.Wait() //nolint:errcheck // a killed process answers that it was killed

		got := readMachineFile(t, root)
		if got != string(crashData[0]) && got != string(crashData[1]) {
			t.Fatalf("after kill %d the machine file holds %d bytes that are neither write whole", kill, len(got))
		}
		entries, err := os.ReadDir(filepath.Join(root, "reader-notes", ".yomihon", "marks"))
		if err != nil {
			t.Fatal(err)
		}
		var temps []string
		for _, entry := range entries {
			if name := entry.Name(); name != thisMachine+".jsonl" {
				if !strings.HasPrefix(name, ".tmp-") {
					t.Fatalf("after kill %d the marks folder holds %s", kill, name)
				}
				temps = append(temps, name)
			}
		}
		if len(temps) > caughtWriting {
			caughtWriting++
		}
		if len(temps) > kill+1 {
			t.Fatalf("after kill %d the marks folder holds %d temporary files, more than one a kill", kill, len(temps))
		}
	}
	if caughtWriting == 0 {
		t.Fatalf("none of %d kills landed between making a temporary file and renaming it", kills)
	}
	t.Logf("%d of %d kills landed between making a temporary file and renaming it", caughtWriting, kills)
}

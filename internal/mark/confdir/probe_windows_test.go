package confdir_test

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// TestProbeWhatWindowsFlushes records, on the runner, which of the ways to make
// a rename durable the filesystem accepts. It asserts nothing.
func TestProbeWhatWindowsFlushes(t *testing.T) {
	dir := t.TempDir()
	root := filepath.VolumeName(dir) + `\`
	var fsName [windows.MAX_PATH + 1]uint16
	rootPtr, _ := windows.UTF16PtrFromString(root)
	err := windows.GetVolumeInformation(rootPtr, nil, 0, nil, nil, nil, &fsName[0], uint32(len(fsName)))
	t.Logf("probe: %s is on %q (%v)", dir, windows.UTF16ToString(fsName[:]), err)

	if d, err := os.Open(dir); err != nil {
		t.Logf("probe: os.Open of the directory: %v", err)
	} else {
		t.Logf("probe: os.Open of the directory, then File.Sync: %v", d.Sync())
		_ = d.Close()
	}

	name, _ := windows.UTF16PtrFromString(dir)
	for _, access := range []struct {
		label string
		mask  uint32
	}{{"GENERIC_READ", windows.GENERIC_READ}, {"GENERIC_WRITE", windows.GENERIC_WRITE}, {"GENERIC_READ|GENERIC_WRITE", windows.GENERIC_READ | windows.GENERIC_WRITE}} {
		h, err := windows.CreateFile(name, access.mask, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if err != nil {
			t.Logf("probe: CreateFile(directory, %s, BACKUP_SEMANTICS): %v", access.label, err)
			continue
		}
		t.Logf("probe: CreateFile(directory, %s, BACKUP_SEMANTICS), then FlushFileBuffers: %v", access.label, windows.FlushFileBuffers(h))
		_ = windows.CloseHandle(h)
	}

	target := filepath.Join(dir, "marks.json")
	for i, flags := range []uint32{windows.MOVEFILE_REPLACE_EXISTING | windows.MOVEFILE_WRITE_THROUGH, windows.MOVEFILE_REPLACE_EXISTING | windows.MOVEFILE_WRITE_THROUGH} {
		temporary := filepath.Join(dir, ".marks-probe.json")
		if err := os.WriteFile(temporary, []byte{byte('a' + i)}, 0o600); err != nil {
			t.Fatal(err)
		}
		from, _ := windows.UTF16PtrFromString(temporary)
		to, _ := windows.UTF16PtrFromString(target)
		err := windows.MoveFileEx(from, to, flags)
		held, readErr := os.ReadFile(target)
		t.Logf("probe: MoveFileEx(REPLACE_EXISTING|WRITE_THROUGH), target existed=%v: %v; target holds %q, %v", i == 1, err, held, readErr)
	}
}

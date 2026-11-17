package wording

import "testing"

func TestScanningProgressGroupsTheCounts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		lang        Lang
		done, total int
		want        string
	}{
		{ZhHant, 0, 5, "0 / 5 個檔案"},
		{ZhHant, 1204, 2413, "1,204 / 2,413 個檔案"},
		{En, 999, 1000, "999 / 1,000 files"},
		{En, 1234567, 12345678, "1,234,567 / 12,345,678 files"},
	}
	for _, tt := range tests {
		if got := ScanningProgress(tt.lang, tt.done, tt.total); got != tt.want {
			t.Errorf("ScanningProgress(%q, %d, %d) = %q, want %q", tt.lang, tt.done, tt.total, got, tt.want)
		}
	}
}

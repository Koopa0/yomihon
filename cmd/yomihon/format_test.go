package main

import "testing"

// TestResolveFormat asserts an explicit flag wins and, without one, a terminal
// gets the human view and a pipe the machine view.
func TestResolveFormat(t *testing.T) {
	t.Parallel()
	json, human := formatJSON, formatHuman
	tests := []struct {
		name     string
		explicit *outputFormat
		isTTY    bool
		want     outputFormat
	}{
		{name: "explicit json over a terminal", explicit: &json, isTTY: true, want: formatJSON},
		{name: "explicit human over a pipe", explicit: &human, isTTY: false, want: formatHuman},
		{name: "default terminal is human", explicit: nil, isTTY: true, want: formatHuman},
		{name: "default pipe is json", explicit: nil, isTTY: false, want: formatJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveFormat(tt.explicit, tt.isTTY); got != tt.want {
				t.Errorf("resolveFormat(%v, %t) = %d, want %d", tt.explicit, tt.isTTY, got, tt.want)
			}
		})
	}
}

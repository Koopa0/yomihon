package schema

import "testing"

// TestNormalizeDeclaredDir holds the one reading every contract directory list
// shares. A parent step is refused even where cleaning would walk it away,
// because what is left is a folder its author never wrote down; a plain
// directory spelled the long way round still declares that directory.
func TestNormalizeDeclaredDir(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		declared, want string
		ok             bool
	}{
		{declared: "Private", want: "Private", ok: true},
		{declared: "./Private", want: "Private", ok: true},
		{declared: "Private/", want: "Private", ok: true},
		{declared: "Private//Journal/", want: "Private/Journal", ok: true},
		{declared: "./Private/./Journal", want: "Private/Journal", ok: true},
		{declared: "Cafe\u0301/models", want: "Caf\u00e9/models", ok: true},
		{declared: ""},
		{declared: "."},
		{declared: "./"},
		{declared: ".."},
		{declared: "a/.."},
		{declared: "../Private"},
		{declared: "a/../../outside"},
		{declared: "/Private"},
		{declared: `Private\Journal`},
		{declared: "Private/.."},
		{declared: "Private/../Notes"},
		{declared: "./Private/../Notes"},
	} {
		got, ok := normalizeDeclaredDir(tt.declared)
		if got != tt.want || ok != tt.ok {
			t.Errorf("normalizeDeclaredDir(%+q) = (%+q, %t), want (%+q, %t)", tt.declared, got, ok, tt.want, tt.ok)
		}
	}
}

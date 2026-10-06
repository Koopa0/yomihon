package wording

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestIndexedMonthValueCheck(t *testing.T) {
	t.Parallel()
	zh, zhErr := takes(MonthFmt.In(ZhHant))
	good, goodErr := takes(MonthFmt.In(En))
	badFormat := "%[2]d %[1]s"
	bad, badErr := takes(badFormat)
	if zhErr != nil || goodErr != nil || badErr != nil {
		t.Fatalf("takes month formats: zh=%v en=%v exchanged=%v", zhErr, goodErr, badErr)
	}
	values := []any{2026, "June"}
	output := fmt.Sprintf(badFormat, values...)
	if !strings.Contains(output, "%!d(string=June)") || !strings.Contains(output, "%!s(int=2026)") {
		t.Fatalf("the malformed MonthFmt stimulus was not offered: %q", output)
	}
	t.Log("invocation-hit: indexed-month-type-check")
	if len(zh) == 0 || len(good) == 0 {
		t.Errorf("caught: indexed month operands were read as empty: zh=%v en=%v", zh, good)
	}
	if !reflect.DeepEqual(zh, good) {
		t.Errorf("valid reordered month operands differ: zh=%v en=%v", zh, good)
	}
	if reflect.DeepEqual(zh, bad) {
		t.Errorf("caught: indexed month type exchange was accepted: zh=%v bad=%v output=%q", zh, bad, output)
	}
}

func TestFormatValueKindsUseArgumentIndexes(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		format string
		want   map[int][]string
	}{
		{name: "no operands", format: "finished", want: map[int][]string{}},
		{name: "implicit values", format: "%d %s", want: map[int][]string{1: {"int"}, 2: {"string"}}},
		{name: "reordered values", format: "%[2]s %[1]d", want: map[int][]string{1: {"int"}, 2: {"string"}}},
		{name: "index resumes implicit values", format: "%[2]s %f %[1]d", want: map[int][]string{1: {"int"}, 2: {"string"}, 3: {"float"}}},
		{name: "repeated compatible value", format: "%[1]s %[1]q", want: map[int][]string{1: {"string"}}},
		{name: "repeated conflicting value", format: "%[1]s %[1]d", want: map[int][]string{1: {"int", "string"}}},
		{name: "literal percent before letter", format: "%%d %s", want: map[int][]string{1: {"string"}}},
		{name: "literal percent between values", format: "%[2]s %% %d", want: map[int][]string{2: {"string"}, 3: {"int"}}},
		{name: "flags and precision", format: "%+08d %.1f %#q", want: map[int][]string{1: {"int"}, 2: {"float"}, 3: {"string"}}},
		{name: "flags before index", format: "%+08[2]d %[1]s", want: map[int][]string{1: {"string"}, 2: {"int"}}},
		{name: "kind spelling", format: "%s %q %v %e %g %x", want: map[int][]string{1: {"string"}, 2: {"string"}, 3: {"string"}, 4: {"float"}, 5: {"float"}, 6: {"x"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := takes(tt.format)
			if err != nil {
				t.Fatalf("takes(%q) error = %v", tt.format, err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("caught: argument-indexed kinds for %q mismatch (-want +got):\n%s", tt.format, diff)
			}
		})
	}
}

func TestFormatValueKindsRefuseUnsupportedDirectives(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"%*s", "%.*f", "%[3]*.[2]*[1]f", "%[x]d", "%[0]d", "%[999999999999999999999999999999]d", "%", "%..d"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			got, err := takes(format)
			if err == nil {
				t.Errorf("caught: unsupported directive %q was accepted as %v", format, got)
			}
		})
	}
}

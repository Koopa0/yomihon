package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLinkTargetsSkipHTMLComments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body string
		want       []string
	}{
		{name: "inline", body: "[[Shown]] <!-- [[Hidden]] --> [[Also]]", want: []string{"Shown", "Also"}},
		{name: "multiline", body: "[[Shown]]\n<!--\n[[Hidden]]\n-->\n[[Also]]", want: []string{"Shown", "Also"}},
		{name: "greater than", body: "<!-- private > [[Hidden]] --> [[Shown]]", want: []string{"Shown"}},
		{name: "percent inside HTML", body: "<!-- %% [[Hidden]] --> [[Shown]]", want: []string{"Shown"}},
		{name: "HTML inside percent", body: "%% <!-- [[Hidden]] %% [[Shown]]", want: []string{"Shown"}},
		{name: "unclosed", body: "[[Shown]] <!-- [[Hidden]]", want: []string{"Shown"}},
		{name: "code opener", body: "`<!--` [[Shown]]", want: []string{"Shown"}},
		{name: "unclosed quote", body: "[[Shown]]\n> <!-- [[Hidden]]\n> secret\n\n[[Also]]", want: []string{"Shown", "Also"}},
		{name: "unclosed list", body: "- Item\n  <!-- [[Hidden]]\n  secret\n\n[[Also]]", want: []string{"Also"}},
		{name: "escaped opener", body: `\<!-- [[Shown]] -->`, want: []string{"Shown"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, LinkTargets(tt.body)); diff != "" {
				t.Errorf("LinkTargets HTML comments (-want +got):\n%s", diff)
			}
		})
	}
}

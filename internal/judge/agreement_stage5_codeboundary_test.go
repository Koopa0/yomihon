package judge_test

import (
	"testing"

	"github.com/koopa0/yomihon/internal/judge"
)

// A citation starts at its opening brackets. A closing delimiter in prose
// cannot turn brackets that belong to an inline code span into a citation.
// These literals inspect actual HTML and the complete public judge wire.
func TestAgreementStage5CodeBoundary(t *testing.T) {
	stage5Literals(t, "code-boundary", []stage5Literal{
		{
			Name: "link opening in code and closing brackets outside",
			Body: "`[[Missing`]]\n",
			Want: stage5Shape{Text: "[[Missing]]", Roles: []string{"p", "code"}, Code: []string{"[[Missing"}},
		},
		{
			Name: "embed opening in code and closing brackets outside",
			Body: "`![[Missing`]]\n",
			Want: stage5Shape{Text: "![[Missing]]", Roles: []string{"p", "code"}, Code: []string{"![[Missing"}},
		},
		{
			Name: "target-like embed opening remains code",
			Body: "`![[N`]]\n",
			Want: stage5Shape{Text: "![[N]]", Roles: []string{"p", "code"}, Code: []string{"![[N"}},
		},
		{
			Name: "resolved target embed opening remains code",
			Body: "`![[N|alias`]]\n",
			Want: stage5Shape{Text: "![[N|alias]]", Roles: []string{"p", "code"}, Code: []string{"![[N|alias"}},
		},
		{
			Name: "proper whole link code control",
			Body: "`[[Missing]]`\n",
			Want: stage5Shape{Text: "[[Missing]]", Roles: []string{"p", "code"}, Code: []string{"[[Missing]]"}},
		},
		{
			Name: "proper whole embed code control",
			Body: "`![[N]]`\n",
			Want: stage5Shape{Text: "![[N]]", Roles: []string{"p", "code"}, Code: []string{"![[N]]"}},
		},
		{
			Name: "live missing prose link control",
			Body: "[[Missing]]\n",
			Want: stage5Live("Missing", "p"), Targets: []string{"Missing"},
			Findings: []judge.Finding{stage5MissingFinding(9)}, Wire: stage5Wire9, WarnExit: 1,
		},
		{
			Name: "live prose embed control",
			Body: "![[N]]\n",
			Want: stage5Shape{Text: "From N note", Roles: []string{"div.embed", "p.embed__source", "p"}, Transcluded: true},
			Targets: []string{"N"},
		},
	})
}

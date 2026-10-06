package wording

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRootDetail(t *testing.T) {
	t.Parallel()

	selected, opened := "\"/selected%<&>\"", "\"startup\\\"name\""
	for _, tt := range []struct {
		name string
		lang Lang
		want string
	}{
		{name: "zh-Hant", lang: ZhHant, want: "選取路徑：\"/selected%<&>\"；啟動時開啟的名稱：\"startup\\\"name\""},
		{name: "en", lang: En, want: "Selected path: \"/selected%<&>\"; Startup opened name: \"startup\\\"name\""},
		{name: "default", want: "選取路徑：\"/selected%<&>\"；啟動時開啟的名稱：\"startup\\\"name\""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, RootDetail(tt.lang, selected, opened)); diff != "" {
				t.Errorf("caught: whole root wording (-want +got):\n%s", diff)
			}
		})
	}
}

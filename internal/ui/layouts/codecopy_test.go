package layouts

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

func TestCodeCopyPrototypeKeepsLocalizedNativeActionAndCanonicalReply(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		lang                       wording.Lang
		tag, label, copied, failed string
	}{
		{wording.ZhHant, "zh-Hant", "複製程式碼", "已複製。", "請選取程式碼，手動複製。"},
		{wording.En, "en", "Copy code", "Copied.", "Select the code and copy it manually."},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			if err := CodeCopy(tc.lang).Render(t.Context(), &out); err != nil {
				t.Fatalf("render prototype: %v", err)
			}
			t.Log("invoked: code-copy prototype " + tc.tag)
			html := out.String()
			for _, want := range []string{"<template data-codecopy-template", `<button type="button"`, `lang="` + tc.tag + `"`, tc.label, tc.copied, tc.failed, `role="status"`, `aria-live="polite"`, `aria-atomic="true"`, `data-codecopy-reply`} {
				if !strings.Contains(html, want) {
					t.Errorf("caught: code-copy prototype lacks %q", want)
				}
			}
			if strings.Count(html, "<button") != 1 || strings.Count(html, "<p ") != 1 || !strings.HasSuffix(html, "</template>") {
				t.Error("caught: code-copy prototype does not keep one inert action and one reply")
			}
		})
	}
}

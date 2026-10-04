package pages

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/assets"
)

// The rail runs the embedded initializer before parsing the article. Keeping
// its complete source here makes later module calls share the same behavior.
var sidebarSource = func() string {
	source, err := assets.Files.ReadFile("js/sidebar.js")
	if err != nil {
		panic(err)
	}
	declaration, err := sidebarDeclaration(string(source))
	if err != nil {
		panic(err)
	}
	return declaration
}()

// sidebarDeclaration accepts only the fixed initializer's compiled source.
// A closing script token would escape the HTML body even inside a JS string.
func sidebarDeclaration(source string) (string, error) {
	if !strings.HasPrefix(source, "export function initSidebar() {") {
		return "", errors.New("sidebar source must begin with its exported initializer")
	}
	if strings.Contains(strings.ToLower(source), "</script") {
		return "", errors.New("sidebar source contains an HTML closing script token")
	}
	return strings.TrimPrefix(source, "export "), nil
}

func sidebarInitializer(nonce string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, `<script nonce="`+templ.EscapeString(nonce)+`">(() => {
"use strict";
`+sidebarSource+`
initSidebar();
})();</script>`)
		return err
	})
}

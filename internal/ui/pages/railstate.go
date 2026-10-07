package pages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/assets"
)

var railModuleDeclaration = regexp.MustCompile(`(?m)^[\t ]*(?:import|export)\b`)

// railFilterDeclaration projects the one import-free factory, whose final export
// is the module's only declaration beyond the plain function. Header comments
// are preserved. A closing script token would escape even inside a JS string.
func railFilterDeclaration(source string) (string, error) {
	if strings.Contains(strings.ToLower(source), "</script") {
		return "", errors.New("rail filter source contains an HTML closing script token")
	}
	const export = "export { initRailFilter };"
	source = strings.TrimSpace(source)
	if !strings.HasSuffix(source, "\n"+export) {
		return "", errors.New("rail filter source must end with its single factory export")
	}
	body := strings.TrimSuffix(source, "\n"+export)
	declaration := strings.TrimSpace(body)
	for {
		switch {
		case strings.HasPrefix(declaration, "//"):
			_, declaration, _ = strings.Cut(declaration, "\n")
		case strings.HasPrefix(declaration, "/*"):
			_, declaration, _ = strings.Cut(declaration, "*/")
		default:
			if !strings.HasPrefix(declaration, "function initRailFilter(rail, input) {") || !strings.HasSuffix(declaration, "}") || railModuleDeclaration.MatchString(declaration) {
				return "", errors.New("rail filter source must contain only its plain import-free factory")
			}
			return body, nil
		}
		declaration = strings.TrimSpace(declaration)
	}
}

func railFilterInitializer(nonce string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		source, err := assets.Files.ReadFile("js/rail-filter.js")
		if err != nil {
			return fmt.Errorf("read rail filter source: %w", err)
		}
		declaration, err := railFilterDeclaration(string(source))
		if err != nil {
			return fmt.Errorf("project rail filter source: %w", err)
		}
		_, err = io.WriteString(w, `<script nonce="`+templ.EscapeString(nonce)+`">(() => {
"use strict";
`+declaration+`
const rail = document.querySelector('.y-rail-left');
const input = rail?.querySelector('[data-nav-filter]');
if (input) initRailFilter(rail, input);
})();</script>`)
		return err
	})
}

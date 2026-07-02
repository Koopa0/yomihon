// Package render turns Obsidian-dialect markdown into HTML.
//
// Fault-tolerant by contract (wall 4): it renders what it can and reports
// what it can't — it never fixes or rejects a note. Raw HTML passes through
// unsanitized because the vault is a trusted, local-only corpus (wall 2);
// Japanese lesson bodies are hand-written <ruby> markup that must survive.
//
// Reference implementation for the dialect passes (callouts, wikilinks,
// CJK-safe heading slugs — M1+): yomihon's internal/markdown/parser.go.
package render

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
)

// Renderer is a configured, reusable markdown pipeline.
type Renderer struct {
	md goldmark.Markdown
}

// New builds the pipeline: GFM (tables, task lists, strikethrough) plus raw
// HTML passthrough. Frontmatter is split off by the vault package before the
// body ever reaches here.
func New() *Renderer {
	return &Renderer{
		md: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()),
		),
	}
}

// HTML renders a markdown body.
func (r *Renderer) HTML(body string) (string, error) {
	var buf bytes.Buffer
	if err := r.md.Convert([]byte(body), &buf); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return buf.String(), nil
}

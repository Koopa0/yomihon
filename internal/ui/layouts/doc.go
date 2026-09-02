// Package layouts holds the outer HTML shell: the themed document, the shared
// header chrome, the ⌘K search dialog, and the per-request state stamped onto
// the document root. Styling is the repository-owned sheet at /static/app.css
// plus the chroma syntax sheet — no inline <style>.
//
// It is a package of its own, and small, because the boundary is what it buys:
// a page is written by supplying a body to the document here, so a page cannot
// be written outside the shell by accident, and no page can reach past the
// document to add a stylesheet, a script tag, or a second header. Everything
// above a page's own body is decided once, here, which is also what lets the
// chrome hold still while a reader moves between pages. Folding these files in
// beside the pages would leave that rule with nowhere to live but a habit.
//
// The plural name is the one the tree already carries and is left alone; the
// package holds one document, and a rename would touch every page for a
// letter.
//
// The package doc lives in this hand-written file rather than in a .templ:
// generation splits a comment written there away from the package clause, so
// it reaches neither the rendered documentation nor the linter that checks a
// package has one. A non-generated file must own the package comment.
package layouts

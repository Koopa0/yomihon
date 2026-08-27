// Package layouts holds the outer HTML shell: the themed document, the shared
// header chrome, and the ⌘K search dialog. Styling is the repository-owned
// sheet at /static/app.css plus the chroma syntax sheet — no inline <style>.
//
// The package doc lives in this hand-written file rather than in a .templ:
// generation splits a comment written there away from the package clause, so
// it reaches neither the rendered documentation nor the linter that checks a
// package has one. A non-generated file must own the package comment.
package layouts

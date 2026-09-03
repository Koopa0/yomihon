// Package layouts holds the outer HTML shell: the themed document, the shared
// header chrome, the ⌘K search dialog, and the per-request state stamped onto
// the document root. A page is written by supplying a body to the document
// here, so nothing above a page's own body is decided anywhere else. The doc
// lives in a hand-written file because generation splits one written in a
// template away from the package clause, leaving the package without a comment.
package layouts

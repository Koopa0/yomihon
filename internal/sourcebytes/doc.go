// Package sourcebytes locks one property of this repository's own text: no
// tracked source or data file carries a NUL, a U+2028 or a U+2029 outside a
// named exemption. All three are invisible in review, and the last two end a
// line for a JavaScript parser while ending nothing for a human reader. The
// check is a test, so this file exists to carry the package comment.
package sourcebytes

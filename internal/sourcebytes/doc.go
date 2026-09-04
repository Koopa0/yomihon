// Package sourcebytes locks one property of this repository's own text: no
// file it reads as text carries a NUL, a U+2028 or a U+2029 outside a named
// exemption. All three are invisible in review, and the last two end a line
// for a JavaScript parser while ending nothing for a human reader.
//
// What it reads is the working tree, and it decides what is text from the name
// alone: a listed extension, or one of a few names written out. A listed set
// of directories is skipped whole. It never asks git what is tracked, so the
// two sets differ in both directions — a tracked file whose extension is not on
// that list, or which has none at all, is not read: a fuzz corpus seed, a
// checksum list, a gate's expected output. A file present but untracked is
// read. Saying so here rather than
// promising every tracked file is the difference between a reader knowing
// which of their files this protects and assuming it protects all of them.
//
// The check is a test, so this file exists to carry the package comment.
package sourcebytes

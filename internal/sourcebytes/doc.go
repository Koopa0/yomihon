// Package sourcebytes locks one property of this repository's own text: no
// tracked source or data file carries a NUL, a U+2028 or a U+2029 except at a
// named exemption, and every exemption still contains the byte it was granted
// for.
//
// All three are invisible in a review and in ordinary search tools, and the
// last two terminate a line for a JavaScript parser while terminating nothing
// for a human reader — so a file that quietly acquires one stops saying what
// everyone looking at it believes it says, and a frozen wire format built out
// of lines stops meaning what its consumer parses.
//
// The check walks the repository and runs as an ordinary test, which is why
// this package declares nothing the binary links: what it verifies is the
// tree, not something the program does. This file carries the package comment
// so that the reason reaches go doc, which does not read a test file.
package sourcebytes

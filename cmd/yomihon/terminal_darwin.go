package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// isTerminal reports whether f is a terminal a person types at. A character
// device is not enough: /dev/null is one, and a program piping from it is
// exactly what the approval command must tell apart from a person.
func isTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TIOCGETA) // #nosec G115 -- a descriptor number fits an int
	return err == nil
}

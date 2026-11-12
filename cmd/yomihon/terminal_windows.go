package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// isTerminal reports whether f is a console a person types at, rather than a
// pipe or a file.
func isTerminal(f *os.File) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(f.Fd()), &mode) == nil
}

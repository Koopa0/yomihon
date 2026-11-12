package main

import "os"

// isTerminal reports whether f looks like a terminal a person types at: a
// character device that is not the null device. A pipe, a regular file and
// /dev/null all fail it, which is what tells a program feeding an answer from
// a person typing one.
//
// It is a speed bump and is described as one. The usual test for a terminal
// asks the device itself, through the raw system-call wrappers only the two
// writing packages may import, and even that test passes for a program that
// allocates a terminal of its own, as the script command does. What decides
// whether a process can approve is whether it can write the approvals file,
// and a process running as the owner can.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	null, err := os.Stat(os.DevNull)
	return err != nil || !os.SameFile(info, null)
}

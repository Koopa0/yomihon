//go:build !linux && !darwin

package status

import "os"

// refuseHardLinked has no portable nlink on these targets. The write face
// already refuses before opening the note (durable install is unsupported).
func refuseHardLinked(os.FileInfo, string) error { return nil }

func copyXattrsFrom(*os.Root, string, *os.File) error { return nil }

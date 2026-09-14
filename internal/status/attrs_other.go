//go:build !linux && !darwin

package status

import "os"

// refuseHardLinked has no portable nlink on these targets. The write face
// already refuses before opening the note (durable install is unsupported).
func refuseHardLinked(os.FileInfo, string) error { return nil }

// copyXattrsFrom and currentXattrs report an empty set on these targets, so
// the install's comparison of the source's attributes never refuses.
func copyXattrsFrom(*os.Root, string, *os.File, func(int) ([]string, error)) (map[string][]byte, error) {
	return map[string][]byte{}, nil
}

func currentXattrs(*os.Root, string, func(int) ([]string, error)) (map[string][]byte, error) {
	return map[string][]byte{}, nil
}

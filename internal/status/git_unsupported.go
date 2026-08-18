//go:build !darwin && !linux

package status

import (
	"context"
	"fmt"
	"os"
)

func runGit(context.Context, *os.Root, ...string) ([]byte, error) {
	return nil, fmt.Errorf("status: exact-root git is unavailable: %w", ErrDurabilityUnsupported)
}

func runGitInput(context.Context, *os.Root, []byte, ...string) ([]byte, error) {
	return nil, fmt.Errorf("status: exact-root git is unavailable: %w", ErrDurabilityUnsupported)
}

func execGitChild([]string) error {
	return fmt.Errorf("exact-root git is unavailable: %w", ErrDurabilityUnsupported)
}

//go:build darwin || linux

package status

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const gitRootFD = 3

// runGit executes git in root without resolving the vault pathname again.
// The directory descriptor becomes fd 3 in a private same-binary child; that
// child chdirs through the descriptor and replaces itself with git. Replacing
// the child, rather than starting a grandchild, keeps CommandContext's process
// cancellation attached to the actual git process.
func runGit(ctx context.Context, root *os.Root, args ...string) ([]byte, error) {
	return runGitInput(ctx, root, nil, args...)
}

// runGitInput is runGit with the given bytes supplied to git on standard
// input, for subcommands that read content from stdin instead of a path.
func runGitInput(ctx context.Context, root *os.Root, input []byte, args ...string) ([]byte, error) {
	dir, err := root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("duplicate git root: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("locate status git child: %w", err), dir.Close())
	}
	childArgs := make([]string, 0, len(args)+1)
	childArgs = append(childArgs, gitChildCommand)
	childArgs = append(childArgs, args...)
	cmd := exec.CommandContext(ctx, executable, childArgs...) // #nosec G204 G702 -- executable is this binary; arguments remain discrete and the child can only exec git
	cmd.ExtraFiles = []*os.File{dir}
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	out, runErr := cmd.CombinedOutput()
	closeErr := dir.Close()
	if runErr != nil {
		return out, errors.Join(
			fmt.Errorf("git %v: %w: %s", args, runErr, bytes.TrimSpace(out)),
			closeErr,
		)
	}
	if closeErr != nil {
		return out, fmt.Errorf("close duplicated git root: %w", closeErr)
	}
	return out, nil
}

func execGitChild(args []string) error {
	git, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("locate git: %w", err)
	}
	git, err = filepath.Abs(git)
	if err != nil {
		return fmt.Errorf("resolve git executable: %w", err)
	}
	root := os.NewFile(gitRootFD, "yomihon-vault-root")
	if root == nil {
		return errors.New("inherited vault root is unavailable")
	}
	if err := root.Chdir(); err != nil {
		return errors.Join(fmt.Errorf("enter inherited vault root: %w", err), root.Close())
	}
	if err := root.Close(); err != nil {
		return fmt.Errorf("close inherited vault root: %w", err)
	}
	argv := make([]string, 0, len(args)+1)
	argv = append(argv, "git")
	argv = append(argv, args...)
	return syscall.Exec(git, argv, gitChildEnvironment(os.Environ())) // #nosec G204 G702 -- the executable is fixed to git; arguments remain discrete argv values
}

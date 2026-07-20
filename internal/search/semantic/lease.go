package semantic

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

type writerLease struct {
	file *os.File
	once sync.Once
	err  error
}

func acquireWriterLease(parent *storeParent, name string) (*writerLease, error) {
	return acquireWriterLeaseFile(parent, name, true)
}

func acquireExistingWriterLease(parent *storeParent, name string) (*writerLease, error) {
	return acquireWriterLeaseFile(parent, name, false)
}

func acquireWriterLeaseFile(parent *storeParent, name string, create bool) (*writerLease, error) {
	if err := parent.requireCurrent(); err != nil {
		return nil, err
	}
	file, err := openPrivateLeaseFile(parent.root, name, create)
	if err != nil {
		return nil, err
	}
	if err := lockWriterFileNonblocking(file); err != nil {
		closeErr := file.Close()
		if errors.Is(err, ErrWriterHeld) {
			return nil, errors.Join(err, closeErr)
		}
		return nil, errors.Join(fmt.Errorf("lock semantic cache writer lease: %w", err), closeErr)
	}
	lease := &writerLease{file: file}
	if err := parent.requireCurrent(); err != nil {
		return nil, errors.Join(err, lease.Close())
	}
	return lease, nil
}

func openPrivateLeaseFile(root *os.Root, name string, create bool) (*os.File, error) {
	before, statErr := root.Lstat(name)
	switch {
	case statErr == nil:
		if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Mode().Perm() != 0o600 {
			return nil, fmt.Errorf("%w: writer lease must be a regular 0600 file", ErrStorePermissions)
		}
	case errors.Is(statErr, os.ErrNotExist) && !create:
		return nil, ErrStoreNotFound
	case !errors.Is(statErr, os.ErrNotExist):
		return nil, fmt.Errorf("stat semantic cache writer lease: %w", statErr)
	}

	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	file, err := root.OpenFile(name, flags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open semantic cache writer lease: %w", err)
	}
	after, inspectErr := file.Stat()
	if inspectErr != nil {
		return nil, errors.Join(fmt.Errorf("inspect semantic cache writer lease: %w", inspectErr), file.Close())
	}
	if !after.Mode().IsRegular() || after.Mode().Perm() != 0o600 || before != nil && !os.SameFile(before, after) {
		return nil, errors.Join(
			fmt.Errorf("%w: writer lease changed identity or permissions", ErrStorePermissions),
			file.Close(),
		)
	}
	return file, nil
}

func (l *writerLease) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		unlockErr := unlockWriterFile(l.file)
		closeErr := l.file.Close()
		l.err = errors.Join(unlockErr, closeErr)
		if l.err != nil {
			l.err = fmt.Errorf("release semantic cache writer lease: %w", l.err)
		}
	})
	return l.err
}

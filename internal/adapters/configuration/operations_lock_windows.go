package configuration

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Lock serializes operations and recovery across store instances and processes.
// Windows releases the lock if the owning process crashes.
func (s *FileOperationStore) Lock() (func(), error) {
	if err := os.MkdirAll(s.Root, 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(s.Root, ".operation.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	overlapped := &windows.Overlapped{}
	err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlapped)
	if err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlapped)
		file.Close()
	}, nil
}

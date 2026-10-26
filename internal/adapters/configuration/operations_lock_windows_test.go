package configuration

import "testing"

func TestOperationLockExcludesOtherStore(t *testing.T) {
	root := t.TempDir()
	first := NewFileOperationStore(root)
	release, err := first.Lock()
	if err != nil {
		t.Fatal(err)
	}
	second := NewFileOperationStore(root)
	if unlock, err := second.Lock(); err == nil {
		unlock()
		release()
		t.Fatal("concurrent owner acquired lock")
	}
	if err := second.RecoverInterrupted(); err == nil {
		release()
		t.Fatal("recovery ran during active operation")
	}
	release()
	unlock, err := second.Lock()
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}

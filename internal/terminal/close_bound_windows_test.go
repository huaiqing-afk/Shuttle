//go:build windows

package terminal

import (
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// A child that ignores the console close event must not block Close forever
// (L04). The grace period is 1.5s plus a bounded terminate wait.
func TestCloseIsBoundedAndTerminatesChild(t *testing.T) {
	shell, err := exec.LookPath("pwsh.exe")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		s, err := Start(shell, []string{"-NoLogo", "-NoProfile", "-Command", "Start-Sleep -Seconds 300"}, t.TempDir(), func([]byte) {})
		if err != nil {
			t.Fatal(err)
		}
		pid := s.pty.Pid()
		started := time.Now()
		if err := s.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		if elapsed := time.Since(started); elapsed > 8*time.Second {
			t.Fatalf("close took %v; cleanup must stay bounded", elapsed)
		}
		if alive(t, pid) {
			t.Fatalf("iteration %d: child %d still running after Close", i, pid)
		}
	}
}

func alive(t *testing.T, pid int) bool {
	t.Helper()
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false // exited and handle released
	}
	defer windows.CloseHandle(handle)
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		t.Fatalf("wait probe: %v", err)
	}
	return state == uint32(windows.WAIT_TIMEOUT)
}

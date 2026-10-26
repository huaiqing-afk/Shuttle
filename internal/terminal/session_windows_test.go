//go:build windows

package terminal

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConPTYUnicodeInputResizeAndExit(t *testing.T) {
	shell, err := exec.LookPath("pwsh.exe")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var output bytes.Buffer
	s, err := Start(shell, []string{"-NoLogo", "-NoProfile", "-Command", "$x=[Console]::ReadLine(); [Console]::WriteLine('RESULT:'+ $x)"}, t.TempDir(), func(data []byte) {
		mu.Lock()
		output.Write(data)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Resize(120, 40); err != nil {
		t.Fatal(err)
	}
	if err := s.Resize(0, 40); err == nil {
		t.Fatal("invalid size accepted")
	}
	if err := s.Write([]byte("中文输入\r")); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- s.Wait() }()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("terminal did not exit")
	}
	// Output drain is asynchronous; wait within a deadline rather than assuming Wait drains the pipe.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		text := output.String()
		mu.Unlock()
		if strings.Contains(text, "RESULT:中文输入") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	t.Fatalf("Unicode roundtrip missing: %q", output.String())
}

func TestConPTYCloseActiveProcess(t *testing.T) {
	shell, err := exec.LookPath("pwsh.exe")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Start(shell, []string{"-NoLogo", "-NoProfile", "-Command", "Start-Sleep -Seconds 120"}, t.TempDir(), func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("close did not complete")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

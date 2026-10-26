//go:build windows

package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"syscall"

	"github.com/UserExistsError/conpty"
	"golang.org/x/sys/windows"
)

type Session struct {
	pty *conpty.ConPty
	// proc is our own handle: conpty closes its copy on Close, and the child
	// is not guaranteed to honour CTRL_CLOSE_EVENT, so Close needs a bounded
	// wait plus an explicit terminate.
	proc     windows.Handle
	mu       sync.Mutex
	once     sync.Once
	cancel   context.CancelFunc
	waited   chan struct{}
	done     chan struct{}
	readDone chan struct{}
	err      error
	code     uint32
	closeErr error
}

// Start accepts an executable and arguments; quoting is Windows argv encoding, not shell interpolation.
func Start(executable string, args []string, dir string, output func([]byte)) (*Session, error) {
	parts := []string{syscall.EscapeArg(executable)}
	for _, arg := range args {
		parts = append(parts, syscall.EscapeArg(arg))
	}
	pty, err := conpty.Start(strings.Join(parts, " "), conpty.ConPtyWorkDir(dir), conpty.ConPtyDimensions(100, 30))
	if err != nil {
		return nil, fmt.Errorf("start terminal: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{pty: pty, cancel: cancel,
		waited: make(chan struct{}), done: make(chan struct{}), readDone: make(chan struct{})}
	// Pid() is the PID; reopen it as a handle we own for the bounded wait.
	proc, procErr := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pty.Pid()))
	if procErr != nil {
		_ = pty.Close()
		return nil, fmt.Errorf("open terminal process: %w", procErr)
	}
	s.proc = proc
	go func() {
		defer close(s.readDone)
		buf := make([]byte, 16384)
		for {
			n, readErr := pty.Read(buf)
			if n > 0 {
				output(append([]byte(nil), buf[:n]...))
			}
			if readErr != nil {
				return
			}
		}
	}()
	go func() {
		s.code, s.err = pty.Wait(ctx)
		close(s.waited)
		_ = s.Close()
	}()
	return s, nil
}

func (s *Session) Write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.done:
		return errors.New("terminal exited")
	default:
	}
	n, err := s.pty.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}

func (s *Session) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 || cols > 32767 || rows > 32767 {
		return errors.New("invalid terminal dimensions")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.done:
		return errors.New("terminal exited")
	default:
	}
	return s.pty.Resize(cols, rows)
}

func (s *Session) Close() error {
	s.once.Do(func() {
		s.cancel()
		<-s.waited // Wait must release the process handle before ConPTY closes it.
		s.mu.Lock()
		defer s.mu.Unlock()
		s.closeErr = s.pty.Close()
		// ClosePseudoConsole only asks the client to exit; a child busy or
		// ignoring CTRL_CLOSE_EVENT would block forever (L02: cleanup waits
		// must be bounded). Terminate after a short grace period.
		state, waitErr := windows.WaitForSingleObject(s.proc, 1500)
		if waitErr != nil {
			s.closeErr = errors.Join(s.closeErr, waitErr)
		}
		if state == uint32(windows.WAIT_TIMEOUT) {
			if err := windows.TerminateProcess(s.proc, 1); err != nil {
				s.closeErr = errors.Join(s.closeErr, err)
			}
			windows.WaitForSingleObject(s.proc, 5000)
		}
		windows.CloseHandle(s.proc)
		<-s.readDone
		close(s.done)
	})
	return s.closeErr
}

func (s *Session) Wait() error {
	<-s.done
	if s.closeErr != nil {
		return s.closeErr
	}
	if s.err != nil && !strings.Contains(s.err.Error(), "wait canceled") {
		return s.err
	}
	if s.code != 0 && s.code != 259 {
		return fmt.Errorf("process exit code %d", s.code)
	}
	return nil
}

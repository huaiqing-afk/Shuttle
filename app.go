package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"shuttle/internal/agents"
	"shuttle/internal/terminal"
)

type Detection struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
}

// maxTerminals caps concurrent ConPTY sessions; the UI mirrors the same limit.
const maxTerminals = 8

type App struct {
	ctx               context.Context
	mu                sync.Mutex
	sessions          map[string]*terminal.Session
	terminal          atomic.Int64
	exiting           atomic.Bool
	configurationBusy atomic.Bool
	installBusy       atomic.Bool
	configuration     *configurationService
	inventory         *inventoryService
	reconcileError    error
	reconciled        int
}

func NewApp() *App { return &App{sessions: map[string]*terminal.Session{}} }
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.configuration = newConfigurationService()
	a.inventory = newInventoryService()
	// A crash mid-install leaves an intent record; close it by re-detecting.
	// Failure here must not stop the app from starting, so it is reported, not
	// returned: the panel will show the unresolved record as unknown.
	if closed, err := a.ReconcileInventory(); err != nil {
		a.reconcileError = err
	} else if len(closed) > 0 {
		a.reconciled = len(closed)
	}
}

func resolve(name string) (string, error) {
	if name == "powershell" {
		return exec.LookPath("pwsh.exe")
	}
	if name != "claude" && name != "codex" {
		return "", errors.New("unsupported agent")
	}
	// npm's PowerShell shim is a script, so launch it through PowerShell rather than CreateProcess.
	return exec.LookPath(name + ".cmd")
}

func (a *App) DetectEnvironment() []Detection {
	results := []Detection{}
	for _, name := range []string{"powershell", "claude", "codex"} {
		path, err := resolve(name)
		results = append(results, Detection{Name: name, Path: path, Available: err == nil})
	}
	return results
}

func (a *App) StartTerminal(agent string) (string, error) {
	target, err := resolve(agent)
	if err != nil {
		return "", err
	}
	shell, err := exec.LookPath("pwsh.exe")
	if err != nil {
		return "", err
	}
	args := []string{"-NoLogo", "-NoProfile"}
	if agent != "powershell" {
		// Fixed script; the target is passed separately as an argument, not interpolated as code.
		args = append(args, "-CommandWithArgs", "& $args[0]; exit $LASTEXITCODE", target)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return a.launch(shell, args, filepath.Clean(dir))
}

// ResumeSession starts the chosen historical session in its original workspace.
// Terminals are independent; callers open one per tab without closing others.
func (a *App) ResumeSession(agentName, sessionID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	spec, err := agents.ResumeSpec(home, agentName, sessionID)
	if err != nil {
		return "", err
	}
	shell, err := exec.LookPath("pwsh.exe")
	if err != nil {
		return "", err
	}
	args := append([]string{"-NoLogo", "-NoProfile", "-CommandWithArgs", "& $args[0] $args[1..($args.Count-1)]; exit $LASTEXITCODE"},
		append([]string{spec.Executable}, spec.Args...)...)
	return a.launch(shell, args, spec.Dir)
}

// ListSessions returns Claude and Codex sessions across all workspaces, newest first.
func (a *App) ListSessions() []agents.Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return []agents.Session{}
	}
	return agents.List(home)
}

func openInWindowsTerminal(wtPath, dir string) error {
	if dir == "" {
		return errors.New("directory path cannot be empty")
	}
	clean := filepath.Clean(dir)
	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("invalid workspace directory: %s", dir)
	}
	if wtPath == "" {
		var err error
		wtPath, err = exec.LookPath("wt.exe")
		if err != nil {
			return errors.New("Windows Terminal (wt.exe) not found")
		}
	}
	cmd := exec.Command(wtPath, "-d", clean)
	return cmd.Start()
}

// OpenInWindowsTerminal launches Windows Terminal at the specified workspace directory.
func (a *App) OpenInWindowsTerminal(dir string) error {
	return openInWindowsTerminal("", dir)
}

func (a *App) launch(executable string, args []string, dir string) (string, error) {
	a.mu.Lock()
	if len(a.sessions) >= maxTerminals {
		a.mu.Unlock()
		return "", fmt.Errorf("terminal limit reached (%d)", maxTerminals)
	}
	id := "t" + strconv.FormatInt(a.terminal.Add(1), 10)
	a.mu.Unlock()
	s, err := terminal.Start(executable, args, dir, func(data []byte) {
		runtime.EventsEmit(a.ctx, "terminal:output", map[string]string{"id": id, "data": base64.StdEncoding.EncodeToString(data)})
	})
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	a.sessions[id] = s
	a.mu.Unlock()
	go func() {
		waitErr := s.Wait()
		message := ""
		if waitErr != nil {
			message = waitErr.Error()
		}
		a.mu.Lock()
		delete(a.sessions, id)
		a.mu.Unlock()
		runtime.EventsEmit(a.ctx, "terminal:exit", map[string]string{"id": id, "error": message})
	}()
	return id, nil
}

func (a *App) WriteTerminal(id, data string) error {
	a.mu.Lock()
	s := a.sessions[id]
	a.mu.Unlock()
	if s == nil {
		return errors.New("terminal not found")
	}
	return s.Write([]byte(data))
}
func (a *App) ResizeTerminal(id string, cols, rows int) error {
	a.mu.Lock()
	s := a.sessions[id]
	a.mu.Unlock()
	if s == nil {
		return errors.New("terminal not found")
	}
	return s.Resize(cols, rows)
}
func (a *App) CloseTerminal(id string) error {
	a.mu.Lock()
	s := a.sessions[id]
	a.mu.Unlock()
	if s == nil {
		return errors.New("terminal not found")
	}
	return s.Close()
}
func (a *App) beforeClose(ctx context.Context) bool {
	if a.configurationBusy.Load() || a.installBusy.Load() {
		return true
	}
	if a.exiting.Load() {
		return false
	}
	a.mu.Lock()
	count := len(a.sessions)
	a.mu.Unlock()
	if count == 0 {
		return false
	}
	// Native dialogs return platform-dependent answers (L02) and once trapped the
	// window for good; the confirm UI lives in the frontend instead.
	runtime.EventsEmit(ctx, "app:close-requested", count)
	return true
}

// ExitApp terminates the app after the frontend confirmed closing running terminals.
func (a *App) ExitApp() {
	if a.configurationBusy.Load() || a.installBusy.Load() {
		return
	}
	a.exiting.Store(true)
	runtime.Quit(a.ctx)
}
func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	sessions := make([]*terminal.Session, 0, len(a.sessions))
	for _, s := range a.sessions {
		sessions = append(sessions, s)
	}
	a.mu.Unlock()
	var wg sync.WaitGroup
	for _, s := range sessions {
		wg.Add(1)
		go func(s *terminal.Session) { defer wg.Done(); _ = s.Close() }(s)
	}
	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	// pty.Close kills the child before draining readers, but the drain can block on
	// event delivery to an already-quit message loop; never let that stall the exit.
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
	}
}

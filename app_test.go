package main

import (
	"os/exec"
	"testing"
)

func TestResolveRejectsUnknownAgent(t *testing.T) {
	if _, err := resolve("codex; whoami"); err == nil {
		t.Fatal("unknown agent accepted")
	}
}

func TestAgentShimArgumentForwarding(t *testing.T) {
	shell, err := exec.LookPath("pwsh.exe")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude", "codex"} {
		t.Run(name, func(t *testing.T) {
			target, err := resolve(name)
			if err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command(shell, "-NoLogo", "-NoProfile", "-CommandWithArgs", "& $args[0] --version; exit $LASTEXITCODE", target).CombinedOutput()
			if err != nil {
				t.Fatalf("%v: %s", err, output)
			}
			if len(output) == 0 {
				t.Fatal("empty version output")
			}
		})
	}
}

func TestOpenInWindowsTerminalRejectsInvalidDir(t *testing.T) {
	if err := openInWindowsTerminal("", ""); err == nil {
		t.Fatal("expected error for empty directory")
	}
	if err := openInWindowsTerminal("", "C:\\nonexistent_workspace_12345"); err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestOpenInWindowsTerminalValidatesDir(t *testing.T) {
	tempDir := t.TempDir()
	err := openInWindowsTerminal("invalid_wt_exec_that_does_not_exist.exe", tempDir)
	if err == nil {
		t.Fatal("expected exec error")
	}
	if err.Error() == "directory path cannot be empty" || err.Error() == "invalid workspace directory: "+tempDir {
		t.Fatalf("unexpected dir validation error: %v", err)
	}
}

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func runnerFile(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveCommandArgv(t *testing.T) {
	root := t.TempDir()
	runnerFile(t, root, "npm.cmd")
	node := runnerFile(t, root, "node.exe")
	cli := runnerFile(t, root, "node_modules/npm/bin/npm-cli.js")
	winget := runnerFile(t, root, "winget.exe")
	t.Setenv("PATH", root)
	args := []string{"install", "-g", "@scope/pkg & echo unsafe", "with space", `a"b`}
	path, got, err := resolveCommand("npm", args...)
	if err != nil || path != node || !reflect.DeepEqual(got, append([]string{cli}, args...)) {
		t.Fatalf("npm: path=%q args=%q err=%v", path, got, err)
	}
	path, got, err = resolveCommand("winget", args...)
	if err != nil || path != winget || !reflect.DeepEqual(got, args) {
		t.Fatalf("winget: path=%q args=%q err=%v", path, got, err)
	}
	if err := os.Remove(cli); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveCommand("npm", "--version"); err == nil || !strings.Contains(err.Error(), "npm CLI 不可用") || !strings.Contains(err.Error(), "npm-cli.js") {
		t.Fatalf("missing CLI must fail explicitly: %v", err)
	}
}

func TestResolveCommandMissingAndFallback(t *testing.T) {
	root, nodes := t.TempDir(), t.TempDir()
	t.Setenv("PATH", root)
	if _, _, err := resolveCommand("npm"); err == nil {
		t.Fatal("missing shim accepted")
	}
	runnerFile(t, root, "npm.cmd")
	runnerFile(t, root, "node_modules/npm/bin/npm-cli.js")
	if _, _, err := resolveCommand("npm"); err == nil || !strings.Contains(err.Error(), "node.exe") {
		t.Fatalf("missing node accepted: %v", err)
	}
	node := runnerFile(t, nodes, "node.exe")
	t.Setenv("PATH", root+string(os.PathListSeparator)+nodes)
	path, _, err := resolveCommand("npm")
	if err != nil || path != node {
		t.Fatalf("PATH node fallback: %q %v", path, err)
	}
	if _, _, err := resolveCommand("cmd.exe"); err == nil {
		t.Fatal("unsupported shell accepted")
	}
	if _, _, err := (commandInstaller{}).Run(nil); err == nil {
		t.Fatal("empty command accepted")
	}
}

func TestInstallOutputTail(t *testing.T) {
	b := &installOutputTail{}
	for _, chunk := range []string{strings.Repeat("a", installOutputLimit+10), "middle", strings.Repeat("b", installOutputLimit-2), "END"} {
		if n, err := b.Write([]byte(chunk)); n != len(chunk) || err != nil {
			t.Fatalf("write: %d %v", n, err)
		}
		if len(b.data) > installOutputLimit {
			t.Fatalf("output exceeds limit: %d", len(b.data))
		}
	}
	if want := strings.Repeat("b", installOutputLimit-3) + "END"; string(b.data) != want {
		t.Fatal("did not retain output tail")
	}
}

func TestInstallRunnerChild(t *testing.T) {
	switch os.Getenv("SHUTTLE_RUNNER_TEST_CHILD") {
	case "fail":
		fmt.Print(strings.Repeat("x", installOutputLimit*2))
		fmt.Fprint(os.Stderr, "decisive error tail")
		os.Exit(7)
	case "wait":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
}

func TestInstallRunnerFailureAndTimeout(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"fail", "wait"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("SHUTTLE_RUNNER_TEST_CHILD", mode)
			limit := 5 * time.Second
			if mode == "wait" {
				limit = 200 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), limit)
			defer cancel()
			started := time.Now()
			code, output, err := runInstallCommand(ctx, exe, []string{"-test.run=^TestInstallRunnerChild$"})
			if err == nil || len(output) > installOutputLimit {
				t.Fatalf("code=%d output=%d err=%v", code, len(output), err)
			}
			if mode == "fail" && (code != 7 || !strings.HasSuffix(output, "decisive error tail") || !strings.Contains(err.Error(), "decisive error tail")) {
				t.Fatalf("lost exit code or tail: code=%d err=%v", code, err)
			}
			if mode == "wait" && (!errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 4*time.Second) {
				t.Fatalf("timeout not bounded: %v", err)
			}
		})
	}
}

func TestInstallRunnerNPMSmoke(t *testing.T) {
	path, args, err := resolveCommand("npm", "--version")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	code, output, err := runInstallCommand(ctx, path, args)
	if err != nil || code != 0 || output == "" {
		t.Fatalf("npm --version: code=%d output=%q err=%v", code, output, err)
	}
	t.Logf("%s %q => %s", path, args, output)
}

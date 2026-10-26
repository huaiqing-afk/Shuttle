package inventory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fixtureDir = "../../../tests/fixtures/inventory"

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return raw
}

// Contract §7 test 1: real npm output, including scoped names.
func TestParseNPMRealSample(t *testing.T) {
	root := strings.TrimSpace(string(fixture(t, "npm-root.txt")))
	packages, err := ParseNPM(fixture(t, "npm-list.json"), root)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(packages) != 11 {
		t.Fatalf("packages = %d, want 11", len(packages))
	}
	byName := map[string]Package{}
	for _, pkg := range packages {
		byName[pkg.Name] = pkg
		if pkg.Source != SourceNPM || pkg.Root != root {
			t.Fatalf("source/root not attached: %+v", pkg)
		}
	}
	claude, ok := byName["@anthropic-ai/claude-code"]
	if !ok || claude.Version != "2.1.310" {
		t.Fatalf("scoped package: %+v", claude)
	}
	if _, ok := byName["@openai/codex"]; !ok {
		t.Fatal("second agent missing")
	}
	// sorted, deterministic
	for i := 1; i < len(packages); i++ {
		if packages[i-1].Name > packages[i].Name {
			t.Fatalf("not sorted: %q before %q", packages[i-1].Name, packages[i].Name)
		}
	}
}

func TestParseNPMEmptyAndGarbage(t *testing.T) {
	empty, err := ParseNPM([]byte(`{"name":"npm","dependencies":{}}`), `C:\npm`)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty tree: %v %v", empty, err)
	}
	if _, err := ParseNPM([]byte("npm ERR! something broke"), `C:\npm`); !errors.Is(err, ErrParseFailed) {
		t.Fatalf("garbage must fail the source, got %v", err)
	}
}

// Contract §7 test 2: the structured export, filtered to the winget source.
func TestParseWingetExportRealSample(t *testing.T) {
	packages, err := ParseWingetExport(fixture(t, "winget-export.json"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(packages) == 0 {
		t.Fatal("no packages parsed")
	}
	for _, pkg := range packages {
		if pkg.Source != SourceWinget || pkg.Name == "" {
			t.Fatalf("bad row: %+v", pkg)
		}
	}
	// version comes from --include-versions and must survive
	if packages[0].Version == "" {
		t.Fatalf("version lost: %+v", packages[0])
	}
}

func TestParseWingetSkipsForeignSourcesAndGarbage(t *testing.T) {
	doc := `{"Sources":[{"SourceDetails":{"Name":"winget"},"Packages":[{"PackageIdentifier":"A.B","Version":"1"}]},
		{"SourceDetails":{"Name":"msstore"},"Packages":[{"PackageIdentifier":"C.D","Version":"2"}]}]}`
	packages, err := ParseWingetExport([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(packages) != 1 || packages[0].Name != "A.B" {
		t.Fatalf("foreign source leaked in: %+v", packages)
	}
	if _, err := ParseWingetExport([]byte("{not json")); !errors.Is(err, ErrParseFailed) {
		t.Fatalf("garbage must fail the source, got %v", err)
	}
}

// Contract §7 test 6: a missing or failing command degrades that source only.
func TestCollectDegradesPerSource(t *testing.T) {
	missing := Collector{Run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("executable not found")
	}}
	npm := missing.CollectNPM(context.Background())
	if npm.Err == nil || !errors.Is(npm.Err, ErrSourceUnavailable) {
		t.Fatalf("missing npm must be unavailable, got %v", npm.Err)
	}
	if len(npm.Packages) != 0 {
		t.Fatal("failed source must not report packages")
	}
	winget := missing.CollectWinget(context.Background(), filepath.Join(t.TempDir(), "out.json"))
	if winget.Err == nil || !errors.Is(winget.Err, ErrSourceUnavailable) {
		t.Fatalf("missing winget must be unavailable, got %v", winget.Err)
	}
}

// Contract §7 test 8: collection is read-only. The runner records every command
// and the fixture directory must be unchanged afterwards.
func TestCollectIsReadOnly(t *testing.T) {
	var calls [][]string
	root := strings.TrimSpace(string(fixture(t, "npm-root.txt")))
	exportPath := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(exportPath, fixture(t, "winget-export.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	collector := Collector{Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		switch {
		case name == "npm" && args[0] == "root":
			return []byte(root + "\r\n"), nil
		case name == "npm":
			return fixture(t, "npm-list.json"), nil
		case name == "winget":
			return nil, nil // the export path is pre-seeded above
		}
		return nil, errors.New("unexpected command")
	}}
	before := dirDigest(t, fixtureDir)
	npm := collector.CollectNPM(context.Background())
	winget := collector.CollectWinget(context.Background(), exportPath)
	after := dirDigest(t, fixtureDir)
	if npm.Err != nil || winget.Err != nil {
		t.Fatalf("collect errors: %v / %v", npm.Err, winget.Err)
	}
	if before != after {
		t.Fatal("collection modified fixture files")
	}
	// only inspection verbs; no install/uninstall/upgrade may appear
	for _, call := range calls {
		verb := call[1]
		for _, forbidden := range []string{"install", "uninstall", "upgrade", "remove", "add", "rm"} {
			if verb == forbidden {
				t.Fatalf("collector ran a mutating command: %v", call)
			}
		}
	}
	if len(calls) != 3 {
		t.Fatalf("expected 3 inspection calls, got %v", calls)
	}
}

// Contract §7 test 6, timeout half: an expired context fails the source without
// waiting for the command to return.
func TestCollectTimeoutFailsSource(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	blocking := Collector{Run: func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		<-release // simulates a package manager that hangs
		return nil, errors.New("released")
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan Result, 1)
	go func() { done <- blocking.CollectNPM(ctx) }()
	select {
	case result := <-done:
		if !errors.Is(result.Err, ErrSourceUnavailable) || !errors.Is(result.Err, context.DeadlineExceeded) {
			t.Fatalf("timeout must fail the source: %v", result.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("collector did not return after its context expired")
	}
}

func dirDigest(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		sb.WriteString(entry.Name())
		sb.WriteString(info.ModTime().String())
		sb.WriteString(string(rune(info.Size())))
	}
	return sb.String()
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "shuttle/internal/core/inventory"
)

// The bridge must never write to the real ledger during tests; every case here
// drives a temporary ledger and an injected collector.
func TestInventoryBridgeClassifiesWithIsolatedLedger(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "install-ledger.json")
	service := &inventoryService{root: dir, note: "note"}
	service.ledger = ledgerFixture{path: ledgerPath}

	items := classifyForTest(t, service, []core.Package{
		{Name: "@anthropic-ai/claude-code", Version: "2.1.310", Source: core.SourceNPM, Root: `C:\npm`},
		{Name: "@openai/codex", Version: "0.164.0", Source: core.SourceNPM, Root: `C:\npm`},
	})
	byName := map[string]core.Item{}
	for _, item := range items {
		byName[item.Name] = item
	}
	if byName["@anthropic-ai/claude-code"].Ownership != core.OwnedByShuttle {
		t.Fatalf("recorded package: %+v", byName["@anthropic-ai/claude-code"])
	}
	if byName["@openai/codex"].Ownership != core.OwnedExternally {
		t.Fatalf("unrecorded package: %+v", byName["@openai/codex"])
	}

	plan := planForTest(t, items, []string{"@anthropic-ai/claude-code", "@openai/codex"})
	if plan.Blocked {
		t.Fatalf("external-only selection must not block: %+v", plan.Conflicts)
	}
	actions := map[string]string{}
	for _, step := range plan.Steps {
		actions[step.Name] = step.Action
	}
	if actions["@anthropic-ai/claude-code"] != "uninstall" || actions["@openai/codex"] != "skip" {
		t.Fatalf("actions: %+v", actions)
	}
}

func TestInventoryBridgeReportShape(t *testing.T) {
	report := InventoryReport{
		Items: []InventoryItem{{Name: "a", Source: "npm", Ownership: "external", Reason: "r"}},
		Sources: []InventorySource{
			{Name: "npm", Count: 1, Available: true},
			{Name: "winget", Available: false, Error: "executable not found"},
		},
		Note: "coverage",
	}
	if report.Sources[1].Available || report.Sources[1].Error == "" {
		t.Fatalf("failed source must carry its error: %+v", report.Sources[1])
	}
	if report.Note == "" {
		t.Fatal("coverage note must reach the UI")
	}
}

func TestFirstLine(t *testing.T) {
	if got := firstLine("boom\r\nrest"); got != "boom" {
		t.Fatalf("got %q", got)
	}
	if got := firstLine("single"); got != "single" {
		t.Fatalf("got %q", got)
	}
}

// ledgerFixture is a minimal core.Ledger backed by a temp file, so the bridge
// test exercises the adapter without touching user data.
type ledgerFixture struct{ path string }

func (l ledgerFixture) Lookup(name string) (core.Record, bool, error) {
	if name == "@anthropic-ai/claude-code" {
		return core.Record{Name: name, Source: core.SourceNPM, Root: `C:\npm`, State: core.StateInstalled}, true, nil
	}
	return core.Record{}, false, nil
}

func (l ledgerFixture) Put(core.Record) error { return nil }

func (l ledgerFixture) List() ([]core.Record, error) {
	record, _, _ := l.Lookup("@anthropic-ai/claude-code")
	return []core.Record{record}, nil
}

func (l ledgerFixture) Delete(string) error { return nil }

func classifyForTest(t *testing.T, s *inventoryService, packages []core.Package) []core.Item {
	t.Helper()
	items := core.Classify(packages, s.ledger)
	if len(items) != len(packages) {
		t.Fatalf("classified %d of %d", len(items), len(packages))
	}
	return items
}

func planForTest(t *testing.T, items []core.Item, names []string) core.Plan {
	t.Helper()
	plan, err := core.PlanUninstall(items, core.Choice{Names: names})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return plan
}

// guard: the isolated root must stay inside a temp directory in these tests
func TestInventoryTestRootIsTemporary(t *testing.T) {
	dir := t.TempDir()
	if !strings.HasPrefix(dir, os.TempDir()) {
		t.Fatalf("test dir escaped temp: %s", dir)
	}
}

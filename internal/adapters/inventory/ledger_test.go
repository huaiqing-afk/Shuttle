package inventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	core "shuttle/internal/core/inventory"
)

func TestLedgerAbsentIsExternalNotUnknown(t *testing.T) {
	ledger := NewFileLedger(filepath.Join(t.TempDir(), "install-ledger.json"))
	_, ok, err := ledger.Lookup("@openai/codex")
	if err != nil || ok {
		t.Fatalf("absent ledger: ok=%v err=%v", ok, err)
	}
	items := core.Classify([]core.Package{{Name: "@openai/codex", Source: core.SourceNPM}}, ledger)
	if items[0].Ownership != core.OwnedExternally {
		t.Fatalf("absent ledger must be external: %+v", items[0])
	}
}

// A corrupt ledger means "we cannot tell", never "user installed it".
func TestLedgerCorruptIsUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install-ledger.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	items := core.Classify([]core.Package{{Name: "@openai/codex", Source: core.SourceNPM}}, NewFileLedger(path))
	if items[0].Ownership != core.OwnershipUnknown {
		t.Fatalf("corrupt ledger must be unknown: %+v", items[0])
	}
	if items[0].Reason == "" {
		t.Fatal("reason must explain the broken ledger")
	}
}

// Contract §10.8 test 9: v1 ledgers still read (state implies installed), and
// any version this build does not know is a broken ledger, not an empty one.
func TestLedgerSchemaCompatibility(t *testing.T) {
	dir := t.TempDir()
	v1 := filepath.Join(dir, "v1.json")
	if err := os.WriteFile(v1, []byte(`{"schemaVersion":1,"records":[
		{"Name":"@anthropic-ai/claude-code","Source":"npm","Root":"C:\\npm"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	record, ok, err := NewFileLedger(v1).Lookup("@anthropic-ai/claude-code")
	if err != nil || !ok {
		t.Fatalf("v1 ledger: ok=%v err=%v", ok, err)
	}
	if record.State != core.StateInstalled {
		t.Fatalf("v1 record must read as installed: %+v", record)
	}
	items := core.Classify([]core.Package{{Name: "@anthropic-ai/claude-code", Source: core.SourceNPM, Root: `C:\npm`}}, NewFileLedger(v1))
	if items[0].Ownership != core.OwnedByShuttle {
		t.Fatalf("v1 record must classify as managed: %+v", items[0])
	}

	for _, version := range []string{"0", "3", "99"} {
		path := filepath.Join(dir, "v"+version+".json")
		if err := os.WriteFile(path, []byte(`{"schemaVersion":`+version+`,"records":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		items := core.Classify([]core.Package{{Name: "x", Source: core.SourceNPM}}, NewFileLedger(path))
		if items[0].Ownership != core.OwnershipUnknown {
			t.Fatalf("schema %s must degrade to unknown: %+v", version, items[0])
		}
	}
}

// Saving writes the current schema even when the records came from a v1 file.
func TestLedgerSaveWritesCurrentSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install-ledger.json")
	ledger := NewFileLedger(path)
	record := core.Record{
		Name: "@anthropic-ai/claude-code", Source: core.SourceNPM, Root: `C:\npm`, Version: "2.1.310",
		State: core.StateInstalled, OperationID: "op-1", EvidenceBefore: "absent",
		EvidenceAfter: "2.1.310 @ C:\\npm", InstallerExit: 0, InstalledAt: "2026-10-25T04:00:00Z",
	}
	if err := ledger.Save([]core.Record{record}); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		SchemaVersion int
		Records       []core.Record
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != 2 {
		t.Fatalf("saved schema = %d, want 2: %s", doc.SchemaVersion, raw)
	}
	if len(doc.Records) != 1 || doc.Records[0].State != core.StateInstalled || doc.Records[0].OperationID != "op-1" {
		t.Fatalf("round trip lost fields: %+v", doc.Records)
	}
	// the stored shape is what a v2 reader sees, so lookup returns everything
	got, ok, err := NewFileLedger(path).Lookup("@anthropic-ai/claude-code")
	if err != nil || !ok || got.EvidenceBefore != "absent" || got.InstallerExit != 0 {
		t.Fatalf("reload: %+v ok=%v err=%v", got, ok, err)
	}
}

func TestLedgerSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "install-ledger.json")
	ledger := NewFileLedger(path)
	records := []core.Record{{Name: "@anthropic-ai/claude-code", Source: core.SourceNPM, Root: `C:\npm\node_modules`}}
	if err := ledger.Save(records); err != nil {
		t.Fatalf("save: %v", err)
	}
	reopened := NewFileLedger(path)
	record, ok, err := reopened.Lookup("@anthropic-ai/claude-code")
	if err != nil || !ok || record.Root != `C:\npm\node_modules` {
		t.Fatalf("reload: %+v ok=%v err=%v", record, ok, err)
	}
}

func TestLedgerDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install-ledger.json")
	ledger := NewFileLedger(path)
	records := []core.Record{
		{Name: "pkg-1", Source: core.SourceNPM, Root: `C:\npm`},
		{Name: "pkg-2", Source: core.SourceWinget, Root: "winget"},
	}
	if err := ledger.Save(records); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := ledger.Delete("pkg-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok, err := ledger.Lookup("pkg-1"); err != nil || ok {
		t.Fatalf("pkg-1 should be deleted: ok=%v err=%v", ok, err)
	}
	if _, ok, err := ledger.Lookup("pkg-2"); err != nil || !ok {
		t.Fatalf("pkg-2 should remain: ok=%v err=%v", ok, err)
	}
	if err := ledger.Delete("non-existent"); err != nil {
		t.Fatalf("delete nonexistent should succeed: %v", err)
	}
}

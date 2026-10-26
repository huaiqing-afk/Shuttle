package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	core "shuttle/internal/core/inventory"
)

// Contract §10.8 test 11: an intent that never finished must read back as
// unknown after a restart, never as "Shuttle installed it".
func TestIntentRecordStaysUnknownAfterCrash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install-ledger.json")
	intent := core.Record{
		Name: "@anthropic-ai/claude-code", Source: core.SourceNPM, Root: `C:\npm`,
		State: core.StateIntent, OperationID: "op-1", EvidenceBefore: "absent",
	}
	if err := NewFileLedger(path).Put(intent); err != nil {
		t.Fatalf("put intent: %v", err)
	}

	// the process dies here: the terminal state is never written
	detected := core.Package{Name: "@anthropic-ai/claude-code", Version: "2.1.310", Source: core.SourceNPM, Root: `C:\npm`}
	items := core.Classify([]core.Package{detected}, NewFileLedger(path))
	if items[0].Ownership != core.OwnershipUnknown {
		t.Fatalf("unfinished intent must be unknown: %+v", items[0])
	}
	if !strings.Contains(items[0].Reason, "意图") {
		t.Fatalf("reason must explain the unfinished intent: %q", items[0].Reason)
	}
	plan, err := core.PlanUninstall(items, core.Choice{Names: []string{detected.Name}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.Blocked || plan.Steps[0].Action != "skip" {
		t.Fatalf("unfinished intent must not be uninstallable: %+v", plan)
	}
}

// Contract §10.8 test 13: a write failure surfaces as an error and leaves no
// partial ledger behind. The caller (install operation) turns this into a
// failed operation; that wiring belongs to T11d-4.
func TestLedgerWriteFailureLeavesNothingPartial(t *testing.T) {
	dir := t.TempDir()
	// a regular file where the ledger directory should be: MkdirAll must fail
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	ledger := NewFileLedger(filepath.Join(blocked, "install-ledger.json"))
	err := ledger.Put(core.Record{Name: "pkg", Source: core.SourceNPM, State: core.StateIntent})
	if err == nil {
		t.Fatal("write into an unusable directory must fail")
	}
	if !strings.Contains(err.Error(), "台账") {
		t.Fatalf("error must name the ledger so the UI can explain it: %v", err)
	}
	if _, err := os.Stat(filepath.Join(blocked, "install-ledger.json")); !os.IsNotExist(err) {
		t.Fatalf("failed write must not create a ledger: %v", err)
	}
	assertNoTempFiles(t, dir)
}

// A ledger we cannot read must never be replaced by a fresh one-record file:
// that would silently destroy every recorded install.
func TestPutRefusesToOverwriteUnreadableLedger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "install-ledger.json")
	original := []byte(`{"schemaVersion":99,"records":[]}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	err := NewFileLedger(path).Put(core.Record{Name: "pkg", Source: core.SourceNPM, State: core.StateInstalled})
	if err == nil {
		t.Fatal("put over an unsupported ledger must fail")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("unreadable ledger was overwritten: %s", got)
	}
	assertNoTempFiles(t, dir)
}

// Contract §10.8 test 14 and D4: concurrent writers in one process lose nothing,
// and the ledger keeps one current record per package.
func TestConcurrentPutsKeepEveryRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install-ledger.json")
	ledger := NewFileLedger(path)
	const writers = 20

	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			record := core.Record{
				Name: fmt.Sprintf("pkg-%02d", i), Source: core.SourceNPM,
				State: core.StateInstalled, OperationID: fmt.Sprintf("op-%02d", i),
			}
			if err := ledger.Put(record); err != nil {
				t.Errorf("put %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < writers; i++ {
		name := fmt.Sprintf("pkg-%02d", i)
		record, ok, err := NewFileLedger(path).Lookup(name)
		if err != nil || !ok {
			t.Fatalf("%s lost: ok=%v err=%v", name, ok, err)
		}
		if record.OperationID != fmt.Sprintf("op-%02d", i) {
			t.Fatalf("%s kept the wrong record: %+v", name, record)
		}
	}
	assertNoTempFiles(t, filepath.Dir(path))

	// D4: a second Put for the same package replaces the state, never appends
	if err := ledger.Put(core.Record{Name: "pkg-00", Source: core.SourceNPM, State: core.StateFailed}); err != nil {
		t.Fatalf("put failed state: %v", err)
	}
	record, ok, err := NewFileLedger(path).Lookup("pkg-00")
	if err != nil || !ok || record.State != core.StateFailed {
		t.Fatalf("state transition not applied: %+v ok=%v err=%v", record, ok, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), `"pkg-00"`) != 1 {
		t.Fatalf("ledger kept a history instead of one current record: %s", raw)
	}
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".ledger-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary ledger files left behind: %v", matches)
	}
}

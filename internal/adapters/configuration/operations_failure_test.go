package configuration

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	core "shuttle/internal/core/configuration"
	"strings"
	"testing"
)

// failingStore delegates to the real file store but refuses to persist one
// target, so the orchestration sees a genuine disk error mid-operation.
type failingStore struct {
	*FileOperationStore
	badTarget string
}

func (s *failingStore) Save(id, target, o, a string, absent bool) error {
	if target == s.badTarget {
		return errors.New("注入的落盘失败")
	}
	return s.FileOperationStore.Save(id, target, o, a, absent)
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return settings
}

// A disk failure on the operation record stops the operation, leaves earlier
// writes in place, and is durably reported when the record is reopened.
func TestPersistenceFailureMidOperation(t *testing.T) {
	root := t.TempDir()
	wtPath := filepath.Join(root, "settings.json")
	original := []byte("{\n    \"theme\": \"system\",\n    \"unrelated\": true\n}\n")
	if err := os.WriteFile(wtPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store := &failingStore{FileOperationStore: NewFileOperationStore(filepath.Join(root, "operations")),
		badTarget: "wt:scalar:theme"}
	ed := NewWTEditor(wtPath)
	choice := core.ConfigChoice{SchemaVersion: 1, WT: core.WTChoice{
		DefaultProfileGUID: "{61c54bbd-c2c6-5271-96e7-009a87ff44bf}", Theme: "dark"}}
	plan, err := core.Plan(choice, ed)
	if err != nil || plan.Blocked {
		t.Fatalf("plan: %+v %v", plan, err)
	}

	result, err := core.ExecuteRecorded(plan, ed, store)
	if err != nil {
		t.Fatalf("per-target failure must not be fatal: %v", err)
	}
	if result.Succeeded {
		t.Fatal("partial failure reported as success")
	}
	// defaultProfile sorts first and was written; the refused target was not
	if got := readJSON(t, wtPath)["defaultProfile"]; got != "{61c54bbd-c2c6-5271-96e7-009a87ff44bf}" {
		t.Fatalf("earlier write lost: %v", got)
	}
	if got := readJSON(t, wtPath)["theme"]; got != "system" {
		t.Fatalf("refused target written: %v", got)
	}
	reopened := NewFileOperationStore(store.Root)
	op, err := reopened.Get(result.OpID)
	if err != nil || op.State != "partially_failed" {
		t.Fatalf("record: %+v %v", op, err)
	}
	if len(op.Steps) != 2 || op.Steps[1].Err == "" || !strings.Contains(op.Steps[1].Err, "注入的落盘失败") {
		t.Fatalf("persisted steps: %+v", op.Steps)
	}
	// the written scalar is restorable without touching the refused target
	restore, err := core.PlanRecordedRestore(op.ID, ed, reopened)
	if err != nil || restore.Blocked || restore.Diffs[0].Action != "remove" {
		t.Fatalf("restore plan: %+v %v", restore, err)
	}
}

// If the operation record itself cannot be created nothing may be written.
func TestUnwritableStoreWritesNothing(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "operations") // a file, so MkdirAll must fail
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(root, "settings.json")
	original := []byte("{\n    \"theme\": \"system\"\n}\n")
	if err := os.WriteFile(wtPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	ed := NewWTEditor(wtPath)
	plan, err := core.Plan(core.ConfigChoice{SchemaVersion: 1, WT: core.WTChoice{Theme: "dark"}}, ed)
	if err != nil || plan.Blocked {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	if _, err := core.ExecuteRecorded(plan, ed, NewFileOperationStore(blocked)); err == nil {
		t.Fatal("unwritable record store must abort the operation")
	}
	if raw, err := os.ReadFile(wtPath); err != nil || string(raw) != string(original) {
		t.Fatalf("config touched without a durable record: %q %v", raw, err)
	}
}

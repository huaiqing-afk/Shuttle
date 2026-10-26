package configuration

import (
	"os"
	"path/filepath"
	core "shuttle/internal/core/configuration"
	"testing"
	"time"
)

func TestRecordedApplyRestoreAndReopen(t *testing.T) {
	root := t.TempDir()
	store := NewFileOperationStore(filepath.Join(root, "operations"))
	ed := NewPSEditor(filepath.Join(root, "profile.ps1"))
	plan, err := core.Plan(core.ConfigChoice{SchemaVersion: 1, PS: core.PSCheck{SessionEntry: true}}, ed)
	if err != nil || plan.Blocked {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	result, err := core.ExecuteRecorded(plan, ed, store)
	if err != nil || !result.Succeeded {
		t.Fatalf("execute: %+v %v", result, err)
	}
	reopened := NewFileOperationStore(store.Root)
	op, err := reopened.Get(result.OpID)
	if err != nil || op.State != "succeeded" {
		t.Fatalf("record: %+v %v", op, err)
	}
	restore, err := core.PlanRecordedRestore(op.ID, ed, reopened)
	if err != nil || restore.Blocked || restore.Diffs[0].Action != "remove" {
		t.Fatalf("restore: %+v %v", restore, err)
	}
	undone, err := core.ExecuteRecorded(restore, ed, reopened)
	if err != nil || !undone.Succeeded {
		t.Fatalf("undo: %+v %v", undone, err)
	}
	again, err := core.PlanRecordedRestore(op.ID, ed, reopened)
	if err != nil || again.Diffs[0].Action != "none" {
		t.Fatalf("repeat restore: %+v %v", again, err)
	}
}

func TestInterruptedCheckpointsAndUserConflict(t *testing.T) {
	for _, written := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-write", true: "after-write"}[written], func(t *testing.T) {
			root := t.TempDir()
			store := NewFileOperationStore(filepath.Join(root, "operations"))
			ed := NewPSEditor(filepath.Join(root, "profile.ps1"))
			id := "op-crash"
			if err := store.Begin(core.Operation{ID: id, SchemaVersion: 1, State: "running", StartedAt: time.Now()}); err != nil {
				t.Fatal(err)
			}
			if err := store.Save(id, psTarget, "", core.ProfileBlock, true); err != nil {
				t.Fatal(err)
			}
			if written {
				if _, _, err := ed.Apply(psTarget, core.ProfileBlock); err != nil {
					t.Fatal(err)
				}
			}
			reopened := NewFileOperationStore(store.Root)
			if err := reopened.RecoverInterrupted(); err != nil {
				t.Fatal(err)
			}
			op, err := reopened.Get(id)
			if err != nil || op.State != "interrupted" || op.Steps[0].Action != "pending" {
				t.Fatalf("recovery: %+v %v", op, err)
			}
			plan, err := core.PlanRecordedRestore(id, ed, reopened)
			if err != nil || plan.Blocked {
				t.Fatalf("plan: %+v %v", plan, err)
			}
			want := "none"
			if written {
				want = "remove"
			}
			if plan.Diffs[0].Action != want {
				t.Fatalf("got %+v", plan)
			}
			custom := core.ProfileBlockStart + "\n# user edit\n" + core.ProfileBlockEnd
			if _, _, err := ed.Apply(psTarget, custom); err != nil {
				t.Fatal(err)
			}
			plan, err = core.PlanRecordedRestore(id, ed, reopened)
			if err != nil || !plan.Blocked {
				t.Fatalf("user conflict: %+v %v", plan, err)
			}
			if _, err := core.ExecuteRecorded(plan, ed, reopened); err == nil {
				t.Fatal("conflict executed")
			}
			if value, err := ed.ReadCurrent(psTarget); err != nil || value != custom {
				t.Fatal("user edit lost")
			}
		})
	}
}

func TestStoreRejectsTraversalAndCorruption(t *testing.T) {
	store := NewFileOperationStore(t.TempDir())
	if _, err := store.Get("../outside"); err == nil {
		t.Fatal("accepted traversal")
	}
	if err := store.Begin(core.Operation{ID: "op-bad", SchemaVersion: 1, State: "running"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Root, "op-bad", "record.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.RecoverInterrupted(); err == nil {
		t.Fatal("corruption silently ignored")
	}
}

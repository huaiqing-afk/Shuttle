package configuration

import (
	"fmt"
	"strings"
	"testing"
)

// fakeOps adds the operation-record surface on top of memStore and can fail
// each persistence step independently.
type fakeOps struct {
	*memStore
	op         Operation
	finished   string
	finalSteps []StepResult
	recorded   []StepResult
	failSave   map[string]bool
	failRecord bool
	failFinish bool
}

func newFakeOps() *fakeOps { return &fakeOps{memStore: newMemStore(), failSave: map[string]bool{}} }

func (f *fakeOps) Lock() (func(), error)            { return func() {}, nil }
func (f *fakeOps) Begin(op Operation) error         { f.op = op; return nil }
func (f *fakeOps) Get(id string) (Operation, error) { return f.op, nil }
func (f *fakeOps) List() ([]Operation, error)       { return []Operation{f.op}, nil }
func (f *fakeOps) RecoverInterrupted() error        { return nil }

func (f *fakeOps) Save(opID, target, o, a string, oAbsent bool) error {
	if f.failSave[target] {
		return fmt.Errorf("注入的落盘失败")
	}
	return f.memStore.Save(opID, target, o, a, oAbsent)
}

func (f *fakeOps) RecordStep(_ string, step StepResult) error {
	if f.failRecord {
		return fmt.Errorf("注入的记录失败")
	}
	f.recorded = append(f.recorded, step)
	return nil
}

func (f *fakeOps) Finish(_ string, state string, steps []StepResult) error {
	f.finished, f.finalSteps = state, steps
	if f.failFinish {
		return fmt.Errorf("注入的收尾失败")
	}
	return nil
}

// applyFails refuses one target at write time; everything before it succeeds.
type applyFails struct {
	*memEditor
	badTarget string
}

func (e *applyFails) Apply(target, value string) (string, bool, error) {
	if target == e.badTarget {
		return "", false, fmt.Errorf("注入的写入失败")
	}
	return e.memEditor.Apply(target, value)
}

// writesElsewhere pretends to succeed but stores a different value, so the
// write-then-read verification must catch it.
type writesElsewhere struct{ *memEditor }

func (e *writesElsewhere) Apply(target, value string) (string, bool, error) {
	return e.memEditor.Apply(target, value+",corrupted")
}

func steppingPlan(t *testing.T) ChangePlan {
	t.Helper()
	ed := newMemEditor()
	plan, err := Plan(sampleChoice(), ed)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return plan
}

// A failed write stops the operation, keeps the earlier writes, and leaves a
// record the user can restore from.
func TestWriteFailureKeepsEarlierWritesAndRecords(t *testing.T) {
	ed := &applyFails{memEditor: newMemEditor(), badTarget: "wt:scalar:theme"}
	store := newFakeOps()
	result, err := ExecuteRecorded(steppingPlan(t), ed, store)
	if err == nil || !strings.Contains(err.Error(), "注入的写入失败") {
		t.Fatalf("write error must surface, got %v", err)
	}
	if result.Succeeded {
		t.Fatal("failed write must not report success")
	}
	if store.finished != "partially_failed" {
		t.Fatalf("state = %q, want partially_failed", store.finished)
	}
	// backups are saved before every write, including the one that fails, so
	// the three earlier targets plus the failed target are all recorded
	if got := len(store.Targets(store.op.ID)); got != 4 {
		t.Fatalf("backups = %d, want 4", got)
	}
	last := store.recorded[len(store.recorded)-1]
	if last.Target != "wt:scalar:theme" || !strings.Contains(last.Err, "注入的写入失败") {
		t.Fatalf("failed step not recorded: %+v", last)
	}
	if got := ed.values["wt:scalar:theme"]; got != "" {
		t.Fatalf("failed target must stay untouched, got %q", got)
	}
	// the three completed writes are still restorable; the failed target stayed
	// absent, so its backed-up row is a no-op
	plan, err := PlanRecordedRestore(store.op.ID, ed, store)
	if err != nil || plan.Blocked {
		t.Fatalf("restore plan: %+v %v", plan, err)
	}
	actionable := 0
	for _, diff := range plan.Diffs {
		if diff.Action != "none" {
			actionable++
		}
	}
	if actionable != 3 {
		t.Fatalf("actionable restore rows = %d, want 3: %+v", actionable, plan.Diffs)
	}
}

// A backup that cannot be persisted must stop that target before its write.
func TestBackupFailureSkipsWrite(t *testing.T) {
	ed := newMemEditor()
	store := newFakeOps()
	store.failSave["wt:scalar:defaultProfile"] = true
	result, err := ExecuteRecorded(steppingPlan(t), ed, store)
	if err != nil {
		t.Fatalf("backup failure is per-step, not fatal: %v", err)
	}
	if result.Succeeded {
		t.Fatal("partial failure must not report success")
	}
	if store.finished != "partially_failed" {
		t.Fatalf("state = %q, want partially_failed", store.finished)
	}
	var step StepResult
	for _, s := range result.Steps {
		if s.Target == "wt:scalar:defaultProfile" {
			step = s
		}
	}
	if !strings.Contains(step.Err, "备份失败") {
		t.Fatalf("step must carry the backup error: %+v", step)
	}
	if _, ok := ed.values["wt:scalar:defaultProfile"]; ok {
		t.Fatal("target without a backup must not be written")
	}
	// later targets still proceed
	if _, ok := ed.values["wt:scalar:theme"]; !ok {
		t.Fatal("unrelated target must still be written")
	}
}

// Losing the step record aborts the operation: an unrecorded write cannot be
// restored, so continuing would silently strand the user's config.
func TestRecordStepFailureAborts(t *testing.T) {
	ed := newMemEditor()
	store := newFakeOps()
	store.failRecord = true
	result, err := ExecuteRecorded(steppingPlan(t), ed, store)
	if err == nil || !strings.Contains(err.Error(), "注入的记录失败") {
		t.Fatalf("record error must surface, got %v", err)
	}
	// the write landed before its record failed, so the operation is partly applied
	if result.Succeeded || store.finished != "partially_failed" {
		t.Fatalf("result=%+v state=%q", result, store.finished)
	}
	if backups := store.targets[store.op.ID]; len(backups) != 1 {
		t.Fatalf("operation must stop after the first unrecorded write, backups=%v", backups)
	}
}

// A write that does not read back as the desired value is a failed write.
func TestWriteVerificationMismatchFails(t *testing.T) {
	ed := &writesElsewhere{memEditor: newMemEditor()}
	store := newFakeOps()
	result, err := ExecuteRecorded(steppingPlan(t), ed, store)
	if err == nil || !strings.Contains(err.Error(), "写后验证失败") {
		t.Fatalf("verification error must surface, got %v", err)
	}
	// Apply reported a change before verification failed, so this is a partial write
	if result.Succeeded || store.finished != "partially_failed" {
		t.Fatalf("result=%+v state=%q", result, store.finished)
	}
	if len(result.Steps) != 1 || result.Steps[0].Err == "" {
		t.Fatalf("steps: %+v", result.Steps)
	}
}

// If the final record cannot be saved the operation must not claim success.
func TestFinishFailureReportsFailure(t *testing.T) {
	ed := newMemEditor()
	store := newFakeOps()
	store.failFinish = true
	result, err := ExecuteRecorded(steppingPlan(t), ed, store)
	if err == nil || !strings.Contains(err.Error(), "注入的收尾失败") {
		t.Fatalf("finish error must surface, got %v", err)
	}
	if result.Succeeded {
		t.Fatal("unfinished operation must not report success")
	}
}

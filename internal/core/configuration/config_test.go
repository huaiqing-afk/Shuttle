package configuration

import (
	"errors"
	"fmt"
	"testing"
)

// memEditor stores target values; a missing key means the target is absent.
type memEditor struct {
	values map[string]string
}

func newMemEditor() *memEditor { return &memEditor{values: map[string]string{}} }

func (m *memEditor) ReadCurrent(target string) (string, error) {
	value, ok := m.values[target]
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

func (m *memEditor) Check(target string, desired string) error { return nil }

func (m *memEditor) Apply(target string, value string) (string, bool, error) {
	before, ok := m.values[target]
	m.values[target] = value
	return before, !ok || before != value, nil
}

// conflictEditor fails reads for one target with an adapter-reported conflict.
type conflictEditor struct {
	*memEditor
	badTarget string
}

func (c *conflictEditor) ReadCurrent(target string) (string, error) {
	return c.memEditor.ReadCurrent(target)
}

func (c *conflictEditor) Check(target string, desired string) error {
	if target == c.badTarget {
		return &Error{Kind: "conflict", Target: target, Reason: "同名 Profile 但 GUID 不同"}
	}
	return nil
}

func (c *conflictEditor) Apply(target string, value string) (string, bool, error) {
	if target == c.badTarget {
		return "", false, fmt.Errorf("blocked target %s", target)
	}
	return c.memEditor.Apply(target, value)
}

// memStore keeps insertion order per operation.
type memStore struct {
	targets map[string][]string
	rows    map[string]map[string][3]string // target -> {o, a, oAbsent}
}

func newMemStore() *memStore {
	return &memStore{targets: map[string][]string{}, rows: map[string]map[string][3]string{}}
}

func (s *memStore) Save(opID string, target string, o string, a string, oAbsent bool) error {
	if _, ok := s.rows[opID]; !ok {
		s.rows[opID] = map[string][3]string{}
	}
	if _, seen := s.rows[opID][target]; !seen {
		s.targets[opID] = append(s.targets[opID], target)
	}
	s.rows[opID][target] = [3]string{o, a, fmt.Sprintf("%t", oAbsent)}
	return nil
}

func (s *memStore) Load(opID string, target string) (string, string, bool, error) {
	row, ok := s.rows[opID][target]
	if !ok {
		return "", "", false, fmt.Errorf("no backup for %s", target)
	}
	return row[0], row[1], row[2] == "true", nil
}

func (s *memStore) Targets(opID string) []string { return s.targets[opID] }

func sampleChoice() ConfigChoice {
	return ConfigChoice{
		SchemaVersion: 1,
		WT: WTChoice{
			Theme:              "Shuttle Night",
			DefaultProfileGUID: "{61c54bbd-c2c6-5271-96e7-009a87ff44bf}",
			Params:             map[string]string{"profiles.defaults.fontFace": "Maple Mono NF CN"},
		},
		PS: PSCheck{SessionEntry: true},
	}
}

func targetOf(t *testing.T, plan ChangePlan, target string) FieldDiff {
	t.Helper()
	for _, diff := range plan.Diffs {
		if diff.Target == target {
			return diff
		}
	}
	t.Fatalf("target %q not in plan", target)
	return FieldDiff{}
}

func TestPlanActionsAndIdempotency(t *testing.T) {
	ed := newMemEditor()
	ed.values["wt:scalar:theme"] = "Campbell" // modify row

	plan, err := Plan(sampleChoice(), ed)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.Blocked {
		t.Fatal("clean choice must not block")
	}
	if got := targetOf(t, plan, "wt:scalar:theme"); got.Action != "modify" || got.Before != "Campbell" {
		t.Fatalf("theme row: %+v", got)
	}
	if got := targetOf(t, plan, "wt:scalar:defaultProfile"); got.Action != "add" || !got.WasAbsent {
		t.Fatalf("defaultProfile row: %+v", got)
	}
	if got := targetOf(t, plan, "ps:managed-block"); got.Action != "add" {
		t.Fatalf("ps row: %+v", got)
	}

	// execute then re-plan: every diff must be none (contract §4.3)
	if _, err := Execute(plan, ed, newMemStore()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	second, err := Plan(sampleChoice(), ed)
	if err != nil {
		t.Fatalf("second Plan: %v", err)
	}
	for _, diff := range second.Diffs {
		if diff.Action != "none" {
			t.Fatalf("second plan %s: action %s, want none", diff.Target, diff.Action)
		}
	}
}

func TestPlanConflictBlocksAndExecuteRefuses(t *testing.T) {
	ed := &conflictEditor{memEditor: newMemEditor(), badTarget: "wt:scalar:theme"}
	plan, err := Plan(sampleChoice(), ed)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !plan.Blocked {
		t.Fatal("conflict row must block the plan")
	}
	if got := targetOf(t, plan, "wt:scalar:theme"); got.Conflict == "" {
		t.Fatalf("conflict reason missing: %+v", got)
	}
	// unrelated rows stay actionable
	if got := targetOf(t, plan, "wt:scalar:defaultProfile"); got.Action != "add" || got.Conflict != "" {
		t.Fatalf("unrelated row damaged by conflict: %+v", got)
	}
	if _, err := Execute(plan, ed, newMemStore()); err == nil {
		t.Fatal("Execute must refuse a blocked plan")
	}
}

func TestExecuteSavesBackupAndApplies(t *testing.T) {
	ed := newMemEditor()
	ed.values["wt:scalar:theme"] = "Campbell"
	store := newMemStore()
	plan, err := Plan(sampleChoice(), ed)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	result, err := Execute(plan, ed, store)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Succeeded {
		t.Fatalf("result not succeeded: %+v", result)
	}
	if got := ed.values["wt:scalar:theme"]; got != `"Shuttle Night"` {
		t.Fatalf("theme value %q, want Shuttle Night", got)
	}
	o, a, _, err := store.Load(result.OpID, "wt:scalar:theme")
	if err != nil || o != "Campbell" || a != `"Shuttle Night"` {
		t.Fatalf("backup o=%q a=%q err=%v", o, a, err)
	}
}

func TestExecuteStaleOnDrift(t *testing.T) {
	ed := newMemEditor()
	ed.values["wt:scalar:theme"] = "Campbell"
	plan, err := Plan(sampleChoice(), ed)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// the user changes the file between Plan and Execute
	ed.values["wt:scalar:theme"] = "One Half Dark"
	result, _ := Execute(plan, ed, newMemStore())
	if result.Succeeded {
		t.Fatal("drifted target must fail")
	}
	for _, step := range result.Steps {
		if step.Target == "wt:scalar:theme" && step.Err == "" {
			t.Fatalf("theme step must carry the stale error: %+v", step)
		}
	}
	if got := ed.values["wt:scalar:theme"]; got != "One Half Dark" {
		t.Fatalf("stale target must not be written, got %q", got)
	}
}

func TestExecuteSkipsNoneRows(t *testing.T) {
	ed := newMemEditor()
	ed.values["wt:scalar:theme"] = `"Shuttle Night"` // already equal → none
	plan, err := Plan(sampleChoice(), ed)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	result, err := Execute(plan, ed, newMemStore())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, step := range result.Steps {
		if step.Target == "wt:scalar:theme" && !step.Skipped {
			t.Fatalf("none row must be skipped: %+v", step)
		}
	}
	if len(plan.Diffs) != len(result.Steps) {
		t.Fatalf("steps %d != diffs %d", len(result.Steps), len(plan.Diffs))
	}
}

func TestPlanRejectsUnsupportedChoice(t *testing.T) {
	bad := sampleChoice()
	bad.SchemaVersion = 99
	if _, err := Plan(bad, newMemEditor()); err == nil {
		t.Fatal("unknown schemaVersion must be rejected")
	}
	st := sampleChoice()
	st.Statusline = "powerline"
	if _, err := Plan(st, newMemEditor()); err == nil {
		t.Fatal("statusline must be rejected as unsupported in this batch")
	}
}

func TestDecideRestoreMatrix(t *testing.T) {
	cases := []struct {
		name    string
		o, a, c string
		oAbsent bool
		want    RestoreAction
	}{
		{"c-equals-a-restores-o", "old", "new", "new", false, RestoreToOriginal},
		{"c-equals-a-o-absent-removes", "", "new", "new", true, RestoreRemove},
		{"c-equals-o-skips", "old", "new", "old", false, RestoreSkip},
		{"user-changed-keeps-current", "old", "new", "theirs", false, RestoreConflict},
	}
	for _, tc := range cases {
		if got := DecideRestore(tc.o, tc.a, tc.c, tc.oAbsent); got != tc.want {
			t.Fatalf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestRestoreOrchestration(t *testing.T) {
	ed := newMemEditor()
	ed.values["wt:scalar:theme"] = "Campbell" // O present for this row
	store := newMemStore()
	plan, err := Plan(sampleChoice(), ed)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	result, err := Execute(plan, ed, store)
	if err != nil || !result.Succeeded {
		t.Fatalf("Execute: %+v err=%v", result, err)
	}
	// C == A everywhere → restore all
	restored, err := Restore(result.OpID, ed, store)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if !restored.Succeeded {
		t.Fatalf("restore failed: %+v", restored)
	}
	// C == A everywhere → restore all; rows whose O was absent must remove
	expected := map[string]RestoreAction{
		"wt:scalar:theme":                      RestoreToOriginal, // O was "Campbell"
		"wt:scalar:defaultProfile":             RestoreRemove,
		"wt:scalar:profiles.defaults.fontFace": RestoreRemove,
		"ps:managed-block":                     RestoreRemove,
	}
	for _, step := range restored.Steps {
		want := expected[step.Target]
		if step.Action != string(want) || step.Skipped {
			t.Fatalf("target %s: got %s, want %s", step.Target, step.Action, want)
		}
	}
	// after restore, current equals O; second restore must skip
	again, err := Restore(result.OpID, ed, store)
	if err != nil {
		t.Fatalf("second Restore: %v", err)
	}
	for _, step := range again.Steps {
		if step.Action != string(RestoreSkip) {
			t.Fatalf("expected skip, got %+v", step)
		}
	}
}

func TestRestoreRemoveWhenOriginalAbsent(t *testing.T) {
	ed := newMemEditor()
	store := newMemStore()
	plan, err := Plan(sampleChoice(), ed)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	result, err := Execute(plan, ed, store)
	if err != nil || !result.Succeeded {
		t.Fatalf("Execute: %+v err=%v", result, err)
	}
	// every target was absent before (O absent) → restore must remove
	restored, err := Restore(result.OpID, ed, store)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if !restored.Succeeded {
		t.Fatalf("restore failed: %+v", restored)
	}
	for _, step := range restored.Steps {
		if step.Action != string(RestoreRemove) {
			t.Fatalf("expected remove, got %+v", step)
		}
	}
	if got := ed.values["wt:scalar:theme"]; got != "" {
		t.Fatalf("managed value must be removed, got %q", got)
	}
}

func TestRestoreWithoutBackupsFails(t *testing.T) {
	_, err := Restore("op-none", newMemEditor(), newMemStore())
	var cfgErr *Error
	if !errors.As(err, &cfgErr) || cfgErr.Kind != "not_found" {
		t.Fatalf("wrong error: %v", err)
	}
}

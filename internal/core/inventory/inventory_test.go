package inventory

import (
	"errors"
	"testing"
)

// memLedger is the in-memory ledger of contract §4.
type memLedger struct {
	records map[string]Record
	err     error
}

func newLedger() *memLedger { return &memLedger{records: map[string]Record{}} }

func (l *memLedger) Lookup(name string) (Record, bool, error) {
	if l.err != nil {
		return Record{}, false, l.err
	}
	record, ok := l.records[name]
	return record, ok, nil
}

func (l *memLedger) Put(record Record) error {
	if l.err != nil {
		return l.err
	}
	if l.records == nil {
		l.records = map[string]Record{}
	}
	l.records[record.Name] = record
	return nil
}

func (l *memLedger) List() ([]Record, error) {
	if l.err != nil {
		return nil, l.err
	}
	out := make([]Record, 0, len(l.records))
	for _, record := range l.records {
		out = append(out, record)
	}
	return out, nil
}

func (l *memLedger) Delete(name string) error {
	if l.err != nil {
		return l.err
	}
	delete(l.records, name)
	return nil
}

func find(t *testing.T, items []Item, name string) Item {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("%q not in items", name)
	return Item{}
}

// Contract §7 test 3: the three ownership verdicts.
func TestClassifyThreeVerdicts(t *testing.T) {
	ledger := newLedger()
	ledger.records["@anthropic-ai/claude-code"] = Record{
		Name: "@anthropic-ai/claude-code", Source: SourceNPM, Root: `C:\Users\admin\AppData\Roaming\npm\node_modules`}
	ledger.records["Anthropic.Claude"] = Record{Name: "Anthropic.Claude", Source: SourceWinget}

	items := Classify([]Package{
		{Name: "@anthropic-ai/claude-code", Version: "2.1.310", Source: SourceNPM,
			Root: `C:\Users\admin\AppData\Roaming\npm\node_modules`},
		{Name: "@openai/codex", Version: "0.164.0", Source: SourceNPM,
			Root: `C:\Users\admin\AppData\Roaming\npm\node_modules`},
		{Name: "Anthropic.Claude", Source: SourceWinget},
		{Name: "codex", Source: SourcePath},
	}, ledger)

	if got := find(t, items, "@anthropic-ai/claude-code"); got.Ownership != OwnedByShuttle {
		t.Fatalf("managed package: %+v", got)
	}
	if got := find(t, items, "@openai/codex"); got.Ownership != OwnedExternally {
		t.Fatalf("unrecorded package must be external: %+v", got)
	}
	if got := find(t, items, "codex"); got.Ownership != OwnedExternally {
		t.Fatalf("path source must always be external: %+v", got)
	}
	for _, item := range items {
		if item.Reason == "" {
			t.Fatalf("every verdict needs a reason: %+v", item)
		}
	}
}

// Contract §7 test 4: a similar name must never imply ownership.
func TestSimilarNamesDoNotInheritOwnership(t *testing.T) {
	ledger := newLedger()
	ledger.records["@anthropic-ai/claude-code"] = Record{Name: "@anthropic-ai/claude-code", Source: SourceNPM}
	items := Classify([]Package{
		{Name: "@anthropic-ai/claude-code-helper", Source: SourceNPM},
		{Name: "claude-code", Source: SourceNPM},
		{Name: "@anthropic-ai/CLAUDE-CODE", Source: SourceNPM},
	}, ledger)
	for _, item := range items {
		if item.Ownership != OwnedExternally {
			t.Fatalf("similar name must stay external: %+v", item)
		}
	}
}

// Contract §7 test 5: a broken ledger makes everything unknown, never external.
func TestBrokenLedgerIsUnknownNotExternal(t *testing.T) {
	ledger := newLedger()
	ledger.err = errors.New("record.json 损坏")
	items := Classify([]Package{{Name: "@openai/codex", Source: SourceNPM}}, ledger)
	if items[0].Ownership != OwnershipUnknown {
		t.Fatalf("broken ledger must be unknown: %+v", items[0])
	}
	if items[0].Reason == "" {
		t.Fatal("broken ledger reason must be shown")
	}
	// an absent ledger is different: nothing was ever installed by Shuttle
	absent := Classify([]Package{{Name: "@openai/codex", Source: SourceNPM}}, nil)
	if absent[0].Ownership != OwnedExternally {
		t.Fatalf("nil ledger must be external: %+v", absent[0])
	}
}

func TestSourceMismatchAndRootMismatchAreUnknown(t *testing.T) {
	ledger := newLedger()
	ledger.records["Anthropic.Claude"] = Record{Name: "Anthropic.Claude", Source: SourceNPM}
	ledger.records["OpenAI.Codex"] = Record{Name: "OpenAI.Codex", Source: SourceWinget, Root: `C:\A`}
	items := Classify([]Package{
		{Name: "Anthropic.Claude", Source: SourceWinget},
		{Name: "OpenAI.Codex", Source: SourceWinget, Root: `D:\B`},
	}, ledger)
	if items[0].Ownership != OwnershipUnknown {
		t.Fatalf("source mismatch must be unknown: %+v", items[0])
	}
	if items[1].Ownership != OwnershipUnknown {
		t.Fatalf("root mismatch must be unknown: %+v", items[1])
	}
}

func TestUnknownSourceIsNotManaged(t *testing.T) {
	ledger := newLedger()
	ledger.records["weird"] = Record{Name: "weird", Source: Source("scoop")}
	items := Classify([]Package{{Name: "weird", Source: Source("scoop")}}, ledger)
	if items[0].Ownership != OwnershipUnknown {
		t.Fatalf("unsupported source must be unknown: %+v", items[0])
	}
}

// Contract §7 test 7: conflicts block, unrelated selections survive.
func TestPlanUninstallBlocksConflictsAndKeepsOthers(t *testing.T) {
	items := []Item{
		{Package: Package{Name: "managed-pkg", Source: SourceNPM}, Ownership: OwnedByShuttle, Reason: "ok"},
		{Package: Package{Name: "user-pkg", Source: SourceNPM}, Ownership: OwnedExternally, Reason: "用户自行安装"},
		{Package: Package{Name: "odd-pkg", Source: SourceWinget}, Ownership: OwnershipUnknown, Reason: "来源不一致"},
	}
	plan, err := PlanUninstall(items, Choice{Names: []string{"managed-pkg", "user-pkg", "odd-pkg", "ghost-pkg"}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.Blocked || len(plan.Conflicts) != 2 {
		t.Fatalf("conflicts: %+v", plan)
	}
	actions := map[string]string{}
	for _, step := range plan.Steps {
		actions[step.Name] = step.Action
	}
	if actions["managed-pkg"] != "uninstall" {
		t.Fatalf("managed must be uninstallable: %+v", actions)
	}
	if actions["user-pkg"] != "skip" || actions["odd-pkg"] != "skip" {
		t.Fatalf("external and unknown must be skipped: %+v", actions)
	}
	// the skipped rows keep their reason so the UI can explain
	for _, step := range plan.Steps {
		if step.Action == "skip" && step.Reason == "" {
			t.Fatalf("skip reason missing: %+v", step)
		}
	}
}

func TestPlanUninstallRequiresSelection(t *testing.T) {
	if _, err := PlanUninstall(nil, Choice{}); err == nil {
		t.Fatal("empty selection must be rejected")
	}
}

// Contract §10.8 test 10: only installed becomes managed; every other state
// either blocks (unknown) or is explicitly the user's own (external).
func TestRecordStateMapsToOwnership(t *testing.T) {
	cases := []struct {
		state RecordState
		want  Ownership
	}{
		{StateInstalled, OwnedByShuttle},
		{"", OwnedByShuttle}, // v1 record: a state-less row only existed after a successful write
		{StateUnverified, OwnershipUnknown},
		{StateIntent, OwnershipUnknown},
		{StateFailed, OwnedExternally},
		{RecordState("bogus"), OwnershipUnknown},
	}
	for _, c := range cases {
		ledger := newLedger()
		ledger.records["pkg"] = Record{Name: "pkg", Source: SourceNPM, Root: `C:\npm`, State: c.state}
		items := Classify([]Package{{Name: "pkg", Source: SourceNPM, Root: `C:\npm`}}, ledger)
		if items[0].Ownership != c.want {
			t.Fatalf("state %q: got %s want %s (%s)", c.state, items[0].Ownership, c.want, items[0].Reason)
		}
		if items[0].Reason == "" {
			t.Fatalf("state %q: reason missing", c.state)
		}
	}
}

// A non-installed record must never be uninstallable, even when the name,
// source and root all match.
func TestNonInstalledRecordsAreNotUninstallable(t *testing.T) {
	items := []Item{
		{Package: Package{Name: "intent", Source: SourceNPM, Root: `C:\npm`}, Ownership: OwnershipUnknown, Reason: "只有意图"},
		{Package: Package{Name: "failed", Source: SourceNPM, Root: `C:\npm`}, Ownership: OwnedExternally, Reason: "安装失败"},
	}
	plan, err := PlanUninstall(items, Choice{Names: []string{"intent", "failed"}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.Blocked {
		t.Fatal("unknown must block the plan")
	}
	for _, step := range plan.Steps {
		if step.Action != "skip" {
			t.Fatalf("non-installed record planned %q: %+v", step.Action, step)
		}
	}
}

func TestPlanUninstallPopulatesCommandAndArgs(t *testing.T) {
	items := []Item{
		{Package: Package{Name: "@anthropic-ai/claude-code", Source: SourceNPM}, Ownership: OwnedByShuttle},
		{Package: Package{Name: "Git.Git", Source: SourceWinget}, Ownership: OwnedByShuttle},
	}
	plan, err := PlanUninstall(items, Choice{Names: []string{"@anthropic-ai/claude-code", "Git.Git"}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.Blocked || len(plan.Steps) != 2 {
		t.Fatalf("plan: %+v", plan)
	}
	npmStep := plan.Steps[0]
	if npmStep.Command != "npm uninstall -g @anthropic-ai/claude-code" || len(npmStep.Args) != 4 {
		t.Fatalf("npm step command/args: %+v", npmStep)
	}
	wingetStep := plan.Steps[1]
	if wingetStep.Command != "winget uninstall --id Git.Git --exact" || len(wingetStep.Args) != 5 {
		t.Fatalf("winget step command/args: %+v", wingetStep)
	}
}

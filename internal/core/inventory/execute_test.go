package inventory

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeInstaller struct {
	calls [][]string
	code  int
	msg   string
	err   error
	// after is the detection the snapshot reports once the installer ran
	after []Package
}

func (f *fakeInstaller) Run(argv []string) (int, string, error) {
	f.calls = append(f.calls, argv)
	return f.code, f.msg, f.err
}

func snapshotOf(pkgs ...Package) Snapshot {
	return func() ([]Package, error) { return pkgs, nil }
}

func installPlan(t *testing.T, pkgs ...Package) Plan {
	t.Helper()
	plan, err := PlanInstall(pkgs, nil)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return plan
}

// Contract §10.8 test 12 (success half): a successful install writes intent
// before running, then installed with evidence from the recheck.
func TestExecuteInstallWritesIntentThenInstalled(t *testing.T) {
	ledger := newLedger()
	installer := &fakeInstaller{code: 0}
	detected := Package{Name: "@openai/codex", Version: "0.165.0", Source: SourceNPM, Root: `C:\npm`}
	plan := installPlan(t, Package{Name: "@openai/codex", Source: SourceNPM})

	execution, err := ExecuteInstall(plan, "op-1", ledger, installer, snapshotOf(detected), nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if execution.State != "succeeded" {
		t.Fatalf("state: %s", execution.State)
	}
	if len(installer.calls) != 1 {
		t.Fatalf("installer calls: %v", installer.calls)
	}
	// argv, not a shell string: the scoped name must arrive as one argument
	want := []string{"npm", "install", "-g", "@openai/codex"}
	if strings.Join(installer.calls[0], "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("argv: %v want %v", installer.calls[0], want)
	}
	record := ledger.records["@openai/codex"]
	if record.State != StateInstalled {
		t.Fatalf("record: %+v", record)
	}
	if record.Version != "0.165.0" || record.Root != `C:\npm` || record.OperationID != "op-1" {
		t.Fatalf("record must carry the recheck evidence: %+v", record)
	}
	if _, err := time.Parse(time.RFC3339, record.InstalledAt); err != nil {
		t.Fatalf("successful install timestamp: %q: %v", record.InstalledAt, err)
	}
	if record.EvidenceBefore != "absent" || record.EvidenceAfter == "" {
		t.Fatalf("evidence: %+v", record)
	}
}

// Contract §10.8 test 12 (failure half): installer failure is failed, but a
// package that does exist afterwards is unverified — never "installed".
func TestExecuteInstallFailureStates(t *testing.T) {
	cases := []struct {
		name     string
		detected []Package
		want     RecordState
	}{
		{"failed and absent", nil, StateFailed},
		{"failed but present", []Package{{Name: "tool", Version: "1.0", Source: SourceNPM, Root: `C:\npm`}}, StateUnverified},
	}
	for _, c := range cases {
		ledger := newLedger()
		installer := &fakeInstaller{code: 1, msg: "npm ERR! 404 Not Found", err: errors.New("exit 1")}
		plan := installPlan(t, Package{Name: "tool", Source: SourceNPM})
		execution, err := ExecuteInstall(plan, "op-2", ledger, installer, snapshotOf(c.detected...), nil)
		if err == nil {
			t.Fatalf("%s: installer failure must surface", c.name)
		}
		record := ledger.records["tool"]
		if record.State != c.want {
			t.Fatalf("%s: state %s want %s", c.name, record.State, c.want)
		}
		if record.InstallerExit != 1 || !strings.Contains(record.InstallerError, "404") {
			t.Fatalf("%s: failure reason must be kept: %+v", c.name, record)
		}
		if execution.State != "failed" && execution.State != "partially_failed" {
			t.Fatalf("%s: execution state: %s", c.name, execution.State)
		}
	}
}

// A successful installer that leaves nothing detectable is unverified.
func TestExecuteInstallUnverifiedWhenNotDetected(t *testing.T) {
	ledger := newLedger()
	plan := installPlan(t, Package{Name: "tool", Source: SourceWinget})
	execution, err := ExecuteInstall(plan, "op-3", ledger, &fakeInstaller{code: 0}, snapshotOf(), nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if execution.State != "partially_failed" {
		t.Fatalf("unverified install must not report success: %s", execution.State)
	}
	if ledger.records["tool"].State != StateUnverified {
		t.Fatalf("record: %+v", ledger.records["tool"])
	}
}

// A recheck that cannot run proves nothing: the record stays unverified rather
// than claiming either outcome.
func TestExecuteInstallSnapshotFailureKeepsUnverified(t *testing.T) {
	ledger := newLedger()
	plan := installPlan(t, Package{Name: "tool", Source: SourceNPM})
	failing := func() ([]Package, error) { return nil, errors.New("npm 不可用") }
	if _, err := ExecuteInstall(plan, "op-4", ledger, &fakeInstaller{code: 0}, failing, nil); err != nil {
		t.Fatalf("execute: %v", err)
	}
	record := ledger.records["tool"]
	if record.State != StateUnverified || !strings.Contains(record.EvidenceAfter, "npm 不可用") {
		t.Fatalf("record: %+v", record)
	}
}

// Contract §10.5 item 3: the ledger write is part of the operation. If it fails
// after the installer ran, the operation fails and the package is not claimed.
func TestExecuteInstallLedgerFailureFailsOperation(t *testing.T) {
	ledger := newLedger()
	// first Put (intent) succeeds, the terminal Put fails
	failing := &failingLedger{memLedger: ledger, failOn: 2}
	plan := installPlan(t, Package{Name: "tool", Source: SourceNPM})
	execution, err := ExecuteInstall(plan, "op-5", failing, &fakeInstaller{code: 0},
		snapshotOf(Package{Name: "tool", Source: SourceNPM, Root: `C:\npm`}), nil)
	if err == nil || !strings.Contains(err.Error(), "台账") {
		t.Fatalf("ledger failure must surface: %v", err)
	}
	if execution.State != "failed" {
		t.Fatalf("state: %s", execution.State)
	}
	if ledger.records["tool"].State != StateIntent {
		t.Fatalf("the durable ledger must still show intent, not a claim: %+v", ledger.records["tool"])
	}
}

// Contract §10.8 test 16: a plan generated against old state must be refused,
// not executed.
func TestExecuteInstallRefusesStalePlan(t *testing.T) {
	ledger := newLedger()
	installer := &fakeInstaller{code: 0}
	plan := installPlan(t, Package{Name: "tool", Source: SourceNPM})
	// by execution time the tool is already installed by the user
	current := func() (Plan, error) {
		return PlanInstall([]Package{{Name: "tool", Source: SourceNPM}},
			[]Item{{Package: Package{Name: "tool", Source: SourceNPM}, Ownership: OwnedExternally, Reason: "台账无记录"}})
	}
	_, err := ExecuteInstall(plan, "op-6", ledger, installer, snapshotOf(), current)
	if !IsStale(err) {
		t.Fatalf("stale plan must be refused: %v", err)
	}
	if len(installer.calls) != 0 {
		t.Fatalf("a stale plan must run nothing: %v", installer.calls)
	}
	if len(ledger.records) != 0 {
		t.Fatalf("a stale plan must write nothing: %v", ledger.records)
	}
}

func TestExecuteInstallRefusesBlockedPlanAndMissingDependencies(t *testing.T) {
	ledger := newLedger()
	installer := &fakeInstaller{}
	blocked := Plan{Blocked: true, Conflicts: []string{"x：冲突"}}
	if _, err := ExecuteInstall(blocked, "op", ledger, installer, snapshotOf(), nil); err == nil {
		t.Fatal("blocked plan must be refused")
	}
	plan := installPlan(t, Package{Name: "tool", Source: SourceNPM})
	if _, err := ExecuteInstall(plan, "", ledger, installer, snapshotOf(), nil); err == nil {
		t.Fatal("missing operation id must be refused")
	}
	if _, err := ExecuteInstall(plan, "op", nil, installer, snapshotOf(), nil); err == nil {
		t.Fatal("missing ledger must be refused")
	}
	if len(installer.calls) != 0 || len(ledger.records) != 0 {
		t.Fatal("refused plans must have no side effects")
	}
}

// Contract §10.7: startup reconciliation closes intent records by re-detecting,
// never by retrying the install.
func TestReconcileInterruptedClosesIntentRecords(t *testing.T) {
	ledger := newLedger()
	ledger.records["present"] = Record{Name: "present", Source: SourceNPM, Root: `C:\npm`, State: StateIntent, OperationID: "op-a"}
	ledger.records["gone"] = Record{Name: "gone", Source: SourceNPM, Root: `C:\npm`, State: StateIntent, OperationID: "op-b"}
	ledger.records["done"] = Record{Name: "done", Source: SourceNPM, State: StateInstalled, OperationID: "op-c"}

	closed, err := ReconcileInterrupted(ledger, snapshotOf(
		Package{Name: "present", Version: "1.2.3", Source: SourceNPM, Root: `C:\npm`},
		Package{Name: "done", Source: SourceNPM},
	))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(closed) != 2 {
		t.Fatalf("closed %d records, want 2", len(closed))
	}
	if ledger.records["present"].State != StateUnverified || ledger.records["present"].Version != "1.2.3" {
		t.Fatalf("present: %+v", ledger.records["present"])
	}
	if ledger.records["gone"].State != StateUnverified {
		t.Fatalf("gone: %+v", ledger.records["gone"])
	}
	for _, record := range closed {
		if record.InstalledAt != "" || !strings.Contains(record.InstallerError, "缺少安装器成功证据") {
			t.Fatalf("reconciliation must not invent success evidence: %+v", record)
		}
	}
	// already-settled records are left untouched
	if ledger.records["done"].OperationID != "op-c" || ledger.records["done"].InstallerError != "" {
		t.Fatalf("settled record was rewritten: %+v", ledger.records["done"])
	}
	// reconciliation must not invent installs
	items := Classify([]Package{
		{Name: "gone", Source: SourceNPM, Root: `C:\npm`},
		{Name: "present", Source: SourceNPM, Root: `C:\npm`},
	}, ledger)
	for _, item := range items {
		if item.Ownership == OwnedByShuttle {
			t.Fatalf("reconciled package must not become managed: %+v", item)
		}
	}
}

func TestStepJSONKeepsNestedItemAndOmitsArgs(t *testing.T) {
	step := Step{
		Item:    Item{Package: Package{Name: "detected", Source: SourceNPM}, Ownership: OwnedExternally, Reason: "existing"},
		Package: Package{Name: "wanted", Source: SourceWinget},
		Action:  ActionInstall, Command: "display command", Args: []string{"private-argument"},
	}
	data, err := json.Marshal(step)
	if err != nil {
		t.Fatal(err)
	}
	var dto map[string]json.RawMessage
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatal(err)
	}
	var item Item
	if err := json.Unmarshal(dto["Item"], &item); err != nil || item.Name != "detected" || item.Reason != "existing" {
		t.Fatalf("nested Item lost: %s, %v", data, err)
	}
	var pkg Package
	if err := json.Unmarshal(dto["Package"], &pkg); err != nil || pkg.Name != "wanted" {
		t.Fatalf("install Package lost: %s, %v", data, err)
	}
	if _, ok := dto["Ownership"]; ok || strings.Contains(string(data), "Args") || strings.Contains(string(data), "private-argument") {
		t.Fatalf("flattened Item or leaked args: %s", data)
	}
}

func TestReconcileInterruptedWithoutIntentSkipsSnapshot(t *testing.T) {
	for _, settled := range []bool{false, true} {
		ledger := newLedger()
		if settled {
			ledger.records["done"] = Record{Name: "done", State: StateInstalled}
		}
		closed, err := ReconcileInterrupted(ledger, func() ([]Package, error) {
			t.Fatal("snapshot must not run without intent records")
			return nil, errors.New("unavailable")
		})
		if err != nil || len(closed) != 0 {
			t.Fatalf("empty reconciliation: %v, %v", closed, err)
		}
	}
}

func TestExecuteInstallNonzeroExitWithoutError(t *testing.T) {
	for _, present := range []bool{false, true} {
		ledger := newLedger()
		plan := installPlan(t, Package{Name: "tool", Source: SourceNPM})
		var detected []Package
		want := StateFailed
		if present {
			detected = []Package{{Name: "tool", Source: SourceNPM, Root: `C:\npm`}}
			want = StateUnverified
		}
		execution, err := ExecuteInstall(plan, "nonzero", ledger, &fakeInstaller{code: 7}, func() ([]Package, error) {
			if intent := ledger.records["tool"]; intent.State != StateIntent || intent.InstalledAt != "" {
				t.Fatalf("intent must have no success timestamp: %+v", intent)
			}
			return detected, nil
		}, nil)
		record := ledger.records["tool"]
		if err == nil || execution.State != "failed" || record.State != want || record.InstallerExit != 7 || record.InstallerError == "" || record.InstalledAt != "" {
			t.Fatalf("nonzero exit: execution=%+v record=%+v err=%v", execution, record, err)
		}
	}
}

func TestExecuteInstallRejectsChangedInstallDetails(t *testing.T) {
	changes := map[string]func(*Plan){
		"blocked":             func(p *Plan) { p.Blocked = true },
		"package name":        func(p *Plan) { p.Steps[0].Package.Name = "other" },
		"source":              func(p *Plan) { p.Steps[0].Package.Source = SourceWinget },
		"root":                func(p *Plan) { p.Steps[0].Package.Root = `C:\other` },
		"command":             func(p *Plan) { p.Steps[0].Command += " --changed" },
		"args":                func(p *Plan) { p.Steps[0].Args = []string{"npm", "install", "other"} },
		"argument boundaries": func(p *Plan) { p.Steps[0].Args = []string{"npm|install", "-g", "tool"} },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			plan := installPlan(t, Package{Name: "tool", Source: SourceNPM})
			fresh := installPlan(t, Package{Name: "tool", Source: SourceNPM})
			change(&fresh)
			ledger := newLedger()
			installer := &fakeInstaller{}
			_, err := ExecuteInstall(plan, "stale", ledger, installer, snapshotOf(), func() (Plan, error) { return fresh, nil })
			if !IsStale(err) || len(installer.calls) != 0 || len(ledger.records) != 0 {
				t.Fatalf("changed plan must fail without effects: %v, %+v, %+v", err, installer.calls, ledger.records)
			}
		})
	}
}

type failingLedger struct {
	*memLedger
	calls  int
	failOn int
}

func (l *failingLedger) Put(record Record) error {
	l.calls++
	if l.calls == l.failOn {
		return errors.New("磁盘已满")
	}
	return l.memLedger.Put(record)
}

func TestExecuteUninstallSuccessDeletesFromLedger(t *testing.T) {
	pkg := Package{Name: "tool", Source: SourceNPM, Root: `C:\npm`}
	item := Item{Package: pkg, Ownership: OwnedByShuttle}
	plan, err := PlanUninstall([]Item{item}, Choice{Names: []string{"tool"}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	ledger := newLedger()
	_ = ledger.Put(Record{Name: "tool", Source: SourceNPM, Root: `C:\npm`, State: StateInstalled})

	installer := &fakeInstaller{}
	snapshot := func() ([]Package, error) {
		if len(installer.calls) > 0 {
			return []Package{}, nil
		}
		return []Package{pkg}, nil
	}

	exec, err := ExecuteUninstall(plan, "op-rm-1", ledger, installer, snapshot, func() (Plan, error) { return plan, nil })
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if exec.State != "succeeded" || len(installer.calls) != 1 {
		t.Fatalf("unexpected execution: %+v calls=%v", exec, installer.calls)
	}
	if _, ok, _ := ledger.Lookup("tool"); ok {
		t.Fatal("tool should have been deleted from ledger upon successful uninstall")
	}
}

func TestExecuteUninstallFailsWhenStillDetected(t *testing.T) {
	pkg := Package{Name: "tool", Source: SourceNPM, Root: `C:\npm`}
	item := Item{Package: pkg, Ownership: OwnedByShuttle}
	plan, err := PlanUninstall([]Item{item}, Choice{Names: []string{"tool"}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	ledger := newLedger()
	_ = ledger.Put(Record{Name: "tool", Source: SourceNPM, Root: `C:\npm`, State: StateInstalled})

	currentPkgs := []Package{pkg}
	snapshot := func() ([]Package, error) { return currentPkgs, nil }
	installer := &fakeInstaller{}

	exec, err := ExecuteUninstall(plan, "op-rm-2", ledger, installer, snapshot, func() (Plan, error) { return plan, nil })
	if err != nil {
		t.Fatalf("uninstall should return partial without fatal err: %v", err)
	}
	if exec.State != "partially_failed" {
		t.Fatalf("unexpected execution state: %+v", exec)
	}
	rec, ok, _ := ledger.Lookup("tool")
	if !ok || rec.State != StateUnverified {
		t.Fatalf("tool should be unverified in ledger: %+v", rec)
	}
}

func TestExecuteUninstallRefusesStalePlan(t *testing.T) {
	pkg := Package{Name: "tool", Source: SourceNPM, Root: `C:\npm`}
	item := Item{Package: pkg, Ownership: OwnedByShuttle}
	plan, _ := PlanUninstall([]Item{item}, Choice{Names: []string{"tool"}})

	ledger := newLedger()
	installer := &fakeInstaller{}
	otherPlan, _ := PlanUninstall([]Item{item, {Package: Package{Name: "other", Source: SourceNPM}, Ownership: OwnedByShuttle}}, Choice{Names: []string{"tool", "other"}})

	_, err := ExecuteUninstall(plan, "op-stale", ledger, installer, snapshotOf(pkg), func() (Plan, error) { return otherPlan, nil })
	if !IsStale(err) || len(installer.calls) != 0 {
		t.Fatalf("stale plan must be rejected: %v, calls=%v", err, installer.calls)
	}
}

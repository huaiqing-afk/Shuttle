package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configadapter "shuttle/internal/adapters/configuration"
	adapter "shuttle/internal/adapters/inventory"
	core "shuttle/internal/core/inventory"
)

// fakeInstaller records argv and never touches the machine. onRun lets a test
// change what detection will report, exactly as a real install would.
type fakeInstaller struct {
	calls [][]string
	code  int
	err   error
	onRun func()
}

func (f *fakeInstaller) Run(argv []string) (int, string, error) {
	f.calls = append(f.calls, argv)
	if f.onRun != nil {
		f.onRun()
	}
	return f.code, "fake output", f.err
}

// installFixture builds a service whose detection is injected, so the bridge
// tests exercise the real plan/execute path without running npm or winget.
func installFixture(t *testing.T, detected []core.Package) (*App, *fakeInstaller, string) {
	t.Helper()
	dir := t.TempDir()
	service := &inventoryService{root: dir, note: "note"}
	ledgerPath := filepath.Join(dir, "install-ledger.json")
	service.ledger = newTestLedger(ledgerPath)
	installer := &fakeInstaller{code: 0}
	service.installer = installer
	service.detectPackages = func() ([]core.Package, error) { return detected, nil }
	app := &App{}
	app.inventory = service
	return app, installer, dir
}

// A preview must never install: nothing runs until the token is executed.
func TestPlanInstallPreviewIsReadOnly(t *testing.T) {
	app, installer, dir := installFixture(t, nil)
	preview, err := app.PlanInstall([]string{"claude", "git"})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Token == "" || len(preview.Plan.Steps) != 2 || preview.Plan.Blocked {
		t.Fatalf("preview: %+v", preview)
	}
	if len(installer.calls) != 0 {
		t.Fatalf("preview ran an installer: %v", installer.calls)
	}
	if _, err := os.Stat(filepath.Join(dir, "install-ledger.json")); !os.IsNotExist(err) {
		t.Fatalf("preview wrote the ledger: %v", err)
	}
	// the command the user must see, and the argv that will run
	claude := preview.Plan.Steps[0]
	if claude.Item.Source != "npm" || claude.Command != "npm install -g @anthropic-ai/claude-code" {
		t.Fatalf("claude step: %+v", claude)
	}
	if strings.Join(app.inventory.preview.plan.Steps[0].Args, " ") != "npm install -g @anthropic-ai/claude-code" {
		t.Fatalf("claude argv: %v", app.inventory.preview.plan.Steps[0].Args)
	}
	if preview.Plan.Steps[1].Item.Source != "winget" || !strings.Contains(preview.Plan.Steps[1].Command, "--id Git.Git --exact") {
		t.Fatalf("git step: %+v", preview.Plan.Steps[1])
	}
}

func TestExecuteInstallRequiresMatchingPreview(t *testing.T) {
	app, installer, _ := installFixture(t, nil)
	if _, err := app.ExecuteInstall("preview-guess"); err == nil {
		t.Fatal("execution without a preview must be refused")
	}
	if len(installer.calls) != 0 {
		t.Fatalf("refused execution ran an installer: %v", installer.calls)
	}

	preview, err := app.PlanInstall([]string{"claude"})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if _, err := app.ExecuteInstall("preview-wrong"); err == nil {
		t.Fatal("a mismatched token must be refused")
	}
	if len(installer.calls) != 0 {
		t.Fatal("mismatched token ran an installer")
	}

	// the injected detection never reports the package, so the run is reported
	// as not fully successful rather than silently passing
	result, err := app.ExecuteInstall(preview.Token)
	if err != nil || result.Error == "" {
		t.Fatalf("an unverified install must preserve its failure result: %+v, %v", result, err)
	}
	if result.State != "partially_failed" {
		t.Fatalf("state: %s", result.State)
	}
	if len(installer.calls) != 1 || strings.Join(installer.calls[0], " ") != "npm install -g @anthropic-ai/claude-code" {
		t.Fatalf("installer calls: %v", installer.calls)
	}
	// one preview authorizes one execution
	if _, err := app.ExecuteInstall(preview.Token); err == nil {
		t.Fatal("a reused preview must be refused")
	}
}

// A confirmed preview followed by a successful recheck records the install as
// managed, which is what makes the item removable later.
func TestExecuteInstallEndToEndRecordsManaged(t *testing.T) {
	var detected []core.Package
	dir := t.TempDir()
	service := &inventoryService{root: dir, note: "note"}
	service.ledger = newTestLedger(filepath.Join(dir, "install-ledger.json"))
	// the machine changes only when the installer actually runs
	installer := &fakeInstaller{code: 0, onRun: func() {
		detected = []core.Package{{Name: "@openai/codex", Version: "0.165.0", Source: core.SourceNPM, Root: `C:\npm`}}
	}}
	service.installer = installer
	service.detectPackages = func() ([]core.Package, error) { return detected, nil }
	app := &App{}
	app.inventory = service

	preview, err := app.PlanInstall([]string{"codex"})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	result, err := app.ExecuteInstall(preview.Token)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.State != "succeeded" || len(result.Records) != 1 || result.Records[0].State != "installed" {
		t.Fatalf("result: %+v", result)
	}
	items := core.Classify(detected, service.ledger)
	op, err := configadapter.NewFileOperationStore(filepath.Join(dir, "operations")).Get(result.OperationID)
	if err != nil || op.State != "succeeded" || len(op.Steps) != 1 || op.Steps[0].Action != "installed" {
		t.Fatalf("durable operation: %+v, %v", op, err)
	}
	if items[0].Ownership != core.OwnedByShuttle {
		t.Fatalf("installed package must be managed: %+v", items[0])
	}
}

// The preview is bound to the plan the user saw: if the target got installed
// by someone else in the meantime, execution is refused rather than repeated.
func TestExecuteInstallRefusesWhenStateChanged(t *testing.T) {
	var detected []core.Package
	dir := t.TempDir()
	service := &inventoryService{root: dir, note: "note"}
	service.ledger = newTestLedger(filepath.Join(dir, "install-ledger.json"))
	installer := &fakeInstaller{code: 0}
	service.installer = installer
	service.detectPackages = func() ([]core.Package, error) { return detected, nil }
	app := &App{}
	app.inventory = service

	preview, err := app.PlanInstall([]string{"git"})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	// by execution time Git is already present
	detected = []core.Package{{Name: "Git.Git", Source: core.SourceWinget, Root: "winget"}}
	if _, err := app.ExecuteInstall(preview.Token); !core.IsStale(err) {
		t.Fatalf("stale preview must be refused: %v", err)
	}
	if len(installer.calls) != 0 {
		t.Fatalf("stale preview ran an installer: %v", installer.calls)
	}
}

func TestReconcileInventoryClosesInterruptedInstall(t *testing.T) {
	app, _, dir := installFixture(t, []core.Package{
		{Name: "@anthropic-ai/claude-code", Version: "2.1.310", Source: core.SourceNPM, Root: `C:\npm`}})
	ledger, ok := app.inventory.ledger.(interface {
		Put(core.Record) error
	})
	if !ok {
		t.Fatal("ledger must be writable")
	}
	if err := ledger.Put(core.Record{Name: "@anthropic-ai/claude-code", Source: core.SourceNPM,
		Root: `C:\npm`, State: core.StateIntent, OperationID: "op-crash"}); err != nil {
		t.Fatalf("seed intent: %v", err)
	}
	closed, err := app.ReconcileInventory()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(closed) != 1 || closed[0].State != "unverified" {
		t.Fatalf("closed: %+v", closed)
	}
	if !strings.Contains(dir, os.TempDir()) {
		t.Fatalf("test root escaped temp: %s", dir)
	}
}

func TestPlanInstallRejectsUnknownComponent(t *testing.T) {
	app, _, _ := installFixture(t, nil)
	if _, err := app.PlanInstall([]string{"scoop"}); err == nil {
		t.Fatal("unknown component must be rejected")
	}
	if _, err := app.PlanInstall(nil); err == nil {
		t.Fatal("empty selection must be rejected")
	}
}

func TestInstallLockAndExitCheckpoint(t *testing.T) {
	app, installer, _ := installFixture(t, nil)
	preview, err := app.PlanInstall([]string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	release, err := app.lockInstall()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := app.ExecuteInstall(preview.Token); err == nil {
		t.Fatal("held operation lock must refuse execution")
	}
	if len(installer.calls) != 0 {
		t.Fatal("locked execution ran installer")
	}
	app.installBusy.Store(true)
	if !app.beforeClose(nil) {
		t.Fatal("window may not close during installation")
	}
	app.ExitApp()
	if app.exiting.Load() {
		t.Fatal("explicit exit bypassed installation checkpoint")
	}
}

func TestInstallOperationFailurePreventsInstaller(t *testing.T) {
	app, installer, dir := installFixture(t, nil)
	preview, err := app.PlanInstall([]string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "operations"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ExecuteInstall(preview.Token); err == nil {
		t.Fatal("operation storage failure must surface")
	}
	if len(installer.calls) != 0 {
		t.Fatal("installer ran without durable history")
	}
}

// testLedger is the real file adapter, so the bridge tests exercise real
// persistence in a temp directory rather than a stub.
type testLedger struct {
	*adapter.FileLedger
}

func newTestLedger(path string) *testLedger {
	return &testLedger{FileLedger: adapter.NewFileLedger(path)}
}

func TestExecuteInventoryRemovalRequiresMatchingPreview(t *testing.T) {
	pkg := core.Package{Name: "claude", Source: core.SourceNPM, Root: `C:\npm`}
	app, installer, _ := installFixture(t, []core.Package{pkg})
	_ = app.inventory.ledger.Put(core.Record{Name: "claude", Source: core.SourceNPM, Root: `C:\npm`, State: core.StateInstalled})

	if _, err := app.ExecuteInventoryRemoval("token-invalid"); err == nil {
		t.Fatal("expected error without preview")
	}

	preview, err := app.PlanInventoryRemoval([]string{"claude"})
	if err != nil {
		t.Fatalf("plan removal: %v", err)
	}
	if preview.Token == "" || preview.Plan.Blocked || len(preview.Plan.Steps) != 1 {
		t.Fatalf("unexpected removal preview: %+v", preview)
	}
	if preview.Plan.Steps[0].Command != "npm uninstall -g claude" {
		t.Fatalf("unexpected uninstall command: %s", preview.Plan.Steps[0].Command)
	}

	if _, err := app.ExecuteInventoryRemoval("wrong-token"); err == nil {
		t.Fatal("expected token mismatch error")
	}

	installer.onRun = func() {
		app.inventory.detectPackages = func() ([]core.Package, error) { return []core.Package{}, nil }
	}
	res, err := app.ExecuteInventoryRemoval(preview.Token)
	if err != nil {
		t.Fatalf("execute removal failed: %v", err)
	}
	if res.State != "succeeded" {
		t.Fatalf("expected succeeded, got: %+v (error: %q)", res, res.Error)
	}
	if _, ok, _ := app.inventory.ledger.Lookup("claude"); ok {
		t.Fatal("claude should be deleted from ledger")
	}
}

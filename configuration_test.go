package main

import (
	"os"
	"path/filepath"
	adapter "shuttle/internal/adapters/configuration"
	core "shuttle/internal/core/configuration"
	"testing"
)

func TestConfigurationDesktopBridge(t *testing.T) {
	root := t.TempDir()
	wt := filepath.Join(root, "settings.json")
	if err := os.WriteFile(wt, []byte(`{"theme":"system","unrelated":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	ps := filepath.Join(root, "profile.ps1")
	store := adapter.NewFileOperationStore(filepath.Join(root, "operations"))
	app := NewApp()
	app.configuration = &configurationService{editor: &configurationEditor{wt: adapter.NewWTEditor(wt), ps: adapter.NewPSEditor(ps)}, store: store, plans: map[string]core.ChangePlan{}}
	plan, err := app.PlanConfiguration(core.ConfigChoice{SchemaVersion: 1, WT: core.WTChoice{Theme: "dark"}, PS: core.PSCheck{SessionEntry: true}})
	if err != nil || plan.Blocked {
		t.Fatalf("preview %+v %v", plan, err)
	}
	if _, err := os.Stat(ps); !os.IsNotExist(err) {
		t.Fatal("preview wrote Profile")
	}
	result, err := app.ExecuteConfiguration(plan.PlanID)
	if err != nil || !result.Succeeded {
		t.Fatalf("apply %+v %v", result, err)
	}
	if _, err := app.ExecuteConfiguration(plan.PlanID); err == nil {
		t.Fatal("replayed plan")
	}
	ops, err := app.ListConfigurationOperations()
	if err != nil || len(ops) != 1 || ops[0].State != "succeeded" {
		t.Fatalf("operations %+v %v", ops, err)
	}
	restore, err := app.PlanConfigurationRestore(result.OpID)
	if err != nil || restore.Blocked {
		t.Fatalf("restore %+v %v", restore, err)
	}
	undone, err := app.ExecuteConfiguration(restore.PlanID)
	if err != nil || !undone.Succeeded {
		t.Fatalf("undo %+v %v", undone, err)
	}
	raw, err := os.ReadFile(wt)
	if err != nil || string(raw) != `{"theme":"system","unrelated":true}` {
		t.Fatalf("WT %s %v", raw, err)
	}
}

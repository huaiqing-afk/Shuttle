package main

import (
	"fmt"
	"time"

	config "shuttle/internal/core/configuration"
	core "shuttle/internal/core/inventory"
)

// The operation history is separate from the current ownership ledger.
type installRecordingLedger struct {
	core.Ledger
	store config.OperationStore
	id    string
	steps []config.StepResult
}

func beginInstallRecord(store config.OperationStore, ledger core.Ledger, id, token string, plan core.Plan) (*installRecordingLedger, error) {
	tracked := &installRecordingLedger{Ledger: ledger, store: store, id: id}
	for _, step := range plan.Steps {
		if step.Action == core.ActionInstall {
			tracked.steps = append(tracked.steps, config.StepResult{Target: string(step.Source) + ":" + step.Name, Action: "pending"})
		} else if step.Action == core.ActionUninstall {
			tracked.steps = append(tracked.steps, config.StepResult{Target: string(step.Item.Source) + ":" + step.Item.Name, Action: "pending"})
		}
	}
	err := store.Begin(config.Operation{SchemaVersion: 1, ID: id, PlanID: token, State: "running", StartedAt: time.Now().UTC(), Steps: tracked.steps})
	return tracked, err
}

func (l *installRecordingLedger) Put(record core.Record) error {
	if record.State == core.StateIntent {
		if err := l.Ledger.Put(record); err != nil {
			return err
		}
	}
	step := config.StepResult{Target: string(record.Source) + ":" + record.Name, Action: string(record.State), Changed: record.State == core.StateInstalled}
	if record.State != core.StateIntent && record.State != core.StateInstalled {
		step.Err = record.EvidenceAfter
		if record.InstallerError != "" {
			step.Err += ": " + record.InstallerError
		}
	}
	if err := l.store.RecordStep(l.id, step); err != nil {
		return fmt.Errorf("安装操作记录写入失败: %w", err)
	}
	for i := range l.steps {
		if l.steps[i].Target == step.Target {
			l.steps[i] = step
		}
	}
	if record.State != core.StateIntent {
		if err := l.Ledger.Put(record); err != nil {
			return err
		}
	}
	return nil
}

func (l *installRecordingLedger) Delete(name string) error {
	step := config.StepResult{Target: name, Action: "removed", Changed: true}
	_ = l.store.RecordStep(l.id, step)
	for i := range l.steps {
		if l.steps[i].Target == name || (len(l.steps[i].Target) > len(name)+1 && l.steps[i].Target[len(l.steps[i].Target)-len(name)-1:] == ":"+name) {
			l.steps[i] = step
		}
	}
	return l.Ledger.Delete(name)
}

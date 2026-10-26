package configuration

import (
	"errors"
	"fmt"
	"time"
)

// Operation records survive application exits; notifications are not evidence of completion.
type Operation struct {
	SchemaVersion int
	ID            string
	PlanID        string
	State         string
	StartedAt     time.Time
	Steps         []StepResult
}

type OperationStore interface {
	BackupStore
	Lock() (release func(), err error)
	Begin(Operation) error
	RecordStep(id string, step StepResult) error
	Finish(id string, state string, steps []StepResult) error
	Get(id string) (Operation, error)
	List() ([]Operation, error)
	RecoverInterrupted() error
}

// ExecuteRecorded persists intent before writes and each completed write before proceeding.
func ExecuteRecorded(plan ChangePlan, ed Editor, store OperationStore) (OperationResult, error) {
	release, err := store.Lock()
	if err != nil {
		return OperationResult{}, err
	}
	defer release()
	if plan.Blocked {
		return OperationResult{}, &Error{Kind: "conflict", Reason: "计划存在未解决冲突"}
	}
	id := newID("op")
	if err := store.Begin(Operation{SchemaVersion: 1, ID: id, PlanID: plan.PlanID,
		State: "running", StartedAt: time.Now().UTC()}); err != nil {
		return OperationResult{}, err
	}
	tracked := &recordingEditor{Editor: ed, store: store, id: id}
	result, err := executeWithID(plan, tracked, store, id)
	state := "succeeded"
	if !result.Succeeded || err != nil {
		state = "failed"
		for _, step := range result.Steps {
			if step.Changed {
				state = "partially_failed"
			}
		}
	}
	if finishErr := store.Finish(id, state, result.Steps); finishErr != nil {
		result.Succeeded = false
		return result, finishErr
	}
	return result, err
}

type recordingEditor struct {
	Editor
	store OperationStore
	id    string
}

func (e *recordingEditor) Apply(target, value string) (string, bool, error) {
	before, changed, err := e.Editor.Apply(target, value)
	if err == nil {
		current, readErr := e.Editor.ReadCurrent(target)
		if value == "" && errors.Is(readErr, ErrNotFound) {
			readErr = nil
		} else if readErr == nil && current != value {
			readErr = fmt.Errorf("写后验证失败: %s", target)
		}
		err = readErr
	}
	step := StepResult{Target: target, Action: "apply", Changed: changed}
	if err != nil {
		step.Err = err.Error()
	}
	if recordErr := e.store.RecordStep(e.id, step); recordErr != nil {
		return before, changed, recordErr
	}
	return before, changed, err
}

// PlanRecordedRestore is read-only, including for interrupted operations.
// ExecuteRecorded creates a separate operation when the user accepts this plan.
func PlanRecordedRestore(id string, ed Editor, store OperationStore) (ChangePlan, error) {
	if _, err := store.Get(id); err != nil {
		return ChangePlan{}, err
	}
	plan := ChangePlan{PlanID: newID("restore")}
	for _, target := range store.Targets(id) {
		o, a, absent, err := store.Load(id, target)
		if err != nil {
			return ChangePlan{}, err
		}
		c, readErr := ed.ReadCurrent(target)
		missing := readErr == ErrNotFound
		diff := FieldDiff{Target: target, Before: c, WasAbsent: missing, After: o}
		switch {
		case readErr != nil && !missing:
			diff.Conflict = readErr.Error()
		case missing && absent, !missing && !absent && c == o:
			diff.Action = "none"
		case !missing && c == a:
			diff.Action = "modify"
			if absent {
				diff.Action = "remove"
				diff.After = ""
			}
		default:
			diff.Conflict = "当前内容与应用前值、写入值均不同，保留当前内容"
		}
		if diff.Conflict != "" {
			plan.Blocked = true
		}
		plan.Diffs = append(plan.Diffs, diff)
	}
	return plan, nil
}

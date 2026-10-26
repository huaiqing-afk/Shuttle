// Package configuration implements the Plan/Execute/Restore use cases from
// docs/config-contract.md. Pure core: all file access goes through the
// Editor/BackupStore interfaces injected by the entry point.
package configuration

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrNotFound signals a managed target that has no current value. Adapters
// return it from ReadCurrent; absence is a value for O/A/C purposes.
var ErrNotFound = errors.New("managed target not found")

// Error kinds follow the architecture error set; Target names the diff row.
type Error struct {
	Kind   string // validation_failed | unsupported | stale_plan | conflict | not_found
	Target string
	Reason string
}

func (e *Error) Error() string {
	if e.Target == "" {
		return fmt.Sprintf("%s: %s", e.Kind, e.Reason)
	}
	return fmt.Sprintf("%s: %s: %s", e.Kind, e.Target, e.Reason)
}

// ConfigChoice is the user-facing selection submitted from the frontend.
type ConfigChoice struct {
	SchemaVersion int
	WT            WTChoice
	PS            PSCheck
	// Statusline is reserved for a later batch; non-empty is rejected as
	// unsupported (config-contract §1).
	Statusline string
}

type WTChoice struct {
	Theme              string
	FontFace           string
	DefaultProfileGUID string
	ManagedProfile     ProfileChoice
	Keys               []KeyChoice
	// Params are scalar settings addressed by stable JSONC paths, e.g.
	// "profiles.defaults.fontFace" -> "Maple Mono NF CN".
	Params map[string]string
}

type ProfileChoice struct {
	GUID    string
	Payload string // JSON fragment written under the managed profile entry
}

type KeyChoice struct {
	Keys    string // e.g. "ctrl+shift+t"
	Command string // WT command identity
	Value   string // full keybinding entry payload
}

type PSCheck struct {
	SessionEntry bool
}

// FieldDiff is one managed target row in a plan.
type FieldDiff struct {
	Target    string // "wt:scalar:<path>" | "wt:profile:<guid>" | "wt:key:<keys>|<command>" | "ps:managed-block"
	Action    string // add | modify | remove | none
	Before    string // O: value before Shuttle touches the target
	WasAbsent bool   // O did not exist
	After     string // A: value Shuttle will write
	Conflict  string // non-empty means unresolved; Blocked plans refuse execution
}

type ChangePlan struct {
	PlanID  string
	Choice  ConfigChoice
	Diffs   []FieldDiff
	Blocked bool
}

type StepResult struct {
	Target  string
	Action  string
	Changed bool
	Skipped bool
	Err     string
}

type OperationResult struct {
	OpID      string
	Steps     []StepResult
	Succeeded bool
}

// Editor abstracts all file access; adapters implement surgical edits.
type Editor interface {
	// ReadCurrent returns the current value of a target, or ErrNotFound.
	ReadCurrent(target string) (string, error)
	// Check reports a semantic conflict between the desired value and the
	// current document without writing, e.g. same WT profile name with a
	// different GUID (config-contract §4.1). Nil means no conflict.
	Check(target string, desired string) error
	// Apply writes the value surgically; empty value means removing the
	// managed content. Returns the value before the write.
	Apply(target string, value string) (before string, changed bool, err error)
}

// BackupStore keeps O and A per operation; both are required to decide
// C==A vs conflict at restore time (config-contract §5). oAbsent records
// that O did not exist — an empty string is a legitimate value and must not
// be confused with absence.
type BackupStore interface {
	Save(opID string, target string, o string, a string, oAbsent bool) error
	Load(opID string, target string) (o string, a string, oAbsent bool, err error)
	// Targets lists the backed-up targets of one operation, in restore order.
	Targets(opID string) []string
}

// BuildTargets maps a choice to stable target identifiers and desired values,
// sorted for deterministic diffs.
func BuildTargets(choice ConfigChoice) []struct {
	Target string
	Value  string
} {
	var rows []struct {
		Target string
		Value  string
	}
	wt := choice.WT
	add := func(target, value string) { rows = append(rows, struct{ Target, Value string }{target, value}) }
	quoted := func(value string) string { raw, _ := json.Marshal(value); return string(raw) }
	if wt.Theme != "" {
		add("wt:scalar:theme", quoted(wt.Theme))
	}
	if wt.FontFace != "" {
		add("wt:scalar:profiles.defaults.fontFace", quoted(wt.FontFace))
	}
	if wt.DefaultProfileGUID != "" {
		add("wt:scalar:defaultProfile", quoted(wt.DefaultProfileGUID))
	}
	if wt.ManagedProfile.GUID != "" {
		add("wt:profile:"+wt.ManagedProfile.GUID, wt.ManagedProfile.Payload)
	}
	for _, key := range wt.Keys {
		add("wt:key:"+key.Keys+"|"+key.Command, key.Value)
	}
	for path, value := range wt.Params {
		add("wt:scalar:"+path, value)
	}
	if choice.PS.SessionEntry {
		add("ps:managed-block", ProfileBlock)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Target < rows[j].Target })
	return rows
}

// Plan is read-only: it diffs the choice against current values. Any read
// error (including adapter-reported conflicts) becomes a Conflict row instead
// of failing the whole plan, so unrelated targets stay actionable.
func Plan(choice ConfigChoice, ed Editor) (ChangePlan, error) {
	if choice.SchemaVersion != 1 {
		return ChangePlan{}, &Error{Kind: "validation_failed", Reason: fmt.Sprintf("schemaVersion %d 不受支持", choice.SchemaVersion)}
	}
	if choice.Statusline != "" {
		return ChangePlan{}, &Error{Kind: "unsupported", Reason: "Claude 状态栏预设本批次未实现"}
	}
	plan := ChangePlan{PlanID: newID("plan"), Choice: choice}
	for _, row := range BuildTargets(choice) {
		current, err := ed.ReadCurrent(row.Target)
		diff := FieldDiff{Target: row.Target, After: row.Value, Before: current}
		switch {
		case errors.Is(err, ErrNotFound):
			diff.WasAbsent = true
			diff.Action = "add"
		case err != nil:
			var cfgErr *Error
			if errors.As(err, &cfgErr) {
				diff.Conflict = cfgErr.Reason
			} else {
				diff.Conflict = err.Error()
			}
			diff.Before = ""
		case current == row.Value:
			diff.Action = "none"
		default:
			diff.Action = "modify"
		}
		if diff.Conflict != "" {
			plan.Blocked = true
		} else if err := ed.Check(row.Target, row.Value); err != nil {
			var cfgErr *Error
			if errors.As(err, &cfgErr) {
				diff.Conflict = cfgErr.Reason
			} else {
				diff.Conflict = err.Error()
			}
			plan.Blocked = true
		}
		plan.Diffs = append(plan.Diffs, diff)
	}
	return plan, nil
}

// Execute re-checks the preconditions (stale_plan on any drift), then applies
// each pending diff after saving its backup. A blocked plan is refused.
func Execute(plan ChangePlan, ed Editor, bs BackupStore) (OperationResult, error) {
	if plan.Blocked {
		return OperationResult{}, &Error{Kind: "validation_failed", Reason: "计划存在未解决冲突，拒绝执行"}
	}
	return executeWithID(plan, ed, bs, newID("op"))
}

func executeWithID(plan ChangePlan, ed Editor, bs BackupStore, opID string) (OperationResult, error) {
	result := OperationResult{OpID: opID, Succeeded: true}
	for _, diff := range plan.Diffs {
		if diff.Action == "none" {
			result.Steps = append(result.Steps, StepResult{Target: diff.Target, Action: diff.Action, Skipped: true})
			continue
		}
		current, err := ed.ReadCurrent(diff.Target)
		if err != nil && !errors.Is(err, ErrNotFound) {
			result.Steps = append(result.Steps, StepResult{Target: diff.Target, Action: diff.Action, Err: err.Error()})
			result.Succeeded = false
			continue
		}
		wasAbsent := errors.Is(err, ErrNotFound)
		if current != diff.Before || wasAbsent != diff.WasAbsent {
			result.Steps = append(result.Steps, StepResult{Target: diff.Target, Action: diff.Action, Err: "前置状态已变化"})
			result.Succeeded = false
			continue
		}
		if err := bs.Save(opID, diff.Target, diff.Before, diff.After, diff.WasAbsent); err != nil {
			result.Steps = append(result.Steps, StepResult{Target: diff.Target, Action: diff.Action, Err: "备份失败: " + err.Error()})
			result.Succeeded = false
			continue
		}
		_, changed, err := ed.Apply(diff.Target, diff.After)
		if err != nil {
			result.Steps = append(result.Steps, StepResult{Target: diff.Target, Action: diff.Action, Changed: changed, Err: err.Error()})
			result.Succeeded = false
			return result, err
		}
		result.Steps = append(result.Steps, StepResult{Target: diff.Target, Action: diff.Action, Changed: changed})
	}
	return result, nil
}

// RestoreAction is the O/A/C decision for one target.
type RestoreAction string

const (
	RestoreToOriginal RestoreAction = "restore"  // C==A: put O back
	RestoreRemove     RestoreAction = "remove"   // C==A and O absent: delete managed content
	RestoreSkip       RestoreAction = "skip"     // C==O: already restored
	RestoreConflict   RestoreAction = "conflict" // user changed it; keep current by default
	RestoreManual     RestoreAction = "manual"   // file lost/unreadable; never force-write
)

// DecideRestore is the pure O/A/C decision function (config-contract §5).
func DecideRestore(o, a, c string, oAbsent bool) RestoreAction {
	switch {
	case c == a:
		if oAbsent {
			return RestoreRemove
		}
		return RestoreToOriginal
	case c == o:
		return RestoreSkip
	default:
		return RestoreConflict
	}
}

// Restore walks one operation's backups and applies the O/A/C rules. Conflicts
// keep the current value; unreadable targets become manual items.
func Restore(opID string, ed Editor, bs BackupStore) (OperationResult, error) {
	if bs == nil {
		return OperationResult{}, &Error{Kind: "validation_failed", Reason: "缺少备份存储"}
	}
	targets := bs.Targets(opID)
	if len(targets) == 0 {
		return OperationResult{}, &Error{Kind: "not_found", Reason: "没有该操作的可还原备份"}
	}
	result := OperationResult{OpID: opID, Succeeded: true}
	for _, target := range targets {
		o, a, oAbsent, err := bs.Load(opID, target)
		if err != nil {
			result.Steps = append(result.Steps, StepResult{Target: target, Action: string(RestoreManual), Err: "读取备份失败: " + err.Error()})
			result.Succeeded = false
			continue
		}
		current, err := ed.ReadCurrent(target)
		if err != nil && !errors.Is(err, ErrNotFound) {
			result.Steps = append(result.Steps, StepResult{Target: target, Action: string(RestoreManual), Err: "读取现状失败: " + err.Error()})
			result.Succeeded = false
			continue
		}
		action := DecideRestore(o, a, current, oAbsent)
		switch action {
		case RestoreToOriginal, RestoreRemove:
			value := o
			if action == RestoreRemove {
				value = ""
			}
			if _, changed, applyErr := ed.Apply(target, value); applyErr != nil {
				result.Steps = append(result.Steps, StepResult{Target: target, Action: string(action), Err: applyErr.Error()})
				result.Succeeded = false
			} else {
				result.Steps = append(result.Steps, StepResult{Target: target, Action: string(action), Changed: changed})
			}
		default:
			result.Steps = append(result.Steps, StepResult{Target: target, Action: string(action), Skipped: true})
		}
	}
	return result, nil
}

func newID(prefix string) string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(buf)
}

package inventory

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Installer runs one argv (never a shell string, D7). It returns the exit code
// and the first line of output that explains a failure.
type Installer interface {
	Run(argv []string) (exitCode int, message string, err error)
}

// Snapshot re-reads what is installed. Execute re-detects after every install
// instead of trusting the installer's own claim.
type Snapshot func() ([]Package, error)

// PlanSource regenerates the plan from current state. Execute refuses a plan
// whose steps no longer match, so a stale plan cannot install something the
// user never saw (architecture §4 stale_plan).
type PlanSource func() (Plan, error)

// Execution is the recorded outcome of one install run.
type Execution struct {
	OperationID string
	State       string // "succeeded" | "partially_failed" | "failed"
	Records     []Record
}

// ErrStalePlan marks a plan generated against state that has since changed.
var ErrStalePlan = &Error{Kind: "stale_plan", Reason: "计划生成后环境已变化，请重新生成计划"}

// ExecuteInstall carries out the install steps of a plan (contract §10.5/§10.7).
// Order is fixed: intent → installer → recheck → terminal state. Every record
// is written before the next step starts, so a crash leaves evidence of what
// was attempted instead of a silent gap.
func ExecuteInstall(plan Plan, operationID string, ledger Ledger, install Installer,
	snapshot Snapshot, current PlanSource) (Execution, error) {
	if ledger == nil || install == nil || snapshot == nil {
		return Execution{}, &Error{Kind: "validation_failed", Reason: "安装执行缺少必要依赖"}
	}
	if operationID == "" {
		return Execution{}, &Error{Kind: "validation_failed", Reason: "缺少操作编号"}
	}
	if plan.Blocked {
		return Execution{}, &Error{Kind: "conflict", Reason: "计划存在未解决冲突"}
	}
	if current != nil {
		if err := assertFresh(plan, current); err != nil {
			return Execution{}, err
		}
	}

	execution := Execution{OperationID: operationID, State: "succeeded"}
	for _, step := range plan.Steps {
		if step.Action != ActionInstall {
			continue
		}
		record, err := installOne(step, operationID, ledger, install, snapshot)
		execution.Records = append(execution.Records, record)
		if err != nil {
			execution.State = "failed"
			if len(execution.Records) > 1 {
				execution.State = "partially_failed"
			}
			return execution, err
		}
		if record.State != StateInstalled {
			execution.State = "partially_failed"
		}
	}
	return execution, nil
}

func installOne(step Step, operationID string, ledger Ledger, install Installer,
	snapshot Snapshot) (Record, error) {
	args := step.Args
	if len(args) == 0 {
		return Record{}, &Error{Kind: "validation_failed", Reason: fmt.Sprintf("%s：缺少可执行命令", step.Name)}
	}
	intent := Record{
		Name: step.Name, Source: step.Source, Root: step.Root, OperationID: operationID,
		State: StateIntent, EvidenceBefore: "absent",
	}
	// intent first: a crash between here and the terminal write must stay
	// explainable as "attempted, unproven".
	if err := ledger.Put(intent); err != nil {
		return intent, fmt.Errorf("%s：写入安装意图失败: %w", step.Name, err)
	}

	code, message, runErr := install.Run(args)
	if code != 0 && runErr == nil {
		runErr = fmt.Errorf("安装器退出码 %d", code)
	}
	detected, snapErr := snapshot()
	record := VerifyInstalled(intent, detected)
	record.InstallerExit = code
	if runErr != nil {
		record.InstallerError = message
	}
	if snapErr != nil {
		// we cannot prove anything about the result: keep it unverified
		record.State = StateUnverified
		record.EvidenceAfter = "安装后重新检测失败: " + snapErr.Error()
	} else if runErr != nil {
		// The installer failed. If the recheck still finds the package we must
		// not claim it as ours (contract §10.7): downgrade to unverified.
		if record.State == StateInstalled {
			record.State = StateUnverified
			record.EvidenceAfter += "（安装器报错，但检测到该包）"
		} else {
			record.State = StateFailed
		}
		if message == "" {
			record.InstallerError = runErr.Error()
		}
	}
	if record.State == StateInstalled {
		record.InstalledAt = time.Now().UTC().Format(time.RFC3339)
		// the root comes from the detection, not from the plan
		for _, pkg := range detected {
			if pkg.Name == step.Name && pkg.Source == step.Source {
				record.Root = pkg.Root
				break
			}
		}
	}
	if err := ledger.Put(record); err != nil {
		// the install may well have happened; the ledger is what failed. The
		// caller must report a failed operation (contract §10.5 item 3).
		return record, fmt.Errorf("%s：安装结果写入台账失败（该项不会被认定为 Shuttle 安装）: %w", step.Name, err)
	}
	if runErr != nil {
		return record, fmt.Errorf("%s：安装器失败: %w", step.Name, runErr)
	}
	return record, nil
}

// assertFresh compares the plan against a freshly generated one. Any difference
// in what would be installed means the user's view is out of date.
func assertFresh(plan Plan, current PlanSource) error {
	fresh, err := current()
	if err != nil {
		return fmt.Errorf("重新生成安装计划失败: %w", err)
	}
	want := planSignature(plan)
	got := planSignature(fresh)
	if want != got {
		return ErrStalePlan
	}
	return nil
}

func planSignature(plan Plan) string {
	type planStep struct {
		Action  string
		Package Package
		Item    Item
		Command string
		Args    []string
	}
	signature := struct {
		Blocked bool
		Steps   []planStep
	}{Blocked: plan.Blocked}
	for _, step := range plan.Steps {
		signature.Steps = append(signature.Steps, planStep{
			Action:  step.Action,
			Package: step.Package,
			Item:    step.Item,
			Command: step.Command,
			Args:    step.Args,
		})
	}
	encoded, _ := json.Marshal(signature)
	return string(encoded)
}

// ReconcileInterrupted closes records left in "intent" by a crash (contract
// §10.7). It never retries an install: it re-detects and records what is true.
func ReconcileInterrupted(ledger Ledger, snapshot Snapshot) ([]Record, error) {
	if ledger == nil || snapshot == nil {
		return nil, &Error{Kind: "validation_failed", Reason: "核对缺少必要依赖"}
	}
	records, err := ledger.List()
	if err != nil {
		return nil, err
	}
	var pending []Record
	for _, record := range records {
		if record.State == StateIntent {
			pending = append(pending, record)
		}
	}
	if len(pending) == 0 {
		return nil, nil
	}
	detected, err := snapshot()
	if err != nil {
		return nil, fmt.Errorf("核对未完成安装时检测失败: %w", err)
	}
	var closed []Record
	for _, record := range pending {
		final := VerifyInstalled(record, detected)
		final.State = StateUnverified
		final.InstalledAt = ""
		final.InstallerError = "上次安装未收尾，缺少安装器成功证据，需人工核对"
		if err := ledger.Put(final); err != nil {
			return closed, fmt.Errorf("%s：核对结果写入台账失败: %w", record.Name, err)
		}
		closed = append(closed, final)
	}
	return closed, nil
}

// IsStale reports whether an error means the plan must be regenerated.
func IsStale(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Kind == "stale_plan"
}

// ExecuteUninstall carries out the uninstall steps of a plan (contract docs/uninstall-contract.md).
func ExecuteUninstall(plan Plan, operationID string, ledger Ledger, install Installer,
	snapshot Snapshot, current PlanSource) (Execution, error) {
	if ledger == nil || install == nil || snapshot == nil {
		return Execution{}, &Error{Kind: "validation_failed", Reason: "卸载执行缺少必要依赖"}
	}
	if operationID == "" {
		return Execution{}, &Error{Kind: "validation_failed", Reason: "缺少操作编号"}
	}
	if plan.Blocked {
		return Execution{}, &Error{Kind: "conflict", Reason: "计划存在未解决冲突"}
	}
	if current != nil {
		if err := assertFresh(plan, current); err != nil {
			return Execution{}, err
		}
	}

	execution := Execution{OperationID: operationID, State: "succeeded"}
	for _, step := range plan.Steps {
		if step.Action != ActionUninstall {
			continue
		}
		record, err := uninstallOne(step, operationID, ledger, install, snapshot)
		execution.Records = append(execution.Records, record)
		if err != nil {
			execution.State = "failed"
			if len(execution.Records) > 1 {
				execution.State = "partially_failed"
			}
			return execution, err
		}
		if record.State != "" {
			execution.State = "partially_failed"
		}
	}
	return execution, nil
}

func uninstallOne(step Step, operationID string, ledger Ledger, install Installer,
	snapshot Snapshot) (Record, error) {
	args := step.Args
	if len(args) == 0 {
		return Record{}, &Error{Kind: "validation_failed", Reason: fmt.Sprintf("%s：缺少卸载命令", step.Item.Name)}
	}
	intent := Record{
		Name: step.Item.Name, Source: step.Item.Source, Root: step.Item.Root, OperationID: operationID,
		State: StateIntent, EvidenceBefore: "present",
	}
	if err := ledger.Put(intent); err != nil {
		return intent, fmt.Errorf("%s：写入卸载意图失败: %w", step.Item.Name, err)
	}

	code, message, runErr := install.Run(args)
	if code != 0 && runErr == nil {
		runErr = fmt.Errorf("卸载器退出码 %d", code)
	}
	detected, snapErr := snapshot()
	if snapErr != nil {
		unverified := intent
		unverified.State = StateUnverified
		unverified.InstallerExit = code
		unverified.InstallerError = snapErr.Error()
		_ = ledger.Put(unverified)
		return unverified, fmt.Errorf("%s：卸载后复检失败: %w", step.Item.Name, snapErr)
	}

	stillPresent := false
	for _, p := range detected {
		if p.Name == step.Item.Name && p.Source == step.Item.Source {
			stillPresent = true
			break
		}
	}

	if !stillPresent && runErr == nil {
		if err := ledger.Delete(step.Item.Name); err != nil {
			unverified := intent
			unverified.State = StateUnverified
			unverified.InstallerError = fmt.Sprintf("删除台账失败: %v", err)
			_ = ledger.Put(unverified)
			return unverified, fmt.Errorf("%s：从台账删除失败: %w", step.Item.Name, err)
		}
		return Record{Name: step.Item.Name, Source: step.Item.Source, Root: step.Item.Root, OperationID: operationID, State: ""}, nil
	}

	unverified := intent
	unverified.State = StateUnverified
	unverified.InstallerExit = code
	if runErr != nil {
		unverified.InstallerError = runErr.Error()
	} else if message != "" {
		unverified.InstallerError = message
	} else {
		unverified.InstallerError = "卸载后复检仍存在该包"
	}
	if err := ledger.Put(unverified); err != nil {
		return unverified, fmt.Errorf("%s：写入卸载异常状态失败: %w", step.Item.Name, err)
	}
	return unverified, runErr
}

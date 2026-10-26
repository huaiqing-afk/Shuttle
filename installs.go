package main

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"path/filepath"
	configadapter "shuttle/internal/adapters/configuration"
	core "shuttle/internal/core/inventory"
)

// Component is one thing Shuttle knows how to look for or install. The set is
// fixed by docs/architecture.md §2; ids are the real ecosystem identifiers so a
// preview shows the exact package name that would be installed.
type Component struct {
	ID        string
	Name      string
	Source    string
	PackageID string
}

var installableComponents = []Component{
	{ID: "powershell", Name: "PowerShell 7", Source: "winget", PackageID: "Microsoft.PowerShell"},
	{ID: "git", Name: "Git", Source: "winget", PackageID: "Git.Git"},
	{ID: "wt", Name: "Windows Terminal", Source: "winget", PackageID: "Microsoft.WindowsTerminal"},
	{ID: "node", Name: "Node.js (LTS)", Source: "winget", PackageID: "OpenJS.NodeJS.LTS"},
	{ID: "claude", Name: "Claude Code", Source: "npm", PackageID: "@anthropic-ai/claude-code"},
	{ID: "codex", Name: "Codex", Source: "npm", PackageID: "@openai/codex"},
}

// InstallPreview is what the UI shows before anything runs.
type InstallPreview struct {
	Token string
	Plan  InventoryPlan
}

type InventoryRecord struct {
	Name           string
	Source         string
	Version        string
	State          string
	OperationID    string
	InstallerError string
}

type InstallResult struct {
	Token       string
	OperationID string
	State       string
	Records     []InventoryRecord
	Error       string
}

// previewLife bounds a preview. It is the user's confirmation that they saw
// these exact commands, so it must not stay valid indefinitely.
const previewLife = 10 * time.Minute

type installPreview struct {
	token   string
	plan    core.Plan
	created time.Time
}

type removalPreview struct {
	token   string
	plan    core.Plan
	names   []string
	created time.Time
}

func componentByID(id string) (Component, bool) {
	for _, component := range installableComponents {
		if component.ID == id {
			return component, true
		}
	}
	return Component{}, false
}

// desiredPackages turns component ids into the packages PlanInstall expects.
func desiredPackages(ids []string) ([]core.Package, error) {
	if len(ids) == 0 {
		return nil, &core.Error{Kind: "validation_failed", Reason: "未选择任何组件"}
	}
	packages := make([]core.Package, 0, len(ids))
	for _, id := range ids {
		component, ok := componentByID(id)
		if !ok {
			return nil, &core.Error{Kind: "validation_failed", Reason: fmt.Sprintf("未知组件 %q", id)}
		}
		packages = append(packages, core.Package{Name: component.PackageID, Source: core.Source(component.Source)})
	}
	return packages, nil
}

// PlanInstall previews an install. Read-only: detect, classify, plan. The plan
// is kept behind a token the UI must echo back, so nothing runs before the user
// has seen the commands (contract §10.10 D8).
func (a *App) PlanInstall(componentIDs []string) (InstallPreview, error) {
	s := a.inventory
	if s == nil {
		return InstallPreview{}, errors.New("归属服务尚未就绪")
	}
	if s.initErr != nil {
		return InstallPreview{}, s.initErr
	}
	packages, err := desiredPackages(componentIDs)
	if err != nil {
		return InstallPreview{}, err
	}
	detected, err := a.detectPackages()
	if err != nil {
		return InstallPreview{}, err
	}
	plan, err := core.PlanInstall(packages, core.Classify(detected, s.ledger))
	if err != nil {
		return InstallPreview{}, err
	}
	preview := &installPreview{token: "preview-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		plan: plan, created: time.Now()}
	s.mu.Lock()
	s.preview = preview
	s.mu.Unlock()
	return InstallPreview{Token: preview.token, Plan: inventoryPlanView(plan)}, nil
}

// ExecuteInstall runs the previewed plan. A missing, reused, mismatched or
// expired preview is refused: there is no path that installs without one.
func (a *App) ExecuteInstall(previewToken string) (InstallResult, error) {
	s := a.inventory
	if s == nil {
		return InstallResult{}, errors.New("归属服务尚未就绪")
	}
	if s.initErr != nil {
		return InstallResult{}, s.initErr
	}
	s.mu.Lock()
	preview := s.preview
	s.mu.Unlock()
	if preview == nil {
		return InstallResult{}, &core.Error{Kind: "validation_failed", Reason: "没有待执行的安装预览，请先生成预览"}
	}
	// Validate before consuming: a typo must not destroy the preview the user
	// just generated. Consumption happens under the lock below, so only one
	// caller can win the race.
	if preview.token != previewToken {
		return InstallResult{}, &core.Error{Kind: "validation_failed", Reason: "预览编号不匹配，请重新生成预览"}
	}
	s.mu.Lock()
	if s.preview != preview {
		s.mu.Unlock()
		return InstallResult{}, &core.Error{Kind: "validation_failed", Reason: "预览已被使用，请重新生成"}
	}
	// consume before running: one preview authorizes exactly one execution,
	// even if the install itself fails
	s.preview = nil
	s.mu.Unlock()
	if time.Since(preview.created) > previewLife {
		return InstallResult{}, &core.Error{Kind: "stale_plan", Reason: "安装预览已过期，请重新生成"}
	}
	if preview.plan.Blocked {
		return InstallResult{}, &core.Error{Kind: "conflict", Reason: "计划存在未解决冲突"}
	}

	if !a.installBusy.CompareAndSwap(false, true) {
		return InstallResult{}, errors.New("已有安装操作正在运行")
	}
	defer a.installBusy.Store(false)
	release, err := a.lockInstall()
	if err != nil {
		return InstallResult{}, err
	}
	defer release()

	s.mu.Lock()
	installer := s.installer
	s.mu.Unlock()
	if installer == nil {
		installer = commandInstaller{}
	}
	packages := packagesOf(preview.plan)
	operationID := "op-install-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	store := configadapter.NewFileOperationStore(filepath.Join(s.root, "operations"))
	tracked, err := beginInstallRecord(store, s.ledger, operationID, previewToken, preview.plan)
	if err != nil {
		return InstallResult{}, err
	}
	execution, executeErr := core.ExecuteInstall(preview.plan, operationID, tracked, installer,
		a.detectPackages, a.planSource(packages))
	if execution.State == "" {
		execution.State = "failed"
	}
	finishErr := store.Finish(operationID, execution.State, tracked.steps)
	result := InstallResult{Token: previewToken, OperationID: operationID,
		State: execution.State, Records: toRecordViews(execution.Records)}
	if executeErr != nil && len(execution.Records) == 0 && finishErr == nil {
		return result, executeErr
	}
	if combined := errors.Join(executeErr, finishErr); combined != nil {
		result.Error = combined.Error()
		if finishErr != nil {
			result.State = "failed"
		}
	} else if execution.State != "succeeded" {
		result.Error = fmt.Sprintf("安装未全部成功（%s），请核对每项结果", execution.State)
	}
	return result, nil
}

// ReconcileInventory closes installs left unfinished by a crash. It runs at
// startup and never retries an install (contract §10.7).
func (a *App) ReconcileInventory() ([]InventoryRecord, error) {
	s := a.inventory
	if s == nil {
		return nil, errors.New("归属服务尚未就绪")
	}
	if s.initErr != nil {
		return nil, s.initErr
	}
	release, err := a.lockInstall()
	if err != nil {
		return nil, err
	}
	defer release()
	closed, err := core.ReconcileInterrupted(s.ledger, a.detectPackages)
	if err != nil {
		return nil, err
	}
	return toRecordViews(closed), nil
}

// planSource regenerates the plan from current state so ExecuteInstall can
// refuse one whose targets changed since the preview (stale_plan).
func (a *App) planSource(packages []core.Package) core.PlanSource {
	return func() (core.Plan, error) {
		detected, err := a.detectPackages()
		if err != nil {
			return core.Plan{}, err
		}
		return core.PlanInstall(packages, core.Classify(detected, a.inventory.ledger))
	}
}

// lockInstall reuses the configuration operation lock: the ledger lives beside
// operations/ and "one install at a time" is the same rule as "one write at a
// time" (contract §10.5). It also keeps two Shuttle instances from installing
// simultaneously.
func (a *App) lockInstall() (func(), error) {
	if a.inventory == nil || a.inventory.root == "" {
		return nil, errors.New("安装操作目录尚未就绪")
	}
	return configadapter.NewFileOperationStore(filepath.Join(a.inventory.root, "operations")).Lock()
}

func (a *App) detectPackages() ([]core.Package, error) {
	packages, report, err := detectInventory(a.inventory)
	if err != nil {
		return nil, err
	}
	for _, source := range report.Sources {
		if !source.Available {
			return nil, fmt.Errorf("安装检测不完整，%s 不可用: %s；请修复后重新预览", source.Name, source.Error)
		}
	}
	return packages, nil
}

func packagesOf(plan core.Plan) []core.Package {
	out := make([]core.Package, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		if step.Action == core.ActionInstall {
			out = append(out, step.Package)
		}
	}
	return out
}

func toRecordViews(records []core.Record) []InventoryRecord {
	out := make([]InventoryRecord, 0, len(records))
	for _, record := range records {
		out = append(out, InventoryRecord{
			Name: record.Name, Source: string(record.Source), Version: record.Version,
			State: string(record.State), OperationID: record.OperationID,
			InstallerError: record.InstallerError,
		})
	}
	return out
}

// ExecuteInventoryRemoval runs the previewed removal plan.
func (a *App) ExecuteInventoryRemoval(previewToken string) (InstallResult, error) {
	s := a.inventory
	if s == nil {
		return InstallResult{}, errors.New("归属服务尚未就绪")
	}
	if s.initErr != nil {
		return InstallResult{}, s.initErr
	}
	s.mu.Lock()
	preview := s.removalPreview
	s.mu.Unlock()
	if preview == nil {
		return InstallResult{}, &core.Error{Kind: "validation_failed", Reason: "没有待执行的卸载预览，请先生成预览"}
	}
	if preview.token != previewToken {
		return InstallResult{}, &core.Error{Kind: "validation_failed", Reason: "预览编号不匹配，请重新生成预览"}
	}
	s.mu.Lock()
	if s.removalPreview != preview {
		s.mu.Unlock()
		return InstallResult{}, &core.Error{Kind: "validation_failed", Reason: "预览已被使用，请重新生成"}
	}
	s.removalPreview = nil
	s.mu.Unlock()

	if time.Since(preview.created) > previewLife {
		return InstallResult{}, &core.Error{Kind: "stale_plan", Reason: "卸载预览已过期，请重新生成"}
	}
	if preview.plan.Blocked {
		return InstallResult{}, &core.Error{Kind: "conflict", Reason: "计划存在未解决冲突"}
	}

	if !a.installBusy.CompareAndSwap(false, true) {
		return InstallResult{}, errors.New("已有安装或卸载操作正在运行")
	}
	defer a.installBusy.Store(false)
	release, err := a.lockInstall()
	if err != nil {
		return InstallResult{}, err
	}
	defer release()

	s.mu.Lock()
	installer := s.installer
	s.mu.Unlock()
	if installer == nil {
		installer = commandInstaller{}
	}

	operationID := "op-rm-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	store := configadapter.NewFileOperationStore(filepath.Join(s.root, "operations"))
	tracked, err := beginInstallRecord(store, s.ledger, operationID, previewToken, preview.plan)
	if err != nil {
		return InstallResult{}, fmt.Errorf("记录卸载操作失败: %w", err)
	}

	snapshot := a.detectPackages

	currentPlanSource := func() (core.Plan, error) {
		report, err := a.DetectInventory()
		if err != nil {
			return core.Plan{}, err
		}
		items := make([]core.Item, 0, len(report.Items))
		for _, item := range report.Items {
			items = append(items, core.Item{
				Package:   core.Package{Name: item.Name, Version: item.Version, Source: core.Source(item.Source), Root: item.Root},
				Ownership: core.Ownership(item.Ownership), Reason: item.Reason,
			})
		}
		return core.PlanUninstall(items, core.Choice{Names: preview.names})
	}

	execution, executeErr := core.ExecuteUninstall(preview.plan, operationID, tracked, installer, snapshot, currentPlanSource)
	if execution.State == "" {
		execution.State = "failed"
	}
	finishErr := store.Finish(operationID, execution.State, tracked.steps)
	result := InstallResult{
		Token:       previewToken,
		OperationID: operationID,
		State:       execution.State,
		Records:     toRecordViews(execution.Records),
	}
	if executeErr != nil && len(execution.Records) == 0 && finishErr == nil {
		return result, executeErr
	}
	if combined := errors.Join(executeErr, finishErr); combined != nil {
		result.Error = combined.Error()
		if finishErr != nil {
			result.State = "failed"
		}
	} else if execution.State != "succeeded" {
		result.Error = fmt.Sprintf("卸载未全部成功（%s），请核对每项结果", execution.State)
	}
	return result, nil
}

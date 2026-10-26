package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	adapter "shuttle/internal/adapters/inventory"
	core "shuttle/internal/core/inventory"
)

// inventoryService owns detection, previews and installation dependencies.
type inventoryService struct {
	mu             sync.Mutex // guards preview + test-injected fields
	detect         sync.Mutex // one detection at a time; npm/winget are slow
	ledger         core.Ledger
	root           string // where winget export writes its temporary JSON
	note           string
	initErr        error
	preview        *installPreview
	removalPreview *removalPreview
	// test replacements; production leaves them nil
	detectPackages func() ([]core.Package, error)
	installer      core.Installer
}

// InventoryItem is the bridge shape: ownership plus the reason the user must see.
type InventoryItem struct {
	Name      string
	Version   string
	Source    string
	Root      string
	Ownership string
	Reason    string
}

type InventoryReport struct {
	Items   []InventoryItem
	Sources []InventorySource
	Note    string
}

// InventorySource reports one collector's outcome so a failed source is never
// displayed as "nothing installed" (contract §6).
type InventorySource struct {
	Name      string
	Count     int
	Available bool
	Error     string
}

type InventoryPaths struct {
	Ledger string
}

type RemovalPreview struct {
	Token string
	Plan  InventoryPlan
}

func newInventoryService() *inventoryService {
	s := &inventoryService{note: adapter.CoverageNote}
	local, err := os.UserCacheDir()
	if err != nil {
		s.initErr = err
		return s
	}
	base := filepath.Join(local, "Shuttle")
	if root := os.Getenv("SHUTTLE_CONFIG_TEST_ROOT"); root != "" {
		if !filepath.IsAbs(root) {
			s.initErr = errors.New("隔离目录必须为绝对路径")
			return s
		}
		base = root
	}
	s.root = base
	s.ledger = adapter.NewFileLedger(filepath.Join(base, "install-ledger.json"))
	return s
}

func (a *App) InventoryPaths() (InventoryPaths, error) {
	s := a.inventory
	if s == nil {
		return InventoryPaths{}, errors.New("归属服务尚未就绪")
	}
	if s.initErr != nil {
		return InventoryPaths{}, s.initErr
	}
	return InventoryPaths{Ledger: filepath.Join(s.root, "install-ledger.json")}, nil
}

// DetectInventory runs both read-only collectors and classifies the result.
func (a *App) DetectInventory() (InventoryReport, error) {
	s := a.inventory
	if s == nil {
		return InventoryReport{}, errors.New("归属服务尚未就绪")
	}
	if s.initErr != nil {
		return InventoryReport{}, s.initErr
	}
	packages, report, err := detectInventory(s)
	if err != nil {
		return report, err
	}
	if a.reconcileError != nil {
		report.Note += "；启动安装核对失败：" + a.reconcileError.Error()
	} else if a.reconciled > 0 {
		report.Note += fmt.Sprintf("；已核对 %d 项中断安装，缺少成功证据的项目仍不可卸载", a.reconciled)
	}
	report.Items = classifyItems(packages, s.ledger)
	return report, nil
}

// detectInventory is the shared read path: it returns raw packages plus the
// per-source outcome. Detection is serialized independently of operation writes.
func detectInventory(s *inventoryService) ([]core.Package, InventoryReport, error) {
	s.detect.Lock()
	defer s.detect.Unlock()
	if s.detectPackages != nil {
		packages, err := s.detectPackages()
		report := InventoryReport{Note: s.note, Sources: []InventorySource{{Name: "注入", Available: true}}}
		return packages, report, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), adapter.Timeout)
	defer cancel()
	collector := adapter.Collector{Run: runCommand}

	var packages []core.Package
	var sources []InventorySource
	npm := collector.CollectNPM(ctx)
	sources = append(sources, describe("npm", npm))
	if npm.Err == nil {
		packages = append(packages, npm.Packages...)
	}
	exportPath := filepath.Join(os.TempDir(), fmt.Sprintf("shuttle-winget-%d.json", os.Getpid()))
	winget := collector.CollectWinget(ctx, exportPath)
	sources = append(sources, describe("winget", winget))
	if winget.Err == nil {
		packages = append(packages, winget.Packages...)
	}
	_ = os.Remove(exportPath)
	report := InventoryReport{Sources: sources, Note: s.note}
	if npm.Err != nil && winget.Err != nil {
		return nil, report, fmt.Errorf("所有归属数据源均不可用: %v; %v", npm.Err, winget.Err)
	}
	return packages, report, nil
}

func classifyItems(packages []core.Package, ledger core.Ledger) []InventoryItem {
	items := core.Classify(packages, ledger)
	out := make([]InventoryItem, 0, len(items))
	for _, item := range items {
		out = append(out, InventoryItem{
			Name: item.Name, Version: item.Version, Source: string(item.Source), Root: item.Root,
			Ownership: string(item.Ownership), Reason: item.Reason})
	}
	return out
}

func describe(name string, result adapter.Result) InventorySource {
	source := InventorySource{Name: name, Count: len(result.Packages), Available: result.Err == nil}
	if result.Err != nil {
		source.Error = result.Err.Error()
	}
	return source
}

// PlanInventoryRemoval builds an uninstall preview with a single-use token.
func (a *App) PlanInventoryRemoval(names []string) (RemovalPreview, error) {
	s := a.inventory
	if s == nil {
		return RemovalPreview{}, errors.New("归属服务尚未就绪")
	}
	report, err := a.DetectInventory()
	if err != nil {
		return RemovalPreview{}, err
	}
	items := make([]core.Item, 0, len(report.Items))
	for _, item := range report.Items {
		items = append(items, core.Item{
			Package:   core.Package{Name: item.Name, Version: item.Version, Source: core.Source(item.Source), Root: item.Root},
			Ownership: core.Ownership(item.Ownership), Reason: item.Reason})
	}
	plan, err := core.PlanUninstall(items, core.Choice{Names: names})
	if err != nil {
		return RemovalPreview{}, err
	}
	token := "preview-rm-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	s.mu.Lock()
	s.removalPreview = &removalPreview{token: token, plan: plan, names: names, created: time.Now()}
	s.mu.Unlock()
	return RemovalPreview{Token: token, Plan: inventoryPlanView(plan)}, nil
}

// runCommand is the real inspection runner; it enforces the timeout itself too,
// because a hung child must not outlive the detection.
func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	path, commandArgs, err := resolveCommand(name, args...)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, path, commandArgs...)
	output, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return nil, fmt.Errorf("%s 退出码 %d: %s", name, exit.ExitCode(), firstLine(string(exit.Stderr)))
		}
		return nil, fmt.Errorf("%s 执行失败: %w", name, err)
	}
	return output, nil
}

func firstLine(text string) string {
	for i, r := range text {
		if r == '\n' || r == '\r' {
			return text[:i]
		}
	}
	return text
}

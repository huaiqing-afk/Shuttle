package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	adapter "shuttle/internal/adapters/configuration"
	core "shuttle/internal/core/configuration"
)

type configurationService struct {
	mu      sync.Mutex
	editor  core.Editor
	store   core.OperationStore
	plans   map[string]core.ChangePlan
	initErr error
	paths   ConfigurationPaths
}

type ConfigurationPaths struct {
	WT         string
	PowerShell string
	Operations string
}

// Target paths are resolved once by the entry point, never supplied by the UI.
func newConfigurationService() *configurationService {
	s := &configurationService{plans: map[string]core.ChangePlan{}}
	// Explicit isolation for desktop acceptance runs; all three targets move together.
	if root := os.Getenv("SHUTTLE_CONFIG_TEST_ROOT"); root != "" {
		if !filepath.IsAbs(root) {
			s.initErr = errors.New("隔离目录必须为绝对路径")
			return s
		}
		s.paths = ConfigurationPaths{WT: filepath.Join(root, "settings.json"), PowerShell: filepath.Join(root, "profile.ps1"), Operations: filepath.Join(root, "operations")}
		s.editor = &configurationEditor{wt: adapter.NewWTEditor(s.paths.WT), ps: adapter.NewPSEditor(s.paths.PowerShell)}
		s.store = adapter.NewFileOperationStore(s.paths.Operations)
		s.initErr = s.store.RecoverInterrupted()
		return s
	}
	local, err := os.UserCacheDir()
	if err != nil {
		s.initErr = err
		return s
	}
	s.paths.Operations = filepath.Join(local, "Shuttle", "operations")
	s.paths.WT = filepath.Join(local, "Microsoft", "Windows Terminal", "settings.json")
	for _, path := range []string{
		filepath.Join(local, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json"),
		s.paths.WT,
	} {
		if _, err := os.Stat(path); err == nil {
			s.paths.WT = path
			break
		}
	}
	// -NoProfile evaluates only the built-in variable; no user script is loaded.
	shell, err := exec.LookPath("pwsh.exe")
	if err != nil {
		s.initErr = err
		return s
	}
	output, err := exec.Command(shell, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "[Console]::Write($PROFILE.CurrentUserAllHosts)").Output()
	if err != nil {
		s.initErr = err
		return s
	}
	s.paths.PowerShell = strings.TrimSpace(string(output))
	if !filepath.IsAbs(s.paths.PowerShell) {
		s.initErr = errors.New("无法定位 PowerShell Profile")
		return s
	}
	s.editor = &configurationEditor{wt: adapter.NewWTEditor(s.paths.WT), ps: adapter.NewPSEditor(s.paths.PowerShell)}
	s.store = adapter.NewFileOperationStore(s.paths.Operations)
	s.initErr = s.store.RecoverInterrupted()
	return s
}

type configurationEditor struct {
	wt core.Editor
	ps core.Editor
}

func (e *configurationEditor) editor(target string) core.Editor {
	if strings.HasPrefix(target, "ps:") {
		return e.ps
	}
	return e.wt
}
func (e *configurationEditor) ReadCurrent(target string) (string, error) {
	return e.editor(target).ReadCurrent(target)
}
func (e *configurationEditor) Check(target, value string) error {
	return e.editor(target).Check(target, value)
}
func (e *configurationEditor) Apply(target, value string) (string, bool, error) {
	return e.editor(target).Apply(target, value)
}

func (a *App) ConfigurationPaths() (ConfigurationPaths, error) {
	s := a.configuration
	if s == nil {
		return ConfigurationPaths{}, errors.New("配置服务尚未就绪")
	}
	return s.paths, s.initErr
}
func (a *App) PlanConfiguration(choice core.ConfigChoice) (core.ChangePlan, error) {
	s := a.configuration
	if s == nil {
		return core.ChangePlan{}, errors.New("配置服务尚未就绪")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initErr != nil {
		return core.ChangePlan{}, s.initErr
	}
	plan, err := core.Plan(choice, s.editor)
	if err == nil {
		s.remember(plan)
	}
	return plan, err
}
func (s *configurationService) remember(plan core.ChangePlan) {
	// Plans are server-owned and single use; keep only the latest preview.
	s.plans = map[string]core.ChangePlan{plan.PlanID: plan}
}
func (a *App) ExecuteConfiguration(planID string) (core.OperationResult, error) {
	s := a.configuration
	if s == nil {
		return core.OperationResult{}, errors.New("配置服务尚未就绪")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initErr != nil {
		return core.OperationResult{}, s.initErr
	}
	plan, ok := s.plans[planID]
	if !ok {
		return core.OperationResult{}, errors.New("计划已过期，请重新预览")
	}
	delete(s.plans, planID)
	a.configurationBusy.Store(true)
	defer a.configurationBusy.Store(false)
	return core.ExecuteRecorded(plan, s.editor, s.store)
}
func (a *App) ListConfigurationOperations() ([]core.Operation, error) {
	s := a.configuration
	if s == nil {
		return nil, errors.New("配置服务尚未就绪")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initErr != nil {
		return nil, s.initErr
	}
	return s.store.List()
}
func (a *App) PlanConfigurationRestore(id string) (core.ChangePlan, error) {
	s := a.configuration
	if s == nil {
		return core.ChangePlan{}, errors.New("配置服务尚未就绪")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initErr != nil {
		return core.ChangePlan{}, s.initErr
	}
	plan, err := core.PlanRecordedRestore(id, s.editor, s.store)
	if err == nil {
		s.remember(plan)
	}
	return plan, err
}

// Package inventory implements read-only ownership detection from
// docs/inventory-contract.md. Pure core: collection and ledger storage are
// injected, so classification stays a testable function of its inputs.
package inventory

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Source names where a package was observed. New values are contract changes
// (docs/inventory-contract.md §9 D2), never a silent extension.
type Source string

const (
	SourceNPM    Source = "npm"
	SourceWinget Source = "winget"
	SourcePath   Source = "path"
)

// Ownership is a closed set: there is no "suspected" value by design.
type Ownership string

const (
	OwnedByShuttle   Ownership = "managed"
	OwnedExternally  Ownership = "external"
	OwnershipUnknown Ownership = "unknown"
)

// Package is one observation from one source. Version and Root may be empty
// when the source cannot report them.
type Package struct {
	Name    string
	Version string
	Source  Source
	Root    string
}

// RecordState is the lifecycle state of a ledger record (contract §10.3). Only
// StateInstalled can ever produce a managed verdict.
type RecordState string

const (
	StateIntent     RecordState = "intent"     // written before the installer runs
	StateInstalled  RecordState = "installed"  // installer succeeded and the recheck found it
	StateUnverified RecordState = "unverified" // installer succeeded but the recheck did not
	StateFailed     RecordState = "failed"     // installer failed and the package is gone
)

// Record is what the ledger remembers about something Shuttle installed
// (contract §10.2). Version is informative only; ownership uses Name+Source+Root.
type Record struct {
	Name           string      `json:"Name"`
	Source         Source      `json:"Source"`
	Root           string      `json:"Root"`
	Version        string      `json:"Version,omitempty"`
	State          RecordState `json:"State,omitempty"`
	OperationID    string      `json:"OperationID,omitempty"`
	EvidenceBefore string      `json:"EvidenceBefore,omitempty"`
	EvidenceAfter  string      `json:"EvidenceAfter,omitempty"`
	InstallerExit  int         `json:"InstallerExit,omitempty"`
	InstallerError string      `json:"InstallerError,omitempty"`
	InstalledAt    string      `json:"InstalledAt,omitempty"`
}

// Ledger is the install ledger (contract §9 D1). Lookup reports ok=false when
// there is no record; a broken ledger must surface as an error, not as absence.
type Ledger interface {
	Lookup(name string) (Record, bool, error)
	// Put writes one state transition. Implementations must refuse to write
	// when the existing ledger cannot be read (contract §10.5).
	Put(record Record) error
	// List feeds startup reconciliation of interrupted installs (§10.7).
	List() ([]Record, error)
	// Delete removes a record after successful uninstallation.
	Delete(name string) error
}

// Item is a package plus its verdict. Reason is always shown to the user.
type Item struct {
	Package
	Ownership Ownership
	Reason    string
}

// ErrUnavailable marks a source whose collection failed. Callers must show it
// rather than treating the source as empty.
var ErrUnavailable = errors.New("inventory source unavailable")

// Classify assigns ownership. Anything not provably installed by Shuttle is
// external; a failed ledger lookup makes the item unknown rather than external,
// because the difference tells the user their ledger is broken.
func Classify(pkgs []Package, ledger Ledger) []Item {
	items := make([]Item, 0, len(pkgs))
	for _, pkg := range pkgs {
		item := Item{Package: pkg}
		switch pkg.Source {
		case SourcePath:
			// A PATH hit proves the tool runs, not who installed it (§9 D3).
			item.Ownership, item.Reason = OwnedExternally, "仅能在 PATH 上找到，无法证明来源，不进入卸载清单"
		case SourceNPM, SourceWinget:
			item.Ownership, item.Reason = classifyKnown(pkg, ledger)
		default:
			item.Ownership, item.Reason = OwnershipUnknown, fmt.Sprintf("来源 %q 不在已支持列表", pkg.Source)
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Source < items[j].Source
	})
	return items
}

func classifyKnown(pkg Package, ledger Ledger) (Ownership, string) {
	if ledger == nil {
		return OwnedExternally, "无安装台账，不归 Shuttle 所有"
	}
	record, ok, err := ledger.Lookup(pkg.Name)
	if err != nil {
		return OwnershipUnknown, "安装台账不可用：" + err.Error()
	}
	if !ok {
		return OwnedExternally, "安装台账无此记录，视为用户自行安装"
	}
	if record.Source != pkg.Source {
		return OwnershipUnknown, fmt.Sprintf("台账记录来源为 %s，实际检测为 %s，需人工核对", record.Source, pkg.Source)
	}
	if record.Root != "" && pkg.Root != "" && !sameRoot(record.Root, pkg.Root) {
		return OwnershipUnknown, fmt.Sprintf("台账记录位置 %s 与实际 %s 不一致，需人工核对", record.Root, pkg.Root)
	}
	// A matching name+source+root is not enough: the record must also carry
	// installation evidence (contract §10.3). An empty state is a v1 record,
	// which only existed after a successful write, so it counts as installed.
	switch record.State {
	case StateInstalled, "":
		return OwnedByShuttle, "安装台账记录与当前检测一致"
	case StateUnverified:
		return OwnershipUnknown, "缺少完整的安装成功证据，需人工核对"
	case StateIntent:
		return OwnershipUnknown, "台账只有安装意图，没有成功证据（可能安装中中断），需人工核对"
	case StateFailed:
		return OwnedExternally, "台账记录该包由 Shuttle 安装失败，当前存在视为用户自行安装"
	default:
		return OwnershipUnknown, fmt.Sprintf("台账记录状态 %q 无法识别，需人工核对", record.State)
	}
}

func sameRoot(a, b string) bool {
	normalize := func(v string) string {
		return strings.ToLower(strings.TrimRight(strings.ReplaceAll(v, "/", `\`), `\`))
	}
	return normalize(a) == normalize(b)
}

// Choice selects what the user wants to uninstall.
type Choice struct {
	Names []string
}

// Step is one planned action. Action is "install", "uninstall" or "skip".
// Item carries the detected package and the reason shown to the user; Package
// is set for install steps that were not detected at all.
type Step struct {
	Item        `json:"Item"`
	Package     Package `json:",omitempty"`
	Action      string
	Command     string   `json:",omitempty"` // the exact command the UI shows (D8)
	Args        []string `json:"-"`          // argv for execution; never sent to the UI
	Instruction string   `json:",omitempty"` // official manual instructions
}

// Plan is read-only: nothing here executes an uninstall (contract §1).
type Plan struct {
	Steps     []Step
	Blocked   bool
	Conflicts []string
}

func buildUninstallCommand(item Item) (string, []string) {
	switch item.Source {
	case SourceNPM:
		return "npm uninstall -g " + item.Name, []string{"npm", "uninstall", "-g", item.Name}
	case SourceWinget:
		id := item.Name
		if item.Root != "" && item.Root != "winget" {
			id = item.Root
		}
		return fmt.Sprintf("winget uninstall --id %s --exact", id), []string{"winget", "uninstall", "--id", id, "--exact"}
	default:
		return "", nil
	}
}

// PlanUninstall builds the uninstall plan. Unknown items block; external items
// are skipped with the reason preserved so the UI can explain why.
func PlanUninstall(items []Item, choice Choice) (Plan, error) {
	if len(choice.Names) == 0 {
		return Plan{}, &Error{Kind: "validation_failed", Reason: "未选择任何组件"}
	}
	wanted := map[string]bool{}
	for _, name := range choice.Names {
		wanted[strings.TrimSpace(name)] = true
	}
	plan := Plan{}
	for _, item := range items {
		if !wanted[item.Name] {
			continue
		}
		delete(wanted, item.Name)
		step := Step{Item: item, Action: ActionSkip}
		switch item.Ownership {
		case OwnedByShuttle:
			step.Action = ActionUninstall
			cmd, args := buildUninstallCommand(item)
			step.Command = cmd
			step.Args = args
		case OwnershipUnknown:
			plan.Blocked = true
			plan.Conflicts = append(plan.Conflicts, fmt.Sprintf("%s：%s", item.Name, item.Reason))
		}
		plan.Steps = append(plan.Steps, step)
	}
	for name := range wanted {
		plan.Blocked = true
		plan.Conflicts = append(plan.Conflicts, fmt.Sprintf("%s：未在检测结果中找到", name))
	}
	sort.Strings(plan.Conflicts)
	return plan, nil
}

// Error mirrors core/configuration's error shape so the bridge maps both alike.
type Error struct {
	Kind   string
	Reason string
}

func (e *Error) Error() string { return e.Kind + ": " + e.Reason }

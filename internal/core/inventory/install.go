package inventory

import (
	"fmt"
	"sort"
	"strings"
)

// Action values shared by both plans.
const (
	ActionInstall   = "install"
	ActionUninstall = "uninstall"
	ActionSkip      = "skip"
)

// PlanInstall is read-only (contract §10.6, D8): it says what would be
// installed, with the full command the UI must show verbatim. Nothing here
// runs anything.
func PlanInstall(wanted []Package, detected []Item) (Plan, error) {
	if len(wanted) == 0 {
		return Plan{}, &Error{Kind: "validation_failed", Reason: "未选择任何组件"}
	}
	observed := make(map[string]Item, len(detected))
	for _, item := range detected {
		observed[item.Name+"\x00"+string(item.Source)] = item
	}
	plan := Plan{}
	seen := map[string]bool{}
	for _, pkg := range wanted {
		key := pkg.Name + "\x00" + string(pkg.Source)
		if seen[key] {
			continue
		}
		seen[key] = true

		args, command, instruction, err := installRecipe(pkg)
		if err != nil {
			plan.Blocked = true
			plan.Conflicts = append(plan.Conflicts, fmt.Sprintf("%s：%v", pkg.Name, err))
			continue
		}
		step := Step{Package: pkg, Action: ActionInstall, Command: command, Instruction: instruction, Args: args}
		step.Item = Item{Package: pkg, Ownership: OwnedExternally, Reason: "当前未检测到，可以安装"}
		if item, ok := observed[key]; ok {
			step.Item = item
			switch item.Ownership {
			case OwnedByShuttle:
				step.Action, step.Reason = ActionSkip, "已由 Shuttle 安装，不重复安装"
			case OwnedExternally:
				step.Action, step.Reason = ActionSkip, "已存在（用户自行安装），不重复安装；如需接管请先自行卸载"
			default:
				// Never install over an ambiguous owner: the install would
				// create a second copy whose ownership still cannot be proven.
				step.Action = ActionSkip
				plan.Blocked = true
				plan.Conflicts = append(plan.Conflicts, fmt.Sprintf("%s：%s", pkg.Name, item.Reason))
			}
		}
		plan.Steps = append(plan.Steps, step)
	}
	sort.Strings(plan.Conflicts)
	return plan, nil
}

// installRecipe is the only place that knows how to install a source (D7).
// Args is what gets executed; Command is the same thing rendered for the user.
// They are kept separate because execution must never go through a shell: a
// scoped npm name contains characters a shell would reinterpret.
func installRecipe(pkg Package) (args []string, command, instruction string, err error) {
	name := strings.TrimSpace(pkg.Name)
	if name == "" {
		return nil, "", "", &Error{Kind: "validation_failed", Reason: "缺少包名"}
	}
	switch pkg.Source {
	case SourceNPM:
		return []string{"npm", "install", "-g", name},
			"npm install -g " + name,
			"官方说明：https://www.npmjs.com/package/" + name, nil
	case SourceWinget:
		args = []string{"winget", "install", "--id", name, "--exact",
			"--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}
		return args, strings.Join(args, " "),
			"官方说明：https://learn.microsoft.com/windows/package-manager/winget/install", nil
	default:
		return nil, "", "", &Error{Kind: "unsupported_source", Reason: fmt.Sprintf("来源 %q 不支持自动安装", pkg.Source)}
	}
}

// VerifyInstalled produces the terminal ledger record after an install
// (contract §10.3, D6): name, source and root must all match what was
// detected, otherwise the record is unverified, never installed.
func VerifyInstalled(intent Record, detected []Package) Record {
	record := intent
	record.State = StateUnverified
	record.EvidenceAfter = "安装后未检测到该包"
	for _, pkg := range detected {
		if pkg.Name != intent.Name || pkg.Source != intent.Source {
			continue
		}
		if intent.Root != "" && pkg.Root != "" && !sameRoot(intent.Root, pkg.Root) {
			record.EvidenceAfter = fmt.Sprintf("检测到的位置 %s 与记录 %s 不一致", pkg.Root, intent.Root)
			continue
		}
		record.State = StateInstalled
		record.Version = pkg.Version
		record.EvidenceAfter = evidence(pkg)
		break
	}
	return record
}

func evidence(pkg Package) string {
	parts := []string{pkg.Name}
	if pkg.Version != "" {
		parts = append(parts, pkg.Version)
	}
	parts = append(parts, string(pkg.Source))
	if pkg.Root != "" {
		parts = append(parts, pkg.Root)
	}
	return strings.Join(parts, " · ")
}

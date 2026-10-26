package inventory

import (
	"strings"
	"testing"
)

// Contract §10.8 test 17: every install step carries the command the UI must
// show verbatim plus a manual fallback (D8).
func TestPlanInstallCarriesCommandAndInstructions(t *testing.T) {
	plan, err := PlanInstall([]Package{
		{Name: "@anthropic-ai/claude-code", Source: SourceNPM},
		{Name: "Anthropic.Claude", Source: SourceWinget},
	}, nil)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.Blocked || len(plan.Steps) != 2 {
		t.Fatalf("both installable: %+v", plan)
	}
	for _, step := range plan.Steps {
		if step.Action != ActionInstall {
			t.Fatalf("expected install step: %+v", step)
		}
		if step.Command == "" || step.Instruction == "" || step.Reason == "" {
			t.Fatalf("step must explain itself and show its command: %+v", step)
		}
		switch step.Source {
		case SourceNPM:
			if !strings.HasPrefix(step.Command, "npm install -g ") {
				t.Fatalf("npm command: %q", step.Command)
			}
			if !strings.Contains(step.Instruction, "npmjs.com/package/@anthropic-ai/claude-code") {
				t.Fatalf("npm instruction: %q", step.Instruction)
			}
		case SourceWinget:
			// --exact keeps a partial id from installing something else
			if !strings.Contains(step.Command, "winget install --id Anthropic.Claude --exact") {
				t.Fatalf("winget command: %q", step.Command)
			}
			if !strings.Contains(step.Instruction, "learn.microsoft.com") {
				t.Fatalf("winget instruction: %q", step.Instruction)
			}
		default:
			t.Fatalf("unexpected source: %+v", step)
		}
	}
}

// Contract §10.8 test 15: planning never installs, and it refuses to plan over
// anything whose ownership is not settled.
func TestPlanInstallNeverReinstalls(t *testing.T) {
	detected := []Item{
		{Package: Package{Name: "@anthropic-ai/claude-code", Source: SourceNPM}, Ownership: OwnedByShuttle, Reason: "台账一致"},
		{Package: Package{Name: "@openai/codex", Source: SourceNPM}, Ownership: OwnedExternally, Reason: "台账无记录"},
		{Package: Package{Name: "Odd.Pkg", Source: SourceWinget}, Ownership: OwnershipUnknown, Reason: "来源不一致"},
	}
	plan, err := PlanInstall([]Package{
		{Name: "@anthropic-ai/claude-code", Source: SourceNPM},
		{Name: "@openai/codex", Source: SourceNPM},
		{Name: "Odd.Pkg", Source: SourceWinget},
		{Name: "Fresh.Pkg", Source: SourceWinget},
	}, detected)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	actions := map[string]string{}
	for _, step := range plan.Steps {
		actions[step.Name] = step.Action
	}
	if actions["Fresh.Pkg"] != ActionInstall {
		t.Fatalf("absent package must be installable: %+v", actions)
	}
	for _, name := range []string{"@anthropic-ai/claude-code", "@openai/codex", "Odd.Pkg"} {
		if actions[name] != ActionSkip {
			t.Fatalf("%s must not be reinstalled: %+v", name, actions)
		}
	}
	// only the ambiguous one blocks: the other two are settled decisions
	if !plan.Blocked || len(plan.Conflicts) != 1 || !strings.Contains(plan.Conflicts[0], "Odd.Pkg") {
		t.Fatalf("only unknown ownership blocks: %+v", plan)
	}
}

func TestPlanInstallRejectsUnsupportedSourceAndEmptySelection(t *testing.T) {
	if _, err := PlanInstall(nil, nil); err == nil {
		t.Fatal("empty selection must be rejected")
	}
	plan, err := PlanInstall([]Package{{Name: "tool", Source: Source("scoop")}}, nil)
	if err != nil {
		t.Fatalf("unsupported source should be a conflict, not an error: %v", err)
	}
	if !plan.Blocked || len(plan.Conflicts) != 1 || len(plan.Steps) != 0 {
		t.Fatalf("unsupported source must block: %+v", plan)
	}
}

// Same name from two sources is two different things: one being present must
// not stop the other from being installed.
func TestPlanInstallMatchesBySourceToo(t *testing.T) {
	plan, err := PlanInstall([]Package{{Name: "tool", Source: SourceWinget}},
		[]Item{{Package: Package{Name: "tool", Source: SourceNPM}, Ownership: OwnedByShuttle, Reason: "npm 已装"}})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Action != ActionInstall {
		t.Fatalf("npm record must not block the winget install: %+v", plan)
	}
}

// Contract §10.8 test 18: the recheck is strict. Name, source and root must
// agree; a version change alone keeps it installed (D6).
func TestVerifyInstalledRequiresMatchingEvidence(t *testing.T) {
	intent := Record{Name: "@openai/codex", Source: SourceNPM, Root: `C:\npm`, State: StateIntent,
		OperationID: "op-1", EvidenceBefore: "absent"}

	cases := []struct {
		name     string
		detected []Package
		want     RecordState
		reason   string
	}{
		{"match", []Package{{Name: "@openai/codex", Version: "0.165.0", Source: SourceNPM, Root: `C:\NPM\`}},
			StateInstalled, ""},
		{"absent", nil, StateUnverified, ""},
		{"other source", []Package{{Name: "@openai/codex", Source: SourceWinget, Root: `C:\npm`}},
			StateUnverified, ""},
		{"other root", []Package{{Name: "@openai/codex", Source: SourceNPM, Root: `D:\other`}},
			StateUnverified, ""},
	}
	for _, c := range cases {
		got := VerifyInstalled(intent, c.detected)
		if got.State != c.want {
			t.Fatalf("%s: state %s want %s", c.name, got.State, c.want)
		}
		if got.Name != intent.Name || got.Source != intent.Source || got.OperationID != "op-1" || got.EvidenceBefore != "absent" {
			t.Fatalf("%s: intent fields must survive: %+v", c.name, got)
		}
		if got.EvidenceAfter == "" {
			t.Fatalf("%s: evidence must be recorded", c.name)
		}
	}
	// the version comes from the recheck, not from the plan
	got := VerifyInstalled(intent, []Package{{Name: "@openai/codex", Version: "0.165.0", Source: SourceNPM, Root: `C:\npm`}})
	if got.Version != "0.165.0" {
		t.Fatalf("version must come from the detection: %+v", got)
	}
	if got.State != StateInstalled {
		t.Fatalf("matched record must be installed: %+v", got)
	}
}

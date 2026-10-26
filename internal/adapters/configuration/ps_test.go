package configuration

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "shuttle/internal/core/configuration"
)

func TestProfileFixtureIdempotencyAndRemoval(t *testing.T) {
	path := copyFixture(t, "ps-clean.ps1")
	before := readFile(t, path)
	ed := NewPSEditor(path)
	choice := core.ConfigChoice{SchemaVersion: 1, PS: core.PSCheck{SessionEntry: true}}
	plan, err := core.Plan(choice, ed)
	if err != nil || plan.Blocked || len(plan.Diffs) != 1 || plan.Diffs[0].After != core.ProfileBlock {
		t.Fatalf("Plan: %+v, %v", plan, err)
	}
	if old, changed, err := ed.Apply(psTarget, plan.Diffs[0].After); err != nil || !changed || old != "" {
		t.Fatalf("first Apply: %q, %v, %v", old, changed, err)
	}
	snapshot := readFile(t, path)
	if !strings.HasPrefix(snapshot, before) {
		t.Fatal("unmanaged prefix changed")
	}
	plan, err = core.Plan(choice, ed)
	if err != nil || plan.Blocked || plan.Diffs[0].Action != "none" {
		t.Fatalf("second Plan: %+v, %v", plan, err)
	}
	if _, changed, err := ed.Apply(psTarget, core.ProfileBlock); err != nil || changed || readFile(t, path) != snapshot {
		t.Fatalf("second Apply: %v, %v", changed, err)
	}
	if _, changed, err := ed.Apply(psTarget, ""); err != nil || !changed {
		t.Fatalf("remove: %v, %v", changed, err)
	}
	if readFile(t, path) != before {
		t.Fatal("removal changed unmanaged bytes")
	}
	if _, err := ed.ReadCurrent(psTarget); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("removed block: %v", err)
	}
}

func TestProfilePromptConflictsRefuseWrites(t *testing.T) {
	for _, source := range []string{"fixture", "starship init powershell | Invoke-Expression\n"} {
		t.Run(source, func(t *testing.T) {
			path := copyFixture(t, "ps-conflict.ps1")
			if source != "fixture" {
				if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before := readFile(t, path)
			ed := NewPSEditor(path)
			plan, err := core.Plan(core.ConfigChoice{SchemaVersion: 1, PS: core.PSCheck{SessionEntry: true}}, ed)
			if err != nil || !plan.Blocked || plan.Diffs[0].Conflict == "" {
				t.Fatalf("conflict Plan: %+v, %v", plan, err)
			}
			if _, _, err := ed.Apply(psTarget, core.ProfileBlock); err == nil {
				t.Fatal("direct Apply bypassed conflict")
			}
			if readFile(t, path) != before {
				t.Fatal("conflict changed file")
			}
		})
	}
}

func TestProfileReplacementPreservesBOMNewlinesAndSuffix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.ps1")
	oldBlock := core.ProfileBlockStart + "\nWrite-Output '旧内容'\n" + core.ProfileBlockEnd
	prefix := "# 自定义配置\r\n"
	suffix := "\r\nWrite-Output '后置内容'\r\n"
	if err := os.WriteFile(path, []byte(prefix+strings.ReplaceAll(oldBlock, "\n", "\r\n")+suffix), 0o600); err != nil {
		t.Fatal(err)
	}
	ed := NewPSEditor(path)
	if old, changed, err := ed.Apply(psTarget, core.ProfileBlock); err != nil || !changed || old != oldBlock {
		t.Fatalf("replace: %q, %v, %v", old, changed, err)
	}
	want := prefix + strings.ReplaceAll(core.ProfileBlock, "\n", "\r\n") + suffix
	if readFile(t, path) != want {
		t.Fatal("replacement changed encoding/newlines/outside bytes")
	}
	if _, _, err := ed.Apply(psTarget, oldBlock); err != nil {
		t.Fatal(err)
	}
	if got, err := ed.ReadCurrent(psTarget); err != nil || got != oldBlock {
		t.Fatalf("restored: %q, %v", got, err)
	}
}

func TestProfileMalformedAndUnsupportedFilesStayUntouched(t *testing.T) {
	for _, text := range []string{
		core.ProfileBlockStart,
		core.ProfileBlockEnd,
		core.ProfileBlockEnd + "\n" + core.ProfileBlockStart,
		core.ProfileBlock + "\n" + core.ProfileBlock,
		string([]byte{0xff, 0xfe, 0x23, 0x00}),
	} {
		path := filepath.Join(t.TempDir(), "profile.ps1")
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		ed := NewPSEditor(path)
		if _, _, err := ed.Apply(psTarget, core.ProfileBlock); err == nil {
			t.Fatalf("accepted %q", text)
		}
		if readFile(t, path) != text {
			t.Fatal("invalid file changed")
		}
	}
}

func TestProfileMissingFileAndCommentedInit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "profile.ps1")
	ed := NewPSEditor(path)
	if _, err := ed.ReadCurrent(psTarget); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, changed, err := ed.Apply(psTarget, ""); err != nil || changed {
		t.Fatalf("absent removal: %v, %v", changed, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("removal created file: %v", err)
	}
	if _, changed, err := ed.Apply(psTarget, core.ProfileBlock); err != nil || !changed {
		t.Fatalf("create: %v, %v", changed, err)
	}
	if err := os.WriteFile(path, []byte("# starship init powershell\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ed.Check(psTarget, core.ProfileBlock); err != nil {
		t.Fatalf("comment conflict: %v", err)
	}
	if err := ed.Check("wt:scalar:theme", core.ProfileBlock); err == nil {
		t.Fatal("accepted unrelated target")
	}
}

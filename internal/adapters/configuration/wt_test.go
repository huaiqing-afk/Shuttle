package configuration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "shuttle/internal/core/configuration"
)

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "fixtures", "config", name))
	if err != nil {
		t.Fatalf("读取 fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "settings.jsonc")
	if err := os.WriteFile(path, source, 0o644); err != nil {
		t.Fatalf("写入临时文件: %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s: %v", path, err)
	}
	return string(data)
}

// contract §7.1: only the target span changes; every other byte is preserved.
func TestScalarEditPreservesEveryOtherByte(t *testing.T) {
	path := copyFixture(t, "wt-sample.jsonc")
	before := readFile(t, path)
	ed := NewWTEditor(path)

	if _, _, err := ed.Apply("wt:scalar:defaultProfile", `"{574e775e-4f2a-5b96-ac1e-a2962a402336}"`); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	after := readFile(t, path)
	if after == before {
		t.Fatal("value was not written")
	}
	// replace the new value with the old one; the rest must match byte for byte
	restored := strings.Replace(after, `"{574e775e-4f2a-5b96-ac1e-a2962a402336}"`, `"{61c54bbd-c2c6-5271-96e7-009a87ff44bf}"`, 1)
	if restored != before {
		t.Fatalf("non-target bytes changed:\n--- want ---\n%s\n--- got ---\n%s", before, restored)
	}
}

// contract §7.2: comments and trailing commas survive the write.
func TestCommentsAndTrailingCommasSurvive(t *testing.T) {
	path := copyFixture(t, "wt-sample.jsonc")
	original := readFile(t, path)
	ed := NewWTEditor(path)
	if _, _, err := ed.Apply("wt:scalar:profiles.defaults.fontFace", `"Maple Mono NF CN"`); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	after := readFile(t, path)
	if !strings.Contains(after, "// Comments and trailing commas must survive") {
		t.Fatal("header comment lost")
	}
	if !strings.Contains(after, "// an unrelated user profile we must never touch") {
		t.Fatal("profile comment lost")
	}
	if !strings.Contains(after, "// user's own theme, unrelated") {
		t.Fatal("theme comment lost")
	}
	if !strings.Contains(after, `"fontSize": 12,`) {
		t.Fatal("trailing comma lost")
	}
	if strings.Contains(original, "Maple Mono") || !strings.Contains(after, `"fontFace": "Maple Mono NF CN"`) {
		t.Fatal("fontFace not written")
	}
}

// contract §7.3: applying the same choice twice is a no-op.
func TestSecondApplyIsIdempotent(t *testing.T) {
	path := copyFixture(t, "wt-sample.jsonc")
	ed := NewWTEditor(path)
	const profile = `{"guid":"{aaaaaaaa-1111-2222-3333-444444444444}","name":"Shuttle","hidden":false}`

	if _, changed, err := ed.Apply("wt:profile:{aaaaaaaa-1111-2222-3333-444444444444}", profile); err != nil || !changed {
		t.Fatalf("first Apply: changed=%v err=%v", changed, err)
	}
	snapshot := readFile(t, path)
	if _, changed, err := ed.Apply("wt:profile:{aaaaaaaa-1111-2222-3333-444444444444}", profile); err != nil || changed {
		t.Fatalf("second Apply must be a no-op: changed=%v err=%v", changed, err)
	}
	if readFile(t, path) != snapshot {
		t.Fatal("second apply rewrote the file")
	}
	if n := strings.Count(readFile(t, path), "{aaaaaaaa-1111-2222-3333-444444444444}"); n != 1 {
		t.Fatalf("managed profile appears %d times", n)
	}
}

// contract §7.4: same name with a different GUID is a conflict, never an overwrite.
func TestSameNameDifferentGUIDConflicts(t *testing.T) {
	path := copyFixture(t, "wt-sample.jsonc")
	ed := NewWTEditor(path)
	desired := `{"guid":"{aaaaaaaa-1111-2222-3333-444444444444}","name":"用户自建 Profile"}`
	err := ed.Check("wt:profile:{aaaaaaaa-1111-2222-3333-444444444444}", desired)
	var cfgErr *core.Error
	if !asError(err, &cfgErr) || cfgErr.Kind != "conflict" {
		t.Fatalf("expected conflict, got %v", err)
	}
	// a different name is not a conflict
	if err := ed.Check("wt:profile:{aaaaaaaa-1111-2222-3333-444444444444}", `{"name":"Shuttle"}`); err != nil {
		t.Fatalf("unexpected conflict: %v", err)
	}
}

func TestReadAndRemoveManagedProfile(t *testing.T) {
	path := copyFixture(t, "wt-sample.jsonc")
	ed := NewWTEditor(path)
	guid := "{aaaaaaaa-1111-2222-3333-444444444444}"
	profile := `{"guid":"` + guid + `","name":"Shuttle","hidden":false}`
	if _, _, err := ed.Apply("wt:profile:"+guid, profile); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	value, err := ed.ReadCurrent("wt:profile:" + guid)
	if err != nil || !strings.Contains(value, "Shuttle") {
		t.Fatalf("ReadCurrent = %q, %v", value, err)
	}
	if _, changed, err := ed.Apply("wt:profile:"+guid, ""); err != nil || !changed {
		t.Fatalf("remove: changed=%v err=%v", changed, err)
	}
	if _, err := ed.ReadCurrent("wt:profile:" + guid); err != core.ErrNotFound {
		t.Fatalf("expected ErrNotFound after removal, got %v", err)
	}
	// the user's own profile is untouched
	if !strings.Contains(readFile(t, path), "用户自建 Profile") {
		t.Fatal("unrelated profile removed")
	}
}

func TestMissingScalarIsNotFound(t *testing.T) {
	path := copyFixture(t, "wt-sample.jsonc")
	ed := NewWTEditor(path)
	if _, err := ed.ReadCurrent("wt:scalar:profiles.defaults.colorScheme"); err != core.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestApplyToMissingFileCreates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "settings.jsonc")
	ed := NewWTEditor(path)
	if _, _, err := ed.Apply("wt:scalar:theme", `"dark"`); err != nil {
		t.Fatalf("Apply to missing file: %v", err)
	}
	if !strings.Contains(readFile(t, path), `"theme": "dark"`) {
		t.Fatalf("theme not created: %s", readFile(t, path))
	}
}

func asError(err error, target **core.Error) bool {
	if err == nil {
		return false
	}
	cfgErr, ok := err.(*core.Error)
	if ok {
		*target = cfgErr
	}
	return ok
}

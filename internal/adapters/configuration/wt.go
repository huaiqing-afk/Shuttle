// Package configuration adapts Shuttle's config plan to real files: WT
// settings.json (JSONC surgical edits) and the PowerShell profile managed
// block. All writes are byte-surgical so comments and trailing commas survive
// (docs/config-contract.md §4).
package configuration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	core "shuttle/internal/core/configuration"
)

// Values passed to Apply/Check/ReadCurrent are always raw JSON literals
// (`"dark"`, `{"guid":...}`), never bare strings: the editor never guesses
// whether a value should be quoted.

// WTEditor edits Windows Terminal settings.json in place. It never
// deserializes the whole document: a JSONC scanner locates value spans and
// only those bytes are replaced.
type WTEditor struct {
	Path string
}

func NewWTEditor(path string) *WTEditor { return &WTEditor{Path: path} }

var _ core.Editor = (*WTEditor)(nil)

func (e *WTEditor) ReadCurrent(target string) (string, error) {
	raw, err := os.ReadFile(e.Path)
	if err != nil {
		return "", err
	}
	spec, err := parseTarget(target)
	if err != nil {
		return "", err
	}
	value, found, err := findValue(raw, spec)
	if err != nil {
		return "", err
	}
	if !found {
		return "", core.ErrNotFound
	}
	return compact(value.value), nil
}

func (e *WTEditor) Check(target string, desired string) error {
	raw, err := os.ReadFile(e.Path)
	if err != nil {
		return nil // a missing file has no conflicts
	}
	return checkConflict(raw, target, desired)
}

func (e *WTEditor) Apply(target string, value string) (string, bool, error) {
	spec, err := parseTarget(target)
	if err != nil {
		return "", false, err
	}
	if value != "" {
		if !json.Valid([]byte(value)) {
			return "", false, &core.Error{Kind: "validation_failed", Target: target, Reason: "值不是合法 JSON 字面量"}
		}
	}
	raw, err := os.ReadFile(e.Path)
	if err != nil {
		if !os.IsNotExist(err) || value == "" {
			return "", false, err
		}
		raw = []byte("{\n}\n")
	}
	current, found, err := findValue(raw, spec)
	if err != nil {
		return "", false, err
	}
	var updated []byte
	switch {
	case value == "" && !found:
		return "", false, nil
	case value == "":
		updated, err = removeValue(raw, current)
	case found:
		updated = splice(raw, current, value)
	default:
		updated, err = insertValue(raw, spec, value)
	}
	if err != nil {
		return "", false, err
	}
	before := ""
	if found {
		before = compact(current.value)
	}
	if string(updated) == string(raw) {
		return before, false, nil
	}
	return before, true, writeAtomic(e.Path, updated)
}

func checkConflict(raw []byte, target string, desired string) error {
	spec, err := parseTarget(target)
	if err != nil || spec.kind != "profile" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(desired), &payload); err != nil {
		return &core.Error{Kind: "conflict", Target: target, Reason: "受管 Profile 不是合法 JSON 对象"}
	}
	name, _ := payload["name"].(string)
	if name == "" {
		return nil
	}
	existing, found, err := findArrayItem(raw, spec.path, func(fields map[string]string) bool {
		return unquoteToken(fields["name"]) == name
	})
	if err != nil || !found {
		return nil
	}
	fields, err := objectFields([]byte(existing.value))
	if err != nil {
		return nil
	}
	if guid := unquoteToken(fields["guid"]); guid != spec.guid {
		return &core.Error{Kind: "conflict", Target: target,
			Reason: fmt.Sprintf("同名 Profile %q 已存在且 GUID 不同（%s）", name, guid)}
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".shuttle-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// compact normalizes whitespace so reformatted documents compare equal.
func compact(s string) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err != nil {
		return strings.TrimSpace(s)
	}
	return buf.String()
}

func unquoteToken(token string) string {
	if len(token) >= 2 && token[0] == '"' {
		var value string
		if err := json.Unmarshal([]byte(token), &value); err == nil {
			return value
		}
	}
	return token
}

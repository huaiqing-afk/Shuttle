package configuration

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	core "shuttle/internal/core/configuration"
)

const psTarget = "ps:managed-block"

// PSEditor only replaces the marked block; Path is supplied by the caller.
type PSEditor struct{ Path string }

func NewPSEditor(path string) *PSEditor { return &PSEditor{Path: path} }

var _ core.Editor = (*PSEditor)(nil)

func (e *PSEditor) read(target string) ([]byte, int, int, error) {
	if target != psTarget {
		return nil, 0, 0, &core.Error{Kind: "unsupported", Target: target, Reason: "未知 Profile 目标"}
	}
	raw, err := os.ReadFile(e.Path)
	if os.IsNotExist(err) {
		return nil, -1, -1, nil
	}
	if err != nil {
		return nil, 0, 0, err
	}
	if !utf8.Valid(raw) || bytes.ContainsRune(raw, 0) {
		return nil, 0, 0, &core.Error{Kind: "unsupported", Target: target, Reason: "Profile 必须为 UTF-8；未改写原文件"}
	}
	start, end, err := profileBounds(string(raw))
	return raw, start, end, err
}

func profileBounds(text string) (int, int, error) {
	start, end := -1, -1
	pos := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		body := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if pos == 0 {
			body = strings.TrimPrefix(body, "")
		}
		switch body {
		case core.ProfileBlockStart:
			if start >= 0 || end >= 0 {
				return 0, 0, badMarkers()
			}
			start = pos
			if strings.HasPrefix(text[pos:], "") {
				start += len("")
			}
		case core.ProfileBlockEnd:
			if start < 0 || end >= 0 {
				return 0, 0, badMarkers()
			}
			end = pos + len(body)
		}
		pos += len(line)
	}
	if (start < 0) != (end < 0) {
		return 0, 0, badMarkers()
	}
	return start, end, nil
}

func badMarkers() error {
	return &core.Error{Kind: "conflict", Target: psTarget, Reason: "Profile 管理标记重复、残缺或顺序错误"}
}

func profileValue(raw []byte, start, end int) string {
	if start < 0 {
		return ""
	}
	return strings.ReplaceAll(string(raw[start:end]), "\r\n", "\n")
}

func (e *PSEditor) ReadCurrent(target string) (string, error) {
	raw, start, end, err := e.read(target)
	if err != nil {
		return "", err
	}
	if start < 0 {
		return "", core.ErrNotFound
	}
	return profileValue(raw, start, end), nil
}

var promptInit = regexp.MustCompile(`(?im)^[^#\r\n]*(?:\boh-my-posh|\bstarship)\s+init\b`)

func checkProfile(raw []byte, start, end int, desired string) error {
	if desired == "" {
		return nil
	}
	first, last, err := profileBounds(desired)
	if err != nil || first != 0 || last != len(desired) {
		return &core.Error{Kind: "validation_failed", Target: psTarget, Reason: "期望值必须是完整的单个管理块"}
	}
	outside := string(raw)
	if start >= 0 {
		outside = string(raw[:start]) + string(raw[end:])
	}
	if promptInit.MatchString(outside) {
		return &core.Error{Kind: "conflict", Target: psTarget, Reason: "块外存在 Starship 或 oh-my-posh 初始化，保留原 Profile"}
	}
	return nil
}

func (e *PSEditor) Check(target, desired string) error {
	raw, start, end, err := e.read(target)
	if err != nil {
		return err
	}
	return checkProfile(raw, start, end, desired)
}

func (e *PSEditor) Apply(target, value string) (string, bool, error) {
	raw, start, end, err := e.read(target)
	if err != nil {
		return "", false, err
	}
	if err := checkProfile(raw, start, end, value); err != nil {
		return "", false, err
	}
	before := profileValue(raw, start, end)
	if before == value {
		return before, false, nil
	}
	newline := "\n"
	if bytes.Contains(raw, []byte("\r\n")) {
		newline = "\r\n"
	}
	block := strings.ReplaceAll(value, "\n", newline)
	var updated []byte
	if start >= 0 {
		updated = append(updated, raw[:start]...)
		updated = append(updated, block...)
		updated = append(updated, raw[end:]...)
	} else {
		updated = append(updated, raw...)
		if len(raw) > 0 && raw[len(raw)-1] != '\n' {
			updated = append(updated, newline...)
		}
		updated = append(updated, block...)
	}
	if err := writeAtomic(e.Path, updated); err != nil {
		return before, false, err
	}
	return before, true, nil
}

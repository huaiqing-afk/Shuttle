package configuration

import (
	"fmt"
	"strings"

	core "shuttle/internal/core/configuration"
)

// spec is the parsed form of a target identifier.
type spec struct {
	kind string   // scalar | profile | key
	path []string // scalar dot path, or array path for profile/key
	key  string   // scalar leaf name
	guid string   // profile match
	keys string   // keybinding match
	cmd  string   // keybinding command
}

func (s spec) arrayPath() []string { return s.path }

func parseTarget(target string) (spec, error) {
	switch {
	case strings.HasPrefix(target, "wt:scalar:"):
		parts := strings.Split(strings.TrimPrefix(target, "wt:scalar:"), ".")
		if len(parts) == 0 || parts[len(parts)-1] == "" {
			return spec{}, &core.Error{Kind: "validation_failed", Target: target, Reason: "空标量路径"}
		}
		return spec{kind: "scalar", path: parts[:len(parts)-1], key: parts[len(parts)-1]}, nil
	case strings.HasPrefix(target, "wt:profile:"):
		guid := strings.TrimPrefix(target, "wt:profile:")
		if guid == "" {
			return spec{}, &core.Error{Kind: "validation_failed", Target: target, Reason: "缺少 Profile GUID"}
		}
		return spec{kind: "profile", path: []string{"profiles", "list"}, guid: guid}, nil
	case strings.HasPrefix(target, "wt:key:"):
		parts := strings.SplitN(strings.TrimPrefix(target, "wt:key:"), "|", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return spec{}, &core.Error{Kind: "validation_failed", Target: target, Reason: "快捷键目标需为 keys|command"}
		}
		return spec{kind: "key", path: []string{"actions"}, keys: parts[0], cmd: parts[1]}, nil
	default:
		return spec{}, &core.Error{Kind: "unsupported", Target: target, Reason: "未知配置目标"}
	}
}

// valueSpan is a byte range of one JSON value plus its raw token text.
type valueSpan struct {
	start int
	end   int
	value string
}

func findValue(raw []byte, s spec) (valueSpan, bool, error) {
	if s.kind == "scalar" {
		return findScalar(raw, s.path, s.key)
	}
	return findArrayItem(raw, s.path, func(fields map[string]string) bool {
		if s.kind == "profile" {
			return unquoteToken(fields["guid"]) == s.guid
		}
		return unquoteToken(fields["keys"]) == s.keys && unquoteToken(fields["command"]) == s.cmd
	})
}

func findScalar(raw []byte, path []string, key string) (valueSpan, bool, error) {
	open, found, err := findContainer(raw, path)
	if err != nil || !found {
		return valueSpan{}, false, err
	}
	s := newScanner(raw)
	s.pos = open
	if err := s.skipSpace(); err != nil {
		return valueSpan{}, false, err
	}
	if s.peek() != '{' {
		return valueSpan{}, false, nil
	}
	s.pos++
	for {
		if err := s.skipSpace(); err != nil {
			return valueSpan{}, false, err
		}
		if s.eof() {
			return valueSpan{}, false, fmt.Errorf("对象未闭合")
		}
		if s.peek() == '}' {
			return valueSpan{}, false, nil
		}
		name, err := s.stringValue()
		if err != nil {
			return valueSpan{}, false, err
		}
		if err := s.skipSpace(); err != nil {
			return valueSpan{}, false, err
		}
		if s.peek() != ':' {
			return valueSpan{}, false, fmt.Errorf("键 %q 后缺少冒号", name)
		}
		s.pos++
		start, end, err := s.valueSpan()
		if err != nil {
			return valueSpan{}, false, err
		}
		if name == key {
			return valueSpan{start: start, end: end, value: string(raw[start:end])}, true, nil
		}
		if err := s.skipSpace(); err != nil {
			return valueSpan{}, false, err
		}
		if s.peek() == ',' {
			s.pos++
		}
	}
}

// findContainer returns the byte offset of the opening brace of the object at
// path, creating nothing.
func findContainer(raw []byte, path []string) (int, bool, error) {
	if len(path) == 0 {
		s := newScanner(raw)
		if err := s.skipSpace(); err != nil {
			return 0, false, err
		}
		if s.peek() != '{' {
			return 0, false, fmt.Errorf("文档根应为对象")
		}
		return s.pos, true, nil
	}
	open, found, err := findContainer(raw, path[:len(path)-1])
	if err != nil || !found {
		return 0, false, err
	}
	s := newScanner(raw)
	s.pos = open + 1
	for {
		if err := s.skipSpace(); err != nil {
			return 0, false, err
		}
		if s.eof() {
			return 0, false, fmt.Errorf("对象未闭合")
		}
		if s.peek() == '}' {
			return 0, false, nil
		}
		name, err := s.stringValue()
		if err != nil {
			return 0, false, err
		}
		if err := s.skipSpace(); err != nil {
			return 0, false, err
		}
		if s.peek() != ':' {
			return 0, false, fmt.Errorf("键 %q 后缺少冒号", name)
		}
		s.pos++
		start, _, err := s.valueSpan()
		if err != nil {
			return 0, false, err
		}
		if name == path[len(path)-1] {
			if raw[start] != '{' && raw[start] != '[' {
				return 0, false, nil
			}
			return start, true, nil
		}
		if err := s.skipSpace(); err != nil {
			return 0, false, err
		}
		if s.peek() == ',' {
			s.pos++
		}
	}
}

func findArrayItem(raw []byte, path []string, match func(map[string]string) bool) (valueSpan, bool, error) {
	open, found, err := findContainer(raw, path)
	if err != nil || !found {
		return valueSpan{}, false, err
	}
	s := newScanner(raw)
	s.pos = open
	if err := s.skipSpace(); err != nil {
		return valueSpan{}, false, err
	}
	if s.peek() != '[' {
		return valueSpan{}, false, nil
	}
	s.pos++
	for {
		if err := s.skipSpace(); err != nil {
			return valueSpan{}, false, err
		}
		if s.eof() {
			return valueSpan{}, false, fmt.Errorf("数组未闭合")
		}
		if s.peek() == ']' {
			return valueSpan{}, false, nil
		}
		start, end, err := s.valueSpan()
		if err != nil {
			return valueSpan{}, false, err
		}
		if raw[start] == '{' {
			fields, err := objectFields(raw[start:end])
			if err != nil {
				return valueSpan{}, false, err
			}
			if match(fields) {
				return valueSpan{start: start, end: end, value: string(raw[start:end])}, true, nil
			}
		}
		if err := s.skipSpace(); err != nil {
			return valueSpan{}, false, err
		}
		if s.peek() == ',' {
			s.pos++
		}
	}
}

// objectFields flattens one object's members into key -> raw token.
func objectFields(raw []byte) (map[string]string, error) {
	out := map[string]string{}
	s := newScanner(raw)
	if err := s.skipSpace(); err != nil {
		return nil, err
	}
	if s.peek() != '{' {
		return nil, fmt.Errorf("应为对象")
	}
	s.pos++
	for {
		if err := s.skipSpace(); err != nil {
			return nil, err
		}
		if s.eof() {
			return nil, fmt.Errorf("对象未闭合")
		}
		if s.peek() == '}' {
			return out, nil
		}
		name, err := s.stringValue()
		if err != nil {
			return nil, err
		}
		if err := s.skipSpace(); err != nil {
			return nil, err
		}
		if s.peek() != ':' {
			return nil, fmt.Errorf("键 %q 后缺少冒号", name)
		}
		s.pos++
		start, end, err := s.valueSpan()
		if err != nil {
			return nil, err
		}
		out[name] = string(raw[start:end])
		if err := s.skipSpace(); err != nil {
			return nil, err
		}
		if s.peek() == ',' {
			s.pos++
		}
	}
}

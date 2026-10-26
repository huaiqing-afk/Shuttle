package configuration

import (
	"encoding/json"
	"fmt"
	"strings"

	core "shuttle/internal/core/configuration"
)

func splice(raw []byte, s valueSpan, value string) []byte {
	out := make([]byte, 0, len(raw)-(s.end-s.start)+len(value))
	out = append(out, raw[:s.start]...)
	out = append(out, value...)
	out = append(out, raw[s.end:]...)
	return out
}

// removeValue deletes a span together with one adjacent comma so the document
// stays valid; the rest of the file is untouched.
func removeValue(raw []byte, s valueSpan) ([]byte, error) {
	start := s.start
	for start > 0 && isSpace(raw[start-1]) {
		start--
	}
	end := s.end
	if start > 0 && raw[start-1] == ',' {
		start--
	} else {
		for end < len(raw) && isSpace(raw[end]) {
			end++
		}
		if end < len(raw) && raw[end] == ',' {
			end++
		}
	}
	out := make([]byte, 0, len(raw)-(end-start))
	out = append(out, raw[:start]...)
	out = append(out, raw[end:]...)
	return out, nil
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// insertValue adds a member or array item, matching the file's 4-space indent.
func insertValue(raw []byte, s spec, value string) ([]byte, error) {
	if s.kind == "scalar" {
		open, found, err := findContainer(raw, s.path)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, &core.Error{Kind: "not_found", Reason: fmt.Sprintf("缺少容器 %s", strings.Join(s.path, "."))}
		}
		encodedKey, _ := json.Marshal(s.key)
		line := fmt.Sprintf("%s: %s,", encodedKey, value)
		return insertAfterOpen(raw, open, line)
	}
	open, found, err := findContainer(raw, s.arrayPath())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, &core.Error{Kind: "not_found", Reason: fmt.Sprintf("缺少容器 %s", strings.Join(s.arrayPath(), "."))}
	}
	return appendToArray(raw, open, value)
}

func insertAfterOpen(raw []byte, open int, line string) ([]byte, error) {
	s := newScanner(raw)
	s.pos = open + 1
	if err := s.skipSpace(); err != nil {
		return nil, err
	}
	indent := "\n    "
	if s.pos < len(raw) && s.peek() == '}' {
		indent = "\n"
	}
	payload := indent + line + "\n"
	out := make([]byte, 0, len(raw)+len(payload))
	out = append(out, raw[:open+1]...)
	out = append(out, payload...)
	out = append(out, raw[open+1:]...)
	return out, nil
}

func appendToArray(raw []byte, open int, value string) ([]byte, error) {
	s := newScanner(raw)
	s.pos = open
	if err := s.skipSpace(); err != nil {
		return nil, err
	}
	if s.peek() != '[' {
		return nil, &core.Error{Kind: "validation_failed", Reason: "目标容器不是数组"}
	}
	// walk to the closing bracket
	s.pos++
	last := 0
	for {
		if err := s.skipSpace(); err != nil {
			return nil, err
		}
		if s.eof() {
			return nil, fmt.Errorf("数组未闭合")
		}
		if s.peek() == ']' {
			closeAt := s.pos
			empty := last == 0
			payload := "\n        " + value + ","
			if !empty {
				// insert after the last item, before its optional trailing comma
				insertAt := last
				for insertAt < closeAt && isSpace(raw[insertAt]) {
					insertAt++
				}
				if insertAt < closeAt && raw[insertAt] == ',' {
					insertAt++
				}
				payload = "\n        " + value + ","
				out := make([]byte, 0, len(raw)+len(payload))
				out = append(out, raw[:insertAt]...)
				out = append(out, payload...)
				out = append(out, raw[insertAt:]...)
				return out, nil
			}
			out := make([]byte, 0, len(raw)+len(payload))
			out = append(out, raw[:closeAt]...)
			out = append(out, payload...)
			out = append(out, '\n', ' ', ' ', ' ', ' ')
			out = append(out, raw[closeAt:]...)
			return out, nil
		}
		start, end, err := s.valueSpan()
		if err != nil {
			return nil, err
		}
		_ = start
		last = end
		if err := s.skipSpace(); err != nil {
			return nil, err
		}
		if s.peek() == ',' {
			s.pos++
		}
	}
}

// checkConflict implements the identity rules from config-contract §4.1:
// the same profile name bound to a different GUID is a conflict, never a
// silent overwrite.

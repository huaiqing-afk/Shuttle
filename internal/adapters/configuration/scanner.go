package configuration

import (
	"fmt"
	"strconv"
	"strings"
)

// scanner is a minimal JSONC reader: it understands comments and trailing
// commas, and reports byte offsets so callers can splice the original text.
type scanner struct {
	raw []byte
	pos int
}

func newScanner(raw []byte) *scanner { return &scanner{raw: raw} }

func (s *scanner) eof() bool { return s.pos >= len(s.raw) }

func (s *scanner) skipSpace() error {
	for !s.eof() {
		switch s.raw[s.pos] {
		case ' ', '\t', '\r', '\n':
			s.pos++
		case '/':
			if s.pos+1 < len(s.raw) && s.raw[s.pos+1] == '/' {
				for !s.eof() && s.raw[s.pos] != '\n' {
					s.pos++
				}
			} else if s.pos+1 < len(s.raw) && s.raw[s.pos+1] == '*' {
				s.pos += 2
				for s.pos+1 < len(s.raw) && !(s.raw[s.pos] == '*' && s.raw[s.pos+1] == '/') {
					s.pos++
				}
				if s.pos+1 >= len(s.raw) {
					return fmt.Errorf("JSONC 注释未闭合")
				}
				s.pos += 2
			} else {
				return fmt.Errorf("非法字符 / 于偏移 %d", s.pos)
			}
		default:
			return nil
		}
	}
	return nil
}

func (s *scanner) peek() byte {
	if s.eof() {
		return 0
	}
	return s.raw[s.pos]
}

// valueSpan returns the byte range of one JSON value starting at pos.
func (s *scanner) valueSpan() (start, end int, err error) {
	if err := s.skipSpace(); err != nil {
		return 0, 0, err
	}
	start = s.pos
	if s.eof() {
		return 0, 0, fmt.Errorf("文档在偏移 %d 处意外结束", s.pos)
	}
	switch c := s.raw[s.pos]; {
	case c == '{' || c == '[':
		depth := 0
		inString := false
		for !s.eof() {
			ch := s.raw[s.pos]
			if inString {
				if ch == '\\' {
					s.pos += 2
					continue
				}
				if ch == '"' {
					inString = false
				}
			} else {
				switch ch {
				case '"':
					inString = true
				case '{', '[':
					depth++
				case '}', ']':
					depth--
					if depth == 0 {
						s.pos++
						return start, s.pos, nil
					}
				}
			}
			s.pos++
		}
		return 0, 0, fmt.Errorf("结构在偏移 %d 处未闭合", start)
	case c == '"':
		if err := s.skipString(); err != nil {
			return 0, 0, err
		}
		return start, s.pos, nil
	default:
		for !s.eof() {
			ch := s.raw[s.pos]
			if ch == ',' || ch == '}' || ch == ']' || ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == '/' {
				break
			}
			s.pos++
		}
		if s.pos == start {
			return 0, 0, fmt.Errorf("偏移 %d 处缺少值", start)
		}
		return start, s.pos, nil
	}
}

func (s *scanner) skipString() error {
	s.pos++ // opening quote
	for !s.eof() {
		switch s.raw[s.pos] {
		case '\\':
			s.pos += 2
		case '"':
			s.pos++
			return nil
		default:
			s.pos++
		}
	}
	return fmt.Errorf("字符串未闭合")
}

// object parses one object at the scanner position, returning key -> raw value
// (all nested keys flattened with dotted identifiers).
func (s *scanner) object(prefix string) (map[string]string, error) {
	out := map[string]string{}
	if err := s.skipSpace(); err != nil {
		return nil, err
	}
	if s.peek() != '{' {
		return nil, fmt.Errorf("偏移 %d 处应为对象", s.pos)
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
			s.pos++
			return out, nil
		}
		key, err := s.stringValue()
		if err != nil {
			return nil, err
		}
		if err := s.skipSpace(); err != nil {
			return nil, err
		}
		if s.peek() != ':' {
			return nil, fmt.Errorf("键 %q 后缺少冒号", key)
		}
		s.pos++
		start, end, err := s.valueSpan()
		if err != nil {
			return nil, err
		}
		full := prefix + key
		out[full] = strings.TrimSpace(string(s.raw[start:end]))
		if s.raw[start] == '{' {
			nested, err := newScanner(s.raw[start:end]).object(full + ".")
			if err == nil {
				for k, v := range nested {
					out[k] = v
				}
			}
		}
		if err := s.skipSpace(); err != nil {
			return nil, err
		}
		if s.peek() == ',' {
			s.pos++
		}
	}
}

func (s *scanner) stringValue() (string, error) {
	if err := s.skipSpace(); err != nil {
		return "", err
	}
	if s.peek() != '"' {
		return "", fmt.Errorf("偏移 %d 处应为字符串键", s.pos)
	}
	start := s.pos
	if err := s.skipString(); err != nil {
		return "", err
	}
	unquoted, err := strconv.Unquote(string(s.raw[start:s.pos]))
	if err != nil {
		return "", err
	}
	return unquoted, nil
}

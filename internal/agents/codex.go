package agents

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type codexMetaPayload struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
}

type codexLine struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type codexMessagePayload struct {
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type codexTurnPayload struct {
	Model string `json:"model"`
}

// CodexSessions walks ~/.codex/sessions/**/rollout-*.jsonl (layout: year/month/day).
func CodexSessions(root string) []Session {
	var out []Session
	_ = filepath.WalkDir(filepath.Join(root, "sessions"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(entry.Name(), ".jsonl") {
			return nil
		}
		if s, ok := parseCodexSession(path); ok {
			out = append(out, s)
		}
		return nil
	})
	return out
}

func parseCodexSession(path string) (Session, bool) {
	file, err := os.Open(path)
	if err != nil {
		return Session{}, false
	}
	defer file.Close()
	s := Session{Agent: "codex"}
	title := ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
	for scanner.Scan() {
		var line codexLine
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		switch line.Type {
		case "session_meta":
			var meta codexMetaPayload
			if json.Unmarshal(line.Payload, &meta) == nil && meta.SessionID != "" {
				s.ID, s.Workspace = meta.SessionID, meta.Cwd
			}
		case "turn_context":
			var turn codexTurnPayload
			if json.Unmarshal(line.Payload, &turn) == nil && turn.Model != "" {
				s.Model = turn.Model
			}
		case "response_item":
			var msg codexMessagePayload
			if json.Unmarshal(line.Payload, &msg) != nil || msg.Role != "user" {
				continue
			}
			for _, block := range msg.Content {
				text := strings.TrimSpace(block.Text)
				if text == "" || strings.HasPrefix(text, "#") || strings.HasPrefix(text, "<") ||
					strings.Contains(text, "AGENTS.md instructions") || strings.Contains(text, "<ENVIRONMENT_") {
					continue
				}
				s.Turns++
				if title == "" {
					title = firstLine(text, 80)
				}
			}
		}
	}
	if s.ID == "" || s.Workspace == "" {
		return Session{}, false
	}
	s.Title = title
	s.LastActive = fileLastActive(path)
	return s, true
}

package agents

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type claudeLine struct {
	Type      string          `json:"type"`
	Summary   string          `json:"summary"`
	Cwd       string          `json:"cwd"`
	GitBranch string          `json:"gitBranch"`
	Model     string          `json:"model"`
	IsMeta    bool            `json:"isMeta"`
	IsSide    bool            `json:"isSidechain"`
	Message   json.RawMessage `json:"message"`
	IsCompact bool            `json:"isCompactSummary"`
}

type claudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type claudeRunning struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
	Name      string `json:"name"`
	Cwd       string `json:"cwd"`
}

// PIDCheck lets tests substitute liveness so fixtures never depend on real processes.
type PIDCheck func(pid int) bool

// ClaudeSessions scans ~/.claude/projects and merges run-state from ~/.claude/sessions.
func ClaudeSessions(root string, alive PIDCheck) []Session {
	projects := filepath.Join(root, "projects")
	running := claudeRunningMap(filepath.Join(root, "sessions"), alive)
	var out []Session
	entries, err := os.ReadDir(projects)
	if err != nil {
		return out
	}
	for _, dir := range entries {
		if !dir.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(projects, dir.Name()))
		if err != nil {
			continue
		}
		for _, file := range files {
			name := file.Name()
			if file.IsDir() || !strings.HasSuffix(name, ".jsonl") || strings.HasPrefix(name, "agent-") {
				continue
			}
			if s, ok := parseClaudeSession(filepath.Join(projects, dir.Name(), name)); ok {
				if run, exists := running[s.ID]; exists {
					s.Running = true
					s.Status = run.Status
					s.Name = run.Name
				}
				out = append(out, s)
			}
		}
	}
	return out
}

func parseClaudeSession(path string) (Session, bool) {
	file, err := os.Open(path)
	if err != nil {
		return Session{}, false
	}
	defer file.Close()
	s := Session{Agent: "claude", ID: strings.TrimSuffix(filepath.Base(path), ".jsonl")}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
	title := ""
	summary := ""
	for scanner.Scan() {
		// shortcut: full-file scan per refresh; on multi-hundred-MB histories switch to tail-sampled parsing
		var line claudeLine
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		if line.Cwd != "" && s.Workspace == "" {
			s.Workspace = line.Cwd
		}
		if line.GitBranch != "" && s.GitBranch == "" {
			s.GitBranch = line.GitBranch
		}
		if line.Model != "" {
			s.Model = line.Model
		}
		if line.Type == "summary" && line.Summary != "" && summary == "" {
			summary = line.Summary
		}
		if line.Type != "user" || line.IsMeta || line.IsSide {
			continue
		}
		var msg claudeMessage
		if json.Unmarshal(line.Message, &msg) != nil || msg.Role != "user" {
			continue
		}
		text := claudeText(msg.Content)
		if text == "" || strings.HasPrefix(text, "<") {
			continue
		}
		if title == "" {
			title = firstLine(strings.TrimPrefix(text, "session-"+s.ID+" "), 80)
		}
	}
	if s.Workspace == "" {
		return Session{}, false
	}
	if summary != "" {
		title = summary
	}
	s.Title = title
	s.LastActive = fileLastActive(path)
	return s, true
}

func claudeText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		return strings.TrimSpace(asString)
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return strings.TrimSpace(block.Text)
		}
	}
	return ""
}

func claudeRunningMap(dir string, alive PIDCheck) map[string]claudeRunning {
	out := map[string]claudeRunning{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var run claudeRunning
		if json.Unmarshal(raw, &run) != nil || run.SessionID == "" || run.PID <= 0 {
			continue
		}
		if alive(run.PID) {
			out[run.SessionID] = run
		}
	}
	return out
}

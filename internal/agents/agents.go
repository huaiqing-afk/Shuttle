package agents

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Session is the shared metadata shape for every agent; unknown fields stay zero-valued.
type Session struct {
	Agent      string `json:"agent"`
	ID         string `json:"id"`
	Title      string `json:"title"`
	Workspace  string `json:"workspace"`
	Model      string `json:"model"`
	Turns      int    `json:"turns"`
	LastActive int64  `json:"lastActive"` // unix milliseconds
	GitBranch  string `json:"gitBranch"`
	Running    bool   `json:"running"`
	Status     string `json:"status"` // busy / idle / ""
	Name       string `json:"name"`   // user-set display name, when the agent exposes one
}

// LaunchSpec is what a resume needs: an executable plus argv, never a shell string.
type LaunchSpec struct {
	Executable string
	Args       []string
	Dir        string
	Label      string
}

// MaxSessions caps a single refresh so pathological histories cannot stall the UI.
const MaxSessions = 200

// List discovers sessions from the standard ~/.claude and ~/.codex layouts under home.
func List(home string) []Session {
	var all []Session
	all = append(all, ClaudeSessions(filepath.Join(home, ".claude"), pidAlive)...)
	all = append(all, CodexSessions(filepath.Join(home, ".codex"))...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].LastActive > all[j].LastActive })
	if len(all) > MaxSessions {
		all = all[:MaxSessions]
	}
	return all
}

// ResumeSpec finds one session's workspace and builds its resume command line.
func ResumeSpec(home, agent, sessionID string) (LaunchSpec, error) {
	switch agent {
	case "claude":
		return claudeResume(filepath.Join(home, ".claude"), sessionID)
	case "codex":
		return codexResume(filepath.Join(home, ".codex"), sessionID)
	default:
		return LaunchSpec{}, os.ErrInvalid
	}
}

func fileLastActive(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now().UnixMilli()
	}
	return info.ModTime().UnixMilli()
}

func firstLine(text string, limit int) string {
	runes := []rune(text)
	if len(runes) > limit {
		runes = runes[:limit]
	}
	for len(runes) > 0 && runes[len(runes)-1] == ' ' {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

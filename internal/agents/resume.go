package agents

import (
	"errors"
	"os/exec"
)

// claudeResume locates the session workspace, then resumes via `claude --resume <id>`.
func claudeResume(root, sessionID string) (LaunchSpec, error) {
	if sessionID == "" {
		return LaunchSpec{}, errors.New("missing claude session id")
	}
	for _, s := range ClaudeSessions(root, pidAlive) {
		if s.ID == sessionID {
			target, err := exec.LookPath("claude.cmd")
			if err != nil {
				target, err = exec.LookPath("claude.exe")
			}
			if err != nil {
				return LaunchSpec{}, err
			}
			return LaunchSpec{Executable: target, Args: []string{"--resume", sessionID}, Dir: s.Workspace, Label: "Claude 会话"}, nil
		}
	}
	return LaunchSpec{}, errors.New("claude session not found: " + sessionID)
}

// codexResume locates the session workspace, then resumes via `codex resume <uuid>`.
func codexResume(root, sessionID string) (LaunchSpec, error) {
	if sessionID == "" {
		return LaunchSpec{}, errors.New("missing codex session id")
	}
	for _, s := range CodexSessions(root) {
		if s.ID == sessionID {
			target, err := exec.LookPath("codex.cmd")
			if err != nil {
				target, err = exec.LookPath("codex.exe")
			}
			if err != nil {
				return LaunchSpec{}, err
			}
			return LaunchSpec{Executable: target, Args: []string{"resume", sessionID}, Dir: s.Workspace, Label: "Codex 会话"}, nil
		}
	}
	return LaunchSpec{}, errors.New("codex session not found: " + sessionID)
}

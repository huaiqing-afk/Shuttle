package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeSessionsParseAndRunningFilter(t *testing.T) {
	root := t.TempDir()
	id := "11111111-2222-3333-4444-555555555555"
	transcript := `{"type":"user","isMeta":true,"cwd":"D:\\ws\\demo","message":{"role":"user","content":"<command-name>/x</command-name>"}}
{"type":"user","cwd":"D:\\ws\\demo","gitBranch":"main","message":{"role":"user","content":"session-` + id + ` 帮我修这个 bug"}}
{"type":"assistant","cwd":"D:\\ws\\demo","model":"glm-5.3","message":{"role":"assistant","content":"好的"}}
{"type":"summary","summary":"压缩后的标题"}
`
	write(t, filepath.Join(root, "projects", "D--ws-demo", id+".jsonl"), transcript)
	write(t, filepath.Join(root, "projects", "D--ws-demo", "agent-9999.jsonl"), transcript) // subagent transcripts are skipped

	write(t, filepath.Join(root, "sessions", "777.json"),
		`{"pid":777,"sessionId":"`+id+`","status":"busy","name":"演示会话","cwd":"D:\\ws\\demo"}`)
	write(t, filepath.Join(root, "sessions", "888.json"),
		`{"pid":888,"sessionId":"dead-session","status":"idle"}`)

	sessions := ClaudeSessions(root, func(pid int) bool { return pid == 777 })
	if len(sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.ID != id || s.Agent != "claude" || s.Workspace != `D:\ws\demo` {
		t.Fatalf("identity mismatch: %+v", s)
	}
	if s.Title != "压缩后的标题" || s.Model != "glm-5.3" || s.GitBranch != "main" {
		t.Fatalf("fields mismatch: %+v", s)
	}
	if !s.Running || s.Status != "busy" || s.Name != "演示会话" {
		t.Fatalf("running state not merged: %+v", s)
	}
}

func TestCodexSessionsParse(t *testing.T) {
	root := t.TempDir()
	meta := `{"type":"session_meta","payload":{"session_id":"01a1b2c3-d4e5-f6a7-b8c9-d0e1f2a3b4c5","cwd":"D:\\hojoai\\code\\demo"}}`
	wrapped := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions\n<INSTRUCTIONS>"}]}}`
	real := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"看下这个模块怎么拆"}]}}`
	turn := `{"type":"turn_context","payload":{"model":"cursor/grok-4.7@256k"}}`
	write(t, filepath.Join(root, "sessions", "2026", "10", "24", "rollout-2026-10-24T09-00-00-01a1b2c3-d4e5-f6a7-b8c9-d0e1f2a3b4c5.jsonl"),
		meta+"\n"+wrapped+"\n"+real+"\n"+turn+"\n")

	sessions := CodexSessions(root)
	if len(sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.Agent != "codex" || s.ID != "01a1b2c3-d4e5-f6a7-b8c9-d0e1f2a3b4c5" {
		t.Fatalf("identity mismatch: %+v", s)
	}
	if s.Title != "看下这个模块怎么拆" {
		t.Fatalf("injected wrapper used as title: %+v", s)
	}
	if s.Model != "cursor/grok-4.7@256k" || s.Turns != 1 {
		t.Fatalf("fields mismatch: %+v", s)
	}
}

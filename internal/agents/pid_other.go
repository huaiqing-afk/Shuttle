//go:build !windows

package agents

// pidAlive is only meaningful on Windows where Claude stores per-pid session files.
func pidAlive(pid int) bool { return false }

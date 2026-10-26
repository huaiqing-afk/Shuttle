package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	installCommandTimeout = 15 * time.Minute
	installOutputLimit    = 64 * 1024
)

type commandInstaller struct{}

func (commandInstaller) Run(argv []string) (int, string, error) {
	if len(argv) == 0 {
		return -1, "", errors.New("空命令")
	}
	path, args, err := resolveCommand(argv[0], argv[1:]...)
	if err != nil {
		return -1, "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), installCommandTimeout)
	defer cancel()
	return runInstallCommand(ctx, path, args)
}

// resolveCommand translates npm's Windows shim into a direct node argv, without a shell.
func resolveCommand(name string, args ...string) (string, []string, error) {
	switch strings.ToLower(name) {
	case "npm", "npm.cmd":
		shim, err := lookPath("npm.cmd")
		if err != nil {
			return "", nil, err
		}
		cli := filepath.Join(filepath.Dir(shim), "node_modules", "npm", "bin", "npm-cli.js")
		if info, err := os.Stat(cli); err != nil {
			return "", nil, fmt.Errorf("npm CLI 不可用 %s: %w", cli, err)
		} else if !info.Mode().IsRegular() {
			return "", nil, fmt.Errorf("npm CLI 不是普通文件: %s", cli)
		}
		node := filepath.Join(filepath.Dir(shim), "node.exe")
		if info, err := os.Stat(node); err != nil || !info.Mode().IsRegular() {
			node, err = lookPath("node.exe")
			if err != nil {
				return "", nil, err
			}
		}
		return node, append([]string{cli}, args...), nil
	case "winget", "winget.exe":
		path, err := lookPath("winget.exe")
		return path, append([]string(nil), args...), err
	default:
		return "", nil, fmt.Errorf("不支持的安装命令 %q", name)
	}
}

func lookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("找不到 %s，请确认已安装并在 PATH 上: %w", name, err)
	}
	return path, nil
}

func runInstallCommand(ctx context.Context, path string, args []string) (int, string, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.WaitDelay = 2 * time.Second
	output := &installOutputTail{}
	cmd.Stdout, cmd.Stderr = output, output
	err := cmd.Run()
	message := strings.TrimSpace(string(output.data))
	if err == nil {
		return 0, message, nil
	}
	code := -1
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	}
	if ctx.Err() != nil {
		err = fmt.Errorf("%w: %v", ctx.Err(), err)
	}
	return code, message, fmt.Errorf("%s 执行失败: %w; 输出尾部: %s", filepath.Base(path), err, message)
}

// os/exec serializes writes when stdout and stderr share the same writer.
type installOutputTail struct{ data []byte }

func (b *installOutputTail) Write(p []byte) (int, error) {
	n := len(p)
	if n >= installOutputLimit {
		b.data = append(b.data[:0], p[n-installOutputLimit:]...)
	} else {
		if excess := len(b.data) + n - installOutputLimit; excess > 0 {
			copy(b.data, b.data[excess:])
			b.data = b.data[:len(b.data)-excess]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// resolveAgentCwd resolves the working directory for an agent subprocess
// against `config workspace-root`. When node.LLMAgentCwd is empty a tmp
// directory is created inside workspaceRoot and the returned cleanup
// removes it. When declared, the path is :param-expanded, resolved with
// EvalSymlinks, and validated to live inside workspaceRoot; cleanup is a
// no-op (the admin owns the lifecycle of declared directories).
func resolveAgentCwd(node parser.Node, app *parser.App, params map[string]string) (string, func(), error) {
	workspaceRoot := ""
	if app != nil && app.Config != nil {
		workspaceRoot = app.Config.WorkspaceRoot
	}
	if workspaceRoot == "" {
		return "", func() {}, fmt.Errorf("agent cwd: `config workspace-root` is required")
	}

	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", func() {}, fmt.Errorf("agent cwd: workspace-root abs: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o750); err != nil {
		return "", func() {}, fmt.Errorf("agent cwd: workspace-root mkdir: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", func() {}, fmt.Errorf("agent cwd: workspace-root symlink eval: %w", err)
	}

	if node.LLMAgentCwd == "" {
		tmp, err := os.MkdirTemp(realRoot, "kilnx-agent-*")
		if err != nil {
			return "", func() {}, fmt.Errorf("agent cwd: mkdir tmp: %w", err)
		}
		cleanup := func() { _ = os.RemoveAll(tmp) }
		return tmp, cleanup, nil
	}

	expanded := substituteParams(node.LLMAgentCwd, params)
	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", func() {}, fmt.Errorf("agent cwd: abs: %w", err)
	}
	if err := os.MkdirAll(absPath, 0o750); err != nil {
		return "", func() {}, fmt.Errorf("agent cwd: mkdir: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", func() {}, fmt.Errorf("agent cwd: symlink eval: %w", err)
	}
	sep := string(os.PathSeparator)
	if !strings.HasPrefix(realPath+sep, realRoot+sep) {
		return "", func() {}, fmt.Errorf("agent cwd: %q escapes workspace-root %q", realPath, realRoot)
	}
	return realPath, func() {}, nil
}

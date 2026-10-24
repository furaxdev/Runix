// This file implements the read-only and write file tools from
// docs/RUNYX_CODE.md §2: read_file, list_dir, search (read-only, no
// approval) and write_file, edit_file, delete (approval-gated via the
// same internal/approval.Queue the shell agent already uses). Every
// path argument is untrusted model input and goes through
// Sandbox.Resolve before any os call touches it.
package codeagent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/furaxdev/runyx/internal/agent"
	"github.com/furaxdev/runyx/internal/approval"
)

// Tools wires the sandbox and approval queue together into the actual
// tool implementations the agent loop calls. ReasonPrefix, if set, is
// prepended to every approval Request's Summary (e.g. a session id),
// purely for a human skimming a list of pending approvals across
// several sessions.
type Tools struct {
	Sandbox   *Sandbox
	Approvals *approval.Queue
}

// NewTools returns Tools scoped to one project root and approval queue.
func NewTools(root string, approvals *approval.Queue) (*Tools, error) {
	sb, err := NewSandbox(root)
	if err != nil {
		return nil, err
	}
	return &Tools{Sandbox: sb, Approvals: approvals}, nil
}

// ReadFile returns the file's content, optionally restricted to
// [startLine, endLine] (1-indexed, inclusive; 0 means "unbounded" on
// either end).
func (t *Tools) ReadFile(path string, startLine, endLine int) (string, error) {
	real, err := t.Sandbox.Resolve(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(real)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}
	if startLine == 0 && endLine == 0 {
		return string(data), nil
	}
	lines := strings.Split(string(data), "\n")
	if startLine <= 0 {
		startLine = 1
	}
	if endLine <= 0 || endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > len(lines) || startLine > endLine {
		return "", nil
	}
	return strings.Join(lines[startLine-1:endLine], "\n"), nil
}

// DirEntry is one entry returned by ListDir.
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ListDir lists the immediate children of path (default ".").
func (t *Tools) ListDir(path string) ([]DirEntry, error) {
	if path == "" {
		path = "."
	}
	real, err := t.Sandbox.Resolve(path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		return nil, fmt.Errorf("list_dir: %w", err)
	}
	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		// Entries whose own name is on the deny-list (e.g. a nested
		// .ssh/) are still listed by name -- hiding existence would be
		// surprising -- but a subsequent read_file/list_dir into them
		// is refused by Resolve, same as for any other denied path.
		info, err := e.Info()
		var size int64
		if err == nil {
			size = info.Size()
		}
		out = append(out, DirEntry{Name: e.Name(), IsDir: e.IsDir(), Size: size})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// SearchMatch is one line matched by Search.
type SearchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// Search walks path (default ".") and returns up to maxResults lines
// matching the regex pattern. Pure Go (no rg dependency) -- simple and
// fully testable; a subprocess ripgrep backend can be layered in later
// without changing this signature.
func (t *Tools) Search(pattern, path string, maxResults int) ([]SearchMatch, error) {
	if path == "" {
		path = "."
	}
	if maxResults <= 0 {
		maxResults = 50
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("search: invalid pattern: %w", err)
	}
	real, err := t.Sandbox.Resolve(path)
	if err != nil {
		return nil, err
	}
	var out []SearchMatch
	err = filepath.Walk(real, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries rather than aborting the whole walk
		}
		if len(out) >= maxResults {
			return filepath.SkipAll
		}
		if info.IsDir() {
			if name := info.Name(); name == ".git" || denyDirNames[name] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(t.Sandbox.Root, p)
		if err != nil {
			return nil
		}
		if denied, _ := isDenied(rel); denied {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			if re.MatchString(scanner.Text()) {
				out = append(out, SearchMatch{Path: filepath.ToSlash(rel), Line: lineNo, Text: scanner.Text()})
				if len(out) >= maxResults {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return out, nil
}

// requestApproval blocks on the approval queue for a write-shaped
// action. If t.Approvals is nil, every such action is refused -- fail
// closed, matching internal/agent's "no approval mechanism wired up:
// never default-allow" rule.
func (t *Tools) requestApproval(ctx context.Context, kind, summary, detail string) error {
	if t.Approvals == nil {
		return fmt.Errorf("%s refused: no approval queue configured", kind)
	}
	req := approval.NewRequest(kind, summary, detail)
	decision, err := t.Approvals.Submit(ctx, req)
	if err != nil {
		return fmt.Errorf("%s: %w", kind, err)
	}
	if !decision.Allow {
		return fmt.Errorf("%s refused by user", kind)
	}
	return nil
}

// WriteFile creates or fully overwrites path. Requires approval.
func (t *Tools) WriteFile(ctx context.Context, path, content, reason string) error {
	real, err := t.Sandbox.Resolve(path)
	if err != nil {
		return err
	}
	summary := fmt.Sprintf("Write %s (%d bytes)", path, len(content))
	detail := fmt.Sprintf("Path: %s\nReason: %s\n\n%s", path, reason, content)
	if err := t.requestApproval(ctx, "write_file", summary, detail); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		return fmt.Errorf("write_file: %w", err)
	}
	if err := os.WriteFile(real, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write_file: %w", err)
	}
	return nil
}

// EditFile applies a unified diff to path. Requires approval. Refuses
// if any hunk's context/removed lines don't match the file's current
// content -- stale-diff protection, per docs/RUNYX_CODE.md §2.
func (t *Tools) EditFile(ctx context.Context, path, diff, reason string) error {
	real, err := t.Sandbox.Resolve(path)
	if err != nil {
		return err
	}
	original, err := os.ReadFile(real)
	if err != nil {
		return fmt.Errorf("edit_file: %w", err)
	}
	patched, err := applyUnifiedDiff(string(original), diff)
	if err != nil {
		return fmt.Errorf("edit_file: %w", err)
	}
	summary := fmt.Sprintf("Edit %s", path)
	detail := fmt.Sprintf("Path: %s\nReason: %s\n\n%s", path, reason, diff)
	if err := t.requestApproval(ctx, "edit_file", summary, detail); err != nil {
		return err
	}
	if err := os.WriteFile(real, []byte(patched), 0o644); err != nil {
		return fmt.Errorf("edit_file: %w", err)
	}
	return nil
}

// Delete removes path, after snapshotting the project's git history
// (if it's a git repo) so the removal is undoable the same way
// internal/agent.Agent's command snapshots already are -- never a
// bare, unrecoverable os.Remove. Requires approval.
func (t *Tools) Delete(ctx context.Context, path, reason string) error {
	real, err := t.Sandbox.Resolve(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(real); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	summary := fmt.Sprintf("Delete %s", path)
	detail := fmt.Sprintf("Path: %s\nReason: %s", path, reason)
	if err := t.requestApproval(ctx, "delete", summary, detail); err != nil {
		return err
	}
	// Snapshot ensures a git repo exists (initializing one if needed)
	// and commits the current state first, so this delete is undoable
	// via agent.Undo. A snapshot failure (e.g. no git binary) doesn't
	// block an explicitly-approved action, it just means this
	// particular delete won't be undoable -- same stance as
	// internal/agent.Agent.Execute.
	_, _ = agent.Snapshot(ctx, t.Sandbox.Root, "before delete: "+path)
	if err := os.RemoveAll(real); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

// Package codeagent implements Runyx Code's tool-calling agent loop:
// a sandboxed view onto one project directory, the tools that read
// and change it, and the loop that ties them to a model. See
// docs/RUNYX_CODE.md for the full design. This file is the sandbox
// every file tool goes through first -- the threat model's first row
// ("Files outside the project"), enforced on every single call, not
// just at approval time.
package codeagent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sandbox resolves and validates paths against one project root.
type Sandbox struct {
	// Root is the project directory, already absolute and with its own
	// symlinks resolved once at construction.
	Root string
}

// NewSandbox resolves root to an absolute, symlink-free path and
// returns a Sandbox scoped to it. root must already exist.
func NewSandbox(root string) (*Sandbox, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("resolve project root (symlinks): %w", err)
	}
	return &Sandbox{Root: resolved}, nil
}

// denyBasenames blocks these filenames anywhere under the project
// root, even though they're inside it -- a project can contain a
// .env the agent must still never read, per docs/RUNYX_CODE.md's
// threat model ("Deny-list checked even inside the project root").
var denyBasenames = map[string]bool{
	".env":             true,
	".npmrc":           true,
	".netrc":           true,
	"credentials.json": true,
	"kaggle.json":      true,
}

// denyDirNames blocks any path with one of these as a path component,
// anywhere under the root (e.g. a nested .ssh/, a .aws/ credentials
// dir vendored into a project by mistake).
var denyDirNames = map[string]bool{
	".ssh":    true,
	".aws":    true,
	".gnupg":  true,
	".config": true,
}

func isDenied(relPath string) (bool, string) {
	base := filepath.Base(relPath)
	if denyBasenames[base] {
		return true, fmt.Sprintf("%q is on the deny-list", base)
	}
	if strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") {
		return true, fmt.Sprintf("%q looks like a private key/credential file", base)
	}
	if strings.Contains(base, "id_rsa") || strings.Contains(base, "id_ed25519") {
		return true, fmt.Sprintf("%q looks like an SSH private key", base)
	}
	for _, part := range strings.Split(filepath.ToSlash(relPath), "/") {
		if denyDirNames[part] {
			return true, fmt.Sprintf("path component %q is on the deny-list", part)
		}
	}
	return false, ""
}

// Resolve validates relPath (as given by the model -- untrusted) and
// returns the real, absolute path to use for I/O. It never follows a
// symlink out of the sandbox: every existing ancestor directory is
// resolved and checked, so a symlink planted inside the project that
// points outside it is caught even though the symlink *file* itself is
// inside the root. relPath need not exist yet (for write_file
// creating a new file), but every directory component that does exist
// is checked.
func (s *Sandbox) Resolve(relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("absolute paths are not allowed: %q", relPath)
	}
	clean := filepath.Clean(relPath)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the project root: %q", relPath)
	}
	if denied, reason := isDenied(clean); denied {
		return "", fmt.Errorf("refused: %s", reason)
	}

	joined := filepath.Join(s.Root, clean)
	if joined != s.Root && !strings.HasPrefix(joined, s.Root+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the project root: %q", relPath)
	}

	resolvedParent, err := resolveExistingAncestor(joined)
	if err != nil {
		return "", err
	}
	if resolvedParent != s.Root && !strings.HasPrefix(resolvedParent, s.Root+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the project root via a symlink: %q", relPath)
	}

	return joined, nil
}

// resolveExistingAncestor walks up from path until it finds a
// directory that actually exists, resolves *that* directory's
// symlinks, and returns the result -- the check that matters for
// containment, since the final path component (a not-yet-created
// file) has nothing to resolve.
func resolveExistingAncestor(path string) (string, error) {
	cur := path
	for {
		if _, err := os.Lstat(cur); err == nil {
			return filepath.EvalSymlinks(cur)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("no existing ancestor directory found for %q", path)
		}
		cur = parent
	}
}

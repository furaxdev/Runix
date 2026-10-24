package codeagent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveAllowsAnOrdinaryPathInsideTheProject(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	s, err := NewSandbox(root)
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	got, err := s.Resolve("src/main.go")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := filepath.Join(root, "src", "main.go")
	if got != want {
		t.Errorf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveRejectsAbsolutePath(t *testing.T) {
	s, _ := NewSandbox(t.TempDir())
	if _, err := s.Resolve("/etc/passwd"); err == nil {
		t.Error("expected an absolute path to be rejected")
	}
}

func TestResolveRejectsDotDotTraversal(t *testing.T) {
	s, _ := NewSandbox(t.TempDir())
	cases := []string{"../outside.txt", "a/../../outside.txt", "a/b/../../../outside.txt"}
	for _, c := range cases {
		if _, err := s.Resolve(c); err == nil {
			t.Errorf("Resolve(%q): expected a traversal escape to be rejected", c)
		}
	}
}

func TestResolveRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevated privileges on Windows")
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("sensitive"), 0o644)

	root := t.TempDir()
	// A symlinked directory *inside* the project that actually points
	// outside it -- the attack this test guards against.
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	s, err := NewSandbox(root)
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	if _, err := s.Resolve("escape/secret.txt"); err == nil {
		t.Error("expected a path through a symlink pointing outside the project to be rejected")
	}
}

func TestResolveAllowsSymlinkThatStaysInside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevated privileges on Windows")
	}
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "real"), 0o755)
	os.WriteFile(filepath.Join(root, "real", "file.txt"), []byte("ok"), 0o644)
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "alias")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	s, _ := NewSandbox(root)
	if _, err := s.Resolve("alias/file.txt"); err != nil {
		t.Errorf("expected a symlink that stays inside the project to be allowed, got: %v", err)
	}
}

func TestResolveAllowsWritingANewFileWhoseParentExists(t *testing.T) {
	root := t.TempDir()
	s, _ := NewSandbox(root)
	// new-file.txt doesn't exist yet -- write_file's whole purpose.
	if _, err := s.Resolve("new-file.txt"); err != nil {
		t.Errorf("expected a not-yet-existing file in an existing dir to resolve, got: %v", err)
	}
}

func TestResolveRejectsDenylistedFilenames(t *testing.T) {
	s, _ := NewSandbox(t.TempDir())
	cases := []string{".env", "credentials.json", "kaggle.json", "id_rsa", "foo/id_ed25519", "server.pem", "api.key"}
	for _, c := range cases {
		if _, err := s.Resolve(c); err == nil {
			t.Errorf("Resolve(%q): expected a deny-listed filename to be rejected", c)
		}
	}
}

func TestResolveRejectsDenylistedDirectories(t *testing.T) {
	s, _ := NewSandbox(t.TempDir())
	cases := []string{".ssh/config", ".aws/credentials", "nested/.ssh/known_hosts", ".gnupg/pubring.gpg"}
	for _, c := range cases {
		if _, err := s.Resolve(c); err == nil {
			t.Errorf("Resolve(%q): expected a deny-listed directory to be rejected", c)
		}
	}
}

func TestResolveAllowsOrdinaryDotfiles(t *testing.T) {
	// Only the specific deny-listed names/dirs are blocked -- an
	// ordinary project dotfile like .gitignore must still work.
	s, _ := NewSandbox(t.TempDir())
	if _, err := s.Resolve(".gitignore"); err != nil {
		t.Errorf("expected .gitignore to be allowed, got: %v", err)
	}
}

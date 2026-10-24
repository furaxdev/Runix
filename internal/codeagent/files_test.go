package codeagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/furaxdev/runyx/internal/approval"
)

func newTestTools(t *testing.T) (*Tools, string) {
	t.Helper()
	root := t.TempDir()
	tools, err := NewTools(root, approval.NewQueue())
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	return tools, root
}

func TestReadFileReturnsFullContent(t *testing.T) {
	tools, root := newTestTools(t)
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("one\ntwo\nthree\n"), 0o644)
	got, err := tools.ReadFile("f.txt", 0, 0)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got != "one\ntwo\nthree\n" {
		t.Errorf("got %q", got)
	}
}

func TestReadFileRespectsLineRange(t *testing.T) {
	tools, root := newTestTools(t)
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("one\ntwo\nthree\n"), 0o644)
	got, err := tools.ReadFile("f.txt", 2, 2)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got != "two" {
		t.Errorf("got %q, want %q", got, "two")
	}
}

func TestReadFileRefusesPathOutsideSandbox(t *testing.T) {
	tools, _ := newTestTools(t)
	if _, err := tools.ReadFile("../outside.txt", 0, 0); err == nil {
		t.Error("expected a path outside the sandbox to be refused")
	}
}

func TestListDirListsAndSortsEntries(t *testing.T) {
	tools, root := newTestTools(t)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("xx"), 0o644)
	os.Mkdir(filepath.Join(root, "sub"), 0o755)
	entries, err := tools.ListDir(".")
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	if entries[0].Name != "a.txt" || entries[1].Name != "b.txt" || entries[2].Name != "sub" {
		t.Errorf("entries not sorted: %+v", entries)
	}
	if !entries[2].IsDir {
		t.Error("expected sub to be reported as a directory")
	}
}

func TestSearchFindsMatchingLines(t *testing.T) {
	tools, root := newTestTools(t)
	os.WriteFile(filepath.Join(root, "f.go"), []byte("package main\nfunc Foo() {}\nfunc Bar() {}\n"), 0o644)
	matches, err := tools.Search("func (Foo|Bar)", ".", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2: %+v", len(matches), matches)
	}
}

func TestSearchSkipsDenylistedFilesAndDirs(t *testing.T) {
	tools, root := newTestTools(t)
	os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=matchme"), 0o644)
	os.Mkdir(filepath.Join(root, ".ssh"), 0o755)
	os.WriteFile(filepath.Join(root, ".ssh", "config"), []byte("matchme"), 0o644)
	matches, err := tools.Search("matchme", ".", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected deny-listed paths to be excluded from search, got %+v", matches)
	}
}

func TestWriteFileRefusedWithoutApproval(t *testing.T) {
	root := t.TempDir()
	q := approval.NewQueue()
	tools, err := NewTools(root, q)
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // never approved: ctx already done
	if err := tools.WriteFile(ctx, "new.txt", "hello", "test"); err == nil {
		t.Error("expected write_file to be refused when nobody approves it")
	}
	if _, err := os.Stat(filepath.Join(root, "new.txt")); err == nil {
		t.Error("file must not exist after a refused write")
	}
}

func TestWriteFileSucceedsWhenApproved(t *testing.T) {
	root := t.TempDir()
	q := approval.NewQueue()
	tools, err := NewTools(root, q)
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	go approveNextRequest(t, q)
	if err := tools.WriteFile(context.Background(), "new.txt", "hello", "test"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "new.txt"))
	if err != nil || string(data) != "hello" {
		t.Errorf("file content = %q, %v", data, err)
	}
}

func TestWriteFileRefusesPathOutsideSandbox(t *testing.T) {
	tools, _ := newTestTools(t)
	if err := tools.WriteFile(context.Background(), "../outside.txt", "x", "test"); err == nil {
		t.Error("expected write_file outside the sandbox to be refused before any approval is requested")
	}
}

func TestWriteFileFailsClosedWithNoApprovalQueue(t *testing.T) {
	root := t.TempDir()
	tools, err := NewTools(root, nil)
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	if err := tools.WriteFile(context.Background(), "new.txt", "x", "test"); err == nil {
		t.Error("expected write_file with no approval queue configured to fail closed")
	}
}

func TestEditFileAppliesApprovedDiff(t *testing.T) {
	root := t.TempDir()
	q := approval.NewQueue()
	tools, err := NewTools(root, q)
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("one\ntwo\nthree\n"), 0o644)
	diff := "@@ -1,3 +1,3 @@\n one\n-two\n+TWO\n three\n"
	go approveNextRequest(t, q)
	if err := tools.EditFile(context.Background(), "f.txt", diff, "test"); err != nil {
		t.Fatalf("EditFile: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "f.txt"))
	if string(data) != "one\nTWO\nthree\n" {
		t.Errorf("got %q", data)
	}
}

func TestEditFileRefusesStaleDiffBeforeRequestingApproval(t *testing.T) {
	root := t.TempDir()
	q := approval.NewQueue()
	tools, err := NewTools(root, q)
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("changed since the diff was made\n"), 0o644)
	diff := "@@ -1,1 +1,1 @@\n-one\n+ONE\n"
	// No approver is running -- if EditFile requested approval anyway,
	// this test would hang/timeout instead of failing fast.
	if err := tools.EditFile(context.Background(), "f.txt", diff, "test"); err == nil {
		t.Error("expected a stale diff to be refused without ever reaching approval")
	}
}

func TestDeleteRemovesFileWhenApproved(t *testing.T) {
	root := t.TempDir()
	q := approval.NewQueue()
	tools, err := NewTools(root, q)
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	target := filepath.Join(root, "f.txt")
	os.WriteFile(target, []byte("bye"), 0o644)
	go approveNextRequest(t, q)
	if err := tools.Delete(context.Background(), "f.txt", "test"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("expected the file to be gone after an approved delete")
	}
}

func TestDeleteRefusedWithoutApprovalLeavesFileIntact(t *testing.T) {
	root := t.TempDir()
	q := approval.NewQueue()
	tools, err := NewTools(root, q)
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	target := filepath.Join(root, "f.txt")
	os.WriteFile(target, []byte("stays"), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := tools.Delete(ctx, "f.txt", "test"); err == nil {
		t.Error("expected delete to be refused when nobody approves it")
	}
	if _, err := os.Stat(target); err != nil {
		t.Error("file must still exist after a refused delete")
	}
}

// approveNextRequest polls the queue for the next pending request and
// approves it, for tests exercising the real async Submit/Resolve
// path rather than stubbing approval out.
func approveNextRequest(t *testing.T, q *approval.Queue) {
	t.Helper()
	for i := 0; i < 200; i++ {
		pending := q.List()
		if len(pending) > 0 {
			q.Resolve(pending[0].ID, pending[0].Hash, approval.Decision{Allow: true, Scope: approval.ScopeOnce})
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Error("no pending approval request appeared in time")
}

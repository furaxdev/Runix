package memory

import (
	"strings"
	"testing"
)

func TestAddThenEntriesRoundTrips(t *testing.T) {
	s := New(t.TempDir(), "p1")
	if err := s.Add(ScopeMemory, "", "tests run with pnpm test", "user said so"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	entries, err := s.Entries(ScopeMemory, "")
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(entries) != 1 || entries[0] != "tests run with pnpm test" {
		t.Errorf("entries = %v", entries)
	}
}

func TestEntriesOnMissingFileIsEmptyNotError(t *testing.T) {
	s := New(t.TempDir(), "p1")
	entries, err := s.Entries(ScopeUser, "")
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no entries for a file that doesn't exist yet, got %v", entries)
	}
}

func TestEntrySeparatorSplitsCorrectly(t *testing.T) {
	s := New(t.TempDir(), "p1")
	s.Add(ScopeMemory, "", "first fact", "x")
	s.Add(ScopeMemory, "", "second fact", "x")
	s.Add(ScopeMemory, "", "third fact", "x")

	raw, err := s.RenderForContext(ScopeMemory, "")
	if err != nil {
		t.Fatalf("RenderForContext: %v", err)
	}
	if !strings.Contains(raw, "\n"+EntrySep+"\n") {
		t.Errorf("expected entries separated by a line containing %q, got:\n%s", EntrySep, raw)
	}

	entries, _ := s.Entries(ScopeMemory, "")
	if len(entries) != 3 {
		t.Fatalf("entries = %v, want 3", entries)
	}
	if entries[0] != "first fact" || entries[1] != "second fact" || entries[2] != "third fact" {
		t.Errorf("entries = %v", entries)
	}
}

func TestCapIsEnforced(t *testing.T) {
	s := New(t.TempDir(), "p1").WithCap(50)
	if err := s.Add(ScopeMemory, "", strings.Repeat("a", 30), "x"); err != nil {
		t.Fatalf("first Add (under cap): %v", err)
	}
	err := s.Add(ScopeMemory, "", strings.Repeat("b", 30), "x")
	if err != ErrFull {
		t.Fatalf("second Add (would exceed cap) = %v, want ErrFull", err)
	}
	// The refused entry must never have been written.
	entries, _ := s.Entries(ScopeMemory, "")
	if len(entries) != 1 {
		t.Errorf("entries after refused Add = %v, want exactly the first one", entries)
	}
}

func TestFullFileRequiresRemoveOrReplaceBeforeAdding(t *testing.T) {
	s := New(t.TempDir(), "p1").WithCap(35)
	s.Add(ScopeMemory, "", strings.Repeat("a", 30), "x")
	if err := s.Add(ScopeMemory, "", "more", "x"); err != ErrFull {
		t.Fatalf("Add on full file = %v, want ErrFull", err)
	}
	// Removing the stale entry must free enough room.
	if err := s.Remove(ScopeMemory, "", 0, "stale"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := s.Add(ScopeMemory, "", "fits now", "x"); err != nil {
		t.Fatalf("Add after Remove: %v", err)
	}
}

func TestReplaceCompressesEntries(t *testing.T) {
	s := New(t.TempDir(), "p1")
	s.Add(ScopeMemory, "", "decision: use TOML", "x")
	s.Add(ScopeMemory, "", "decision: actually use YAML instead", "x")
	// Compress both into one entry at index 0, remove the second.
	if err := s.Replace(ScopeMemory, "", 0, "decision: use YAML (superseded TOML choice)", "compress"); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if err := s.Remove(ScopeMemory, "", 1, "folded into entry 0"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	entries, _ := s.Entries(ScopeMemory, "")
	if len(entries) != 1 || entries[0] != "decision: use YAML (superseded TOML choice)" {
		t.Errorf("entries = %v", entries)
	}
}

func TestEveryModificationIsAudited(t *testing.T) {
	s := New(t.TempDir(), "p1")
	s.Add(ScopeMemory, "", "fact one", "user said so")
	s.Replace(ScopeMemory, "", 0, "fact one, revised", "correction")
	s.Remove(ScopeMemory, "", 0, "no longer true")

	audit, err := s.ReadAudit()
	if err != nil {
		t.Fatalf("ReadAudit: %v", err)
	}
	if len(audit) != 3 {
		t.Fatalf("audit entries = %d, want 3", len(audit))
	}
	if audit[0].Action != "add" || audit[1].Action != "replace" || audit[2].Action != "remove" {
		t.Errorf("audit actions = %v", []string{audit[0].Action, audit[1].Action, audit[2].Action})
	}
	if audit[1].Before != "fact one" || audit[1].After != "fact one, revised" {
		t.Errorf("replace audit before/after = %q / %q", audit[1].Before, audit[1].After)
	}
	if audit[2].Before != "fact one, revised" {
		t.Errorf("remove audit before = %q", audit[2].Before)
	}
}

func TestRefusedWriteIsStillAudited(t *testing.T) {
	s := New(t.TempDir(), "p1").WithCap(20)
	s.Add(ScopeMemory, "", strings.Repeat("a", 15), "x")
	if err := s.Add(ScopeMemory, "", "this will not fit", "y"); err != ErrFull {
		t.Fatalf("expected ErrFull, got %v", err)
	}
	audit, _ := s.ReadAudit()
	found := false
	for _, e := range audit {
		if e.Action == "refused" {
			found = true
		}
	}
	if !found {
		t.Error("expected a refused write to still be logged to the audit trail")
	}
}

func TestSecretsAreRedactedBeforeWrite(t *testing.T) {
	s := New(t.TempDir(), "p1")
	secret := "the github token is ghp_1234567890abcdefghij1234567890abcdef"
	if err := s.Add(ScopeMemory, "", secret, "x"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	raw, _ := s.RenderForContext(ScopeMemory, "")
	if strings.Contains(raw, "ghp_1234567890abcdefghij1234567890abcdef") {
		t.Error("raw secret present in the written memory file")
	}
	if !strings.Contains(raw, "[redacted:") {
		t.Errorf("expected a redaction marker in the written file, got: %s", raw)
	}

	audit, _ := s.ReadAudit()
	for _, e := range audit {
		if strings.Contains(e.After, "ghp_1234567890abcdefghij1234567890abcdef") {
			t.Error("raw secret present in the audit log")
		}
	}
}

func TestUserMemoryAndProjectMemoryAreSeparateFiles(t *testing.T) {
	s := New(t.TempDir(), "p1")
	s.Add(ScopeUser, "", "prefers tabs over spaces", "x")
	s.Add(ScopeMemory, "", "general convention: use feature branches", "x")
	s.Add(ScopeProject, "furaxdev/runyx", "this repo's tests = go test ./...", "x")

	userEntries, _ := s.Entries(ScopeUser, "")
	memEntries, _ := s.Entries(ScopeMemory, "")
	projEntries, _ := s.Entries(ScopeProject, "furaxdev/runyx")

	if len(userEntries) != 1 || len(memEntries) != 1 || len(projEntries) != 1 {
		t.Fatalf("expected exactly one entry each, got user=%v memory=%v project=%v", userEntries, memEntries, projEntries)
	}

	otherProject, _ := s.Entries(ScopeProject, "someone/other-repo")
	if len(otherProject) != 0 {
		t.Error("expected a different project's memory to be empty, got entries from furaxdev/runyx's file")
	}
}

func TestProjectScopeRequiresAnIdentifier(t *testing.T) {
	s := New(t.TempDir(), "p1")
	if err := s.Add(ScopeProject, "", "fact", "x"); err == nil {
		t.Error("expected an error adding a project-scoped memory with no project identifier")
	}
}

func TestReplaceAndRemoveRejectOutOfRangeIndex(t *testing.T) {
	s := New(t.TempDir(), "p1")
	s.Add(ScopeMemory, "", "only entry", "x")
	if err := s.Replace(ScopeMemory, "", 5, "x", "x"); err == nil {
		t.Error("expected Replace to reject an out-of-range index")
	}
	if err := s.Remove(ScopeMemory, "", -1, "x"); err == nil {
		t.Error("expected Remove to reject a negative index")
	}
}

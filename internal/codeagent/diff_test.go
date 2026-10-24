package codeagent

import "testing"

func TestApplyUnifiedDiffAppliesASimpleReplacement(t *testing.T) {
	original := "line1\nline2\nline3\n"
	diff := "--- a/file.txt\n+++ b/file.txt\n@@ -1,3 +1,3 @@\n line1\n-line2\n+line2 edited\n line3\n"
	got, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("applyUnifiedDiff: %v", err)
	}
	want := "line1\nline2 edited\nline3\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplyUnifiedDiffAppendsALine(t *testing.T) {
	original := "a\nb\n"
	diff := "@@ -1,2 +1,3 @@\n a\n b\n+c\n"
	got, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("applyUnifiedDiff: %v", err)
	}
	if got != "a\nb\nc\n" {
		t.Errorf("got %q", got)
	}
}

func TestApplyUnifiedDiffRemovesALine(t *testing.T) {
	original := "a\nb\nc\n"
	diff := "@@ -1,3 +1,2 @@\n a\n-b\n c\n"
	got, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("applyUnifiedDiff: %v", err)
	}
	if got != "a\nc\n" {
		t.Errorf("got %q", got)
	}
}

func TestApplyUnifiedDiffRefusesStaleContext(t *testing.T) {
	original := "line1\nCHANGED ALREADY\nline3\n"
	diff := "@@ -1,3 +1,3 @@\n line1\n-line2\n+line2 edited\n line3\n"
	if _, err := applyUnifiedDiff(original, diff); err == nil {
		t.Error("expected a stale diff (context mismatch) to be refused")
	}
}

func TestApplyUnifiedDiffRefusesHunkPastEndOfFile(t *testing.T) {
	original := "only one line\n"
	diff := "@@ -10,1 +10,1 @@\n-only one line\n+edited\n"
	if _, err := applyUnifiedDiff(original, diff); err == nil {
		t.Error("expected a hunk addressing a nonexistent line to be refused")
	}
}

func TestApplyUnifiedDiffRefusesEmptyDiff(t *testing.T) {
	if _, err := applyUnifiedDiff("a\nb\n", ""); err == nil {
		t.Error("expected an empty diff (no hunks) to be refused")
	}
}

func TestApplyUnifiedDiffHandlesMultipleHunks(t *testing.T) {
	original := "1\n2\n3\n4\n5\n6\n"
	diff := "@@ -1,2 +1,2 @@\n-1\n+one\n 2\n@@ -5,2 +5,2 @@\n 5\n-6\n+six\n"
	got, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("applyUnifiedDiff: %v", err)
	}
	if got != "one\n2\n3\n4\n5\nsix\n" {
		t.Errorf("got %q", got)
	}
}

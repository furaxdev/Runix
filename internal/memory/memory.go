// Package memory implements Runyx Code's level-1 memory: two small,
// capped Markdown files (USER.md, MEMORY.md, plus a per-project
// MEMORY.md) copied into the model's context at the start of every
// turn. See docs/RUNYX_CODE.md section 11 for the full design; this
// package is deliberately just file-format logic -- it has no
// knowledge of models, sessions, or where a fact came from. Whether a
// fact is safe to write (not from untrusted content, no secrets) is
// decided partly here (secret redaction, always) and partly by the
// caller (provenance -- see internal/codeagent/tools/memory.go).
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/furaxdev/runyx/internal/secretscan"
)

// DefaultCapChars is the per-file character budget from the mission
// brief ("plafond strict d'environ 2 200 caractères chacun"). This is
// what makes the system tenable with small local models: the two
// files are copied into context on every turn, so they must stay
// small regardless of how much the agent has learned over time --
// anything beyond the cap has to earn its place via Skills (level 3)
// or be found again via session_search (level 2) instead.
const DefaultCapChars = 2200

// EntrySep separates entries within a memory file -- a line
// containing only "§", per the mission brief. Chosen because it's
// vanishingly unlikely to appear in real prose/code, unlike blank
// lines or "---".
const EntrySep = "§"

var ErrFull = fmt.Errorf("memory file is at its %d-character cap; delete or compress an entry before adding another", DefaultCapChars)

// Scope selects which file a memory operation targets.
type Scope int

const (
	ScopeUser Scope = iota
	ScopeMemory
	ScopeProject
)

// AuditEntry is one line of audit.jsonl -- append-only, every write
// logged regardless of outcome (including a refused one), matching
// the audit discipline internal/agent already uses for commands.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Scope     string    `json:"scope"`
	Project   string    `json:"project,omitempty"`
	Action    string    `json:"action"` // "add" | "replace" | "remove" | "refused"
	Reason    string    `json:"reason,omitempty"`
	Before    string    `json:"before,omitempty"`
	After     string    `json:"after,omitempty"`
}

// Store manages one profile's memory files under dataDir (the app
// data dir -- NOT the project repo; see docs/RUNYX_CODE.md section
// 11: "Dossier de données de l'application (pas dans le dépôt du
// projet)"). capChars is overridable for tests; zero means
// DefaultCapChars.
type Store struct {
	dataDir  string
	profile  string
	capChars int
}

func New(dataDir, profile string) *Store {
	if profile == "" {
		profile = "default"
	}
	return &Store{dataDir: dataDir, profile: profile, capChars: DefaultCapChars}
}

// WithCap returns a copy of the store using a different character
// cap -- for tests that need to fill a file without pasting 2200
// characters of filler.
func (s *Store) WithCap(capChars int) *Store {
	cp := *s
	cp.capChars = capChars
	return &cp
}

func (s *Store) memoriesDir() string {
	return filepath.Join(s.dataDir, "profiles", s.profile, "memories")
}

func (s *Store) path(scope Scope, project string) (string, error) {
	switch scope {
	case ScopeUser:
		return filepath.Join(s.memoriesDir(), "USER.md"), nil
	case ScopeMemory:
		return filepath.Join(s.memoriesDir(), "MEMORY.md"), nil
	case ScopeProject:
		if project == "" {
			return "", fmt.Errorf("project scope requires a non-empty project identifier")
		}
		return filepath.Join(s.memoriesDir(), "projects", sanitizeProject(project), "MEMORY.md"), nil
	default:
		return "", fmt.Errorf("unknown memory scope %d", scope)
	}
}

func sanitizeProject(project string) string {
	return strings.NewReplacer("/", "_", "\\", "_", "..", "_").Replace(project)
}

// Entries reads and splits a memory file into its individual facts,
// in file order. A missing file is not an error -- it just has no
// entries yet.
func (s *Store) Entries(scope Scope, project string) ([]string, error) {
	path, err := s.path(scope, project)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return splitEntries(string(data)), nil
}

func splitEntries(content string) []string {
	raw := strings.Split(content, "\n"+EntrySep+"\n")
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		e = strings.TrimSpace(e)
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

func joinEntries(entries []string) string {
	return strings.Join(entries, "\n"+EntrySep+"\n")
}

// RenderForContext returns the file's current content verbatim, for
// copying into the model's context at the start of a turn -- always
// within the cap by construction, since Add/Replace refuse to exceed
// it.
func (s *Store) RenderForContext(scope Scope, project string) (string, error) {
	path, err := s.path(scope, project)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Add appends a new fact. Secrets are redacted before anything is
// written or logged. Returns ErrFull if the file is already at its
// cap -- the caller (the agent loop, per the mission's "the agent
// must choose: delete a stale entry, or merge/compress, before
// adding") is responsible for freeing space first via Remove/Replace,
// never by silently truncating.
func (s *Store) Add(scope Scope, project, fact, reason string) error {
	fact, _ = secretscan.Redact(fact)
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return fmt.Errorf("refusing to add an empty memory entry")
	}

	entries, err := s.Entries(scope, project)
	if err != nil {
		return err
	}
	candidate := joinEntries(append(entries, fact))
	if len(candidate) > s.capFor() {
		s.audit(scope, project, "refused", reason, "", fact)
		return ErrFull
	}

	if err := s.write(scope, project, candidate); err != nil {
		return err
	}
	s.audit(scope, project, "add", reason, "", fact)
	return nil
}

// Replace overwrites the entry at index (0-based, in file order) --
// used both for a plain edit and for "merge several entries into one
// compressed entry" (caller computes the merged text, then Replace's
// the first of the merged entries while Removing the rest, or more
// simply: callers compress by calling Replace once with the new
// combined text and Remove for every other entry being folded in).
func (s *Store) Replace(scope Scope, project string, index int, newText, reason string) error {
	newText, _ = secretscan.Redact(newText)
	newText = strings.TrimSpace(newText)
	if newText == "" {
		return fmt.Errorf("refusing to replace an entry with empty text (use Remove instead)")
	}

	entries, err := s.Entries(scope, project)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(entries) {
		return fmt.Errorf("entry index %d out of range (0..%d)", index, len(entries)-1)
	}
	before := entries[index]
	updated := append([]string{}, entries...)
	updated[index] = newText
	candidate := joinEntries(updated)
	if len(candidate) > s.capFor() {
		s.audit(scope, project, "refused", reason, before, newText)
		return ErrFull
	}
	if err := s.write(scope, project, candidate); err != nil {
		return err
	}
	s.audit(scope, project, "replace", reason, before, newText)
	return nil
}

// Remove deletes the entry at index.
func (s *Store) Remove(scope Scope, project string, index int, reason string) error {
	entries, err := s.Entries(scope, project)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(entries) {
		return fmt.Errorf("entry index %d out of range (0..%d)", index, len(entries)-1)
	}
	before := entries[index]
	updated := append(append([]string{}, entries[:index]...), entries[index+1:]...)
	if err := s.write(scope, project, joinEntries(updated)); err != nil {
		return err
	}
	s.audit(scope, project, "remove", reason, before, "")
	return nil
}

func (s *Store) capFor() int {
	if s.capChars > 0 {
		return s.capChars
	}
	return DefaultCapChars
}

func (s *Store) write(scope Scope, project, content string) error {
	path, err := s.path(scope, project)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func (s *Store) audit(scope Scope, project, action, reason, before, after string) {
	entry := AuditEntry{
		Timestamp: time.Now(),
		Scope:     scopeName(scope),
		Project:   project,
		Action:    action,
		Reason:    reason,
		Before:    before,
		After:     after,
	}
	appendAuditJSONL(s.memoriesDir(), entry)
}

func scopeName(scope Scope) string {
	switch scope {
	case ScopeUser:
		return "user"
	case ScopeMemory:
		return "memory"
	case ScopeProject:
		return "project"
	default:
		return "unknown"
	}
}

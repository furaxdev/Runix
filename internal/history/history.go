// Package history persists chat conversations in SQLite: messages,
// branches (editing a message forks the conversation), folders, pins,
// and full-text search. Implements FEATURES.md item 14.
package history

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Store wraps the history SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the history database under dataDir.
func Open(dataDir string) (*Store, error) {
	path := filepath.Join(dataDir, "runyx.db")
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open history db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate history db: %w", err)
	}
	return s, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS conversations (
	id         TEXT PRIMARY KEY,
	title      TEXT NOT NULL DEFAULT '',
	model      TEXT NOT NULL DEFAULT '',
	folder     TEXT NOT NULL DEFAULT '',
	pinned     INTEGER NOT NULL DEFAULT 0,
	project_id TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
	id              TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
	parent_id       TEXT NOT NULL DEFAULT '',
	branch_id       TEXT NOT NULL DEFAULT 'main',
	role            TEXT NOT NULL,
	content         TEXT NOT NULL,
	created_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, branch_id, created_at);

CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
	content, content='messages', content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
	INSERT INTO messages_fts(rowid, content) VALUES (new.rowid, new.content);
END;
CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
	INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
END;
CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
	INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
	INSERT INTO messages_fts(rowid, content) VALUES (new.rowid, new.content);
END;
`
	_, err := s.db.Exec(schema)
	return err
}

// Conversation is one chat thread (possibly with multiple branches).
type Conversation struct {
	ID        string
	Title     string
	Model     string
	Folder    string
	Pinned    bool
	ProjectID string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message is one turn within a conversation branch.
type Message struct {
	ID             string
	ConversationID string
	ParentID       string // empty for the first message in a branch
	BranchID       string
	Role           string
	Content        string
	CreatedAt      time.Time
}

func newID() string { return uuid.NewString() }

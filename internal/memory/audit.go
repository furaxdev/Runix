package memory

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

// appendAuditJSONL appends one JSON object per line to
// <memoriesDir>/audit.jsonl, matching internal/agent's audit log
// convention. Best-effort: a logging failure must never block the
// memory write it's recording (the write already succeeded by the
// time this is called).
func appendAuditJSONL(memoriesDir string, entry AuditEntry) {
	if err := os.MkdirAll(memoriesDir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(memoriesDir, "audit.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f.Write(append(data, '\n'))
}

// ReadAudit reads every logged entry for this profile, oldest first --
// for the "What I learned" UI and for tests.
func (s *Store) ReadAudit() ([]AuditEntry, error) {
	path := filepath.Join(s.memoriesDir(), "audit.jsonl")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []AuditEntry
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var e AuditEntry
		if err := dec.Decode(&e); err != nil {
			break
		}
		out = append(out, e)
	}
	return out, nil
}

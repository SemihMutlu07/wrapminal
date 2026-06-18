package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestParseContinueReadsSessions(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".continue", "sessions", "sess-1.json"),
		`{"title":"/tmp/project-gamma","history":[
			{"message":{"role":"user","content":"refactor parser"},"timestamp":"2026-02-11T10:00:00Z"},
			{"message":{"role":"assistant","content":"done"},"timestamp":"2026-02-11T10:00:05Z"},
			{"message":{"role":"user","content":"add a test"},"timestamp":"2026-02-11T10:01:00Z"}
		]}`)

	items, status := parseContinue(home)
	if status.State != "loaded" {
		t.Fatalf("expected Continue loaded, got %q (%s)", status.State, status.Message)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 user prompts, got %d", len(items))
	}
	if items[0].Project != "project-gamma" {
		t.Fatalf("expected project name from title, got %q", items[0].Project)
	}
}

func TestParseAiderReadsInputHistory(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".aider.input.history"),
		"# 2026-02-11 10:00:00.000000\n+first prompt\n# 2026-02-11 10:05:00.000000\n+second prompt\n")

	items, status := parseAider(home)
	if status.State != "loaded" {
		t.Fatalf("expected Aider loaded, got %q (%s)", status.State, status.Message)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(items))
	}
	if items[0].Timestamp.IsZero() {
		t.Fatalf("expected timestamp parsed from comment line")
	}
}

func TestParseOpenCodeReadsMessagesBestEffort(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	mkdirForTest(t, dbPath)
	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `CREATE TABLE messages (id INTEGER PRIMARY KEY, role TEXT, content TEXT, created_at TEXT)`)
	mustExec(t, db, `INSERT INTO messages (role, content, created_at) VALUES ('user','do a thing','2026-02-11T10:00:00Z')`)
	mustExec(t, db, `INSERT INTO messages (role, content, created_at) VALUES ('assistant','sure','2026-02-11T10:00:05Z')`)
	mustExec(t, db, `INSERT INTO messages (role, content, created_at) VALUES ('user','do another','2026-02-11T11:00:00Z')`)
	db.Close()

	items, status := parseOpenCode(home)
	if status.State != "loaded" {
		t.Fatalf("expected OpenCode loaded, got %q (%s)", status.State, status.Message)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 user messages, got %d", len(items))
	}
}

func TestParseOpenCodeFallsBackToDetectedOnUnknownSchema(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	mkdirForTest(t, dbPath)
	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// A table that does not look like messages and has no time/content columns.
	mustExec(t, db, `CREATE TABLE config (k TEXT, v TEXT)`)
	mustExec(t, db, `INSERT INTO config VALUES ('theme','dark')`)
	db.Close()

	items, status := parseOpenCode(home)
	if status.State != "detected" {
		t.Fatalf("expected graceful fallback to detected, got %q", status.State)
	}
	if len(items) != 0 {
		t.Fatalf("expected no fabricated interactions, got %d", len(items))
	}
}

func TestParseCursorCountsPromptsBestEffort(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, ".config", "Cursor", "User", "globalStorage", "state.vscdb")
	mkdirForTest(t, dbPath)
	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `CREATE TABLE ItemTable (key TEXT PRIMARY KEY, value BLOB)`)
	mustExec(t, db, `INSERT INTO ItemTable VALUES ('aiService.prompts','[{"text":"a"},{"text":"b"},{"text":"c"}]')`)
	db.Close()

	items, status := parseCursor(home)
	if status.State != "detected" {
		t.Fatalf("expected Cursor detected, got %q", status.State)
	}
	if status.Records != 3 {
		t.Fatalf("expected 3 prompt records, got %d", status.Records)
	}
	if len(items) != 0 {
		t.Fatalf("Cursor prompts have no timestamps, expected 0 dated interactions, got %d", len(items))
	}
}

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func mkdirForTest(t *testing.T, file string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

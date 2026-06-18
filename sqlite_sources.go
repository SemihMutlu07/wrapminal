package main

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// SQLite-backed sources (Cursor, OpenCode) are read best-effort: the on-disk
// schemas are not a documented/stable public format, so we read what we can and
// degrade gracefully. If a database is found but unreadable or unrecognized, the
// source stays "detected" with an honest message instead of inventing data.

// openReadOnlySQLite opens a database without taking a write lock, so it is safe
// to read while the owning app (Cursor/OpenCode) is running.
func openReadOnlySQLite(path string) (*sql.DB, error) {
	return sql.Open("sqlite", "file:"+path+"?mode=ro&immutable=1&_pragma=busy_timeout(2000)")
}

func sqliteTables(db *sql.DB) []string {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			names = append(names, n)
		}
	}
	return names
}

// parseCursor reads Cursor's state.vscdb key/value store. Cursor keeps prompt
// history under keys like "aiService.prompts" as a JSON array of {text,...}.
// Those entries carry no per-prompt timestamp, so we report a real record count
// (visible in source health) but do not feed undated rows into the timeline.
func parseCursor(home string) ([]Interaction, SourceStatus) {
	bases := []string{
		filepath.Join(home, ".config", "Cursor"),
		filepath.Join(home, "Library", "Application Support", "Cursor"),
		filepath.Join(home, "AppData", "Roaming", "Cursor"),
	}
	status := SourceStatus{
		ID:         "cursor",
		Name:       "Cursor",
		Path:       strings.Join(bases, ", "),
		State:      "missing",
		Confidence: "experimental",
		Message:    "No Cursor local storage found.",
	}

	var dbPath string
	baseFound := false
	for _, base := range bases {
		if fileExists(base) {
			baseFound = true
		}
		candidates := []string{filepath.Join(base, "User", "globalStorage", "state.vscdb")}
		if matches, _ := filepath.Glob(filepath.Join(base, "User", "workspaceStorage", "*", "state.vscdb")); matches != nil {
			candidates = append(candidates, matches...)
		}
		for _, c := range candidates {
			if fileExists(c) {
				dbPath = c
				break
			}
		}
		if dbPath != "" {
			break
		}
	}
	if dbPath == "" {
		if baseFound {
			status.State = "detected"
			status.Message = "Cursor storage detected, but no readable prompt database was found at the known paths."
		}
		return nil, status
	}

	status.Path = dbPath
	status.State = "detected"
	status.Message = "Cursor storage detected, but no recognizable prompt history was read."

	db, err := openReadOnlySQLite(dbPath)
	if err != nil {
		return nil, status
	}
	defer db.Close()

	count := cursorPromptCount(db)
	if count > 0 {
		status.Records = count
		status.Message = "Cursor prompt history detected (best-effort). Entries carry no per-prompt timestamps, so they are shown as a count but not mixed into dated stats."
	}
	return nil, status
}

func cursorPromptCount(db *sql.DB) int {
	keys := []string{"aiService.prompts", "aiService.generations"}
	total := 0
	for _, key := range keys {
		var blob []byte
		row := db.QueryRow("SELECT value FROM ItemTable WHERE key = ?", key)
		if err := row.Scan(&blob); err != nil {
			continue
		}
		var arr []json.RawMessage
		if json.Unmarshal(blob, &arr) == nil {
			total += len(arr)
		}
	}
	return total
}

// parseOpenCode reads OpenCode's SQLite database. The schema is not a stable
// public format, so we introspect: find a message-like table, identify a
// timestamp and content column, and build dated user prompts when possible. If
// only a row count is available, we report that; otherwise the source stays
// "detected".
func parseOpenCode(home string) ([]Interaction, SourceStatus) {
	paths := []string{
		filepath.Join(home, ".local", "share", "opencode", "opencode.db"),
		filepath.Join(home, "Library", "Application Support", "opencode", "opencode.db"),
		filepath.Join(home, "AppData", "Roaming", "opencode", "opencode.db"),
	}
	status := SourceStatus{
		ID:         "opencode",
		Name:       "OpenCode",
		Path:       strings.Join(paths, ", "),
		State:      "missing",
		Confidence: "experimental",
		Message:    "No OpenCode database found.",
	}

	var dbPath string
	for _, p := range paths {
		if fileExists(p) {
			dbPath = p
			break
		}
	}
	if dbPath == "" {
		return nil, status
	}

	status.Path = dbPath
	status.State = "detected"
	status.Message = "OpenCode database detected, but no recognizable message table was read."

	db, err := openReadOnlySQLite(dbPath)
	if err != nil {
		return nil, status
	}
	defer db.Close()

	var table string
	for _, t := range sqliteTables(db) {
		if strings.Contains(strings.ToLower(t), "message") {
			table = t
			break
		}
	}
	if table == "" {
		return nil, status
	}

	cols := sqliteColumns(db, table)
	timeCol := pickColumn(cols, "created_at", "createdat", "time", "timestamp", "ts", "created")
	contentCol := pickColumn(cols, "content", "text", "body", "message", "parts")
	roleCol := pickColumn(cols, "role", "type", "author")

	if timeCol == "" || contentCol == "" {
		// We can at least show a real count even without dated rows.
		if n := sqliteCount(db, table); n > 0 {
			status.Records = n
			status.Message = "OpenCode messages detected (best-effort count). Schema lacks a recognizable timestamp/content column, so rows are not mixed into dated stats."
		}
		return nil, status
	}

	items := openCodeMessages(db, table, timeCol, contentCol, roleCol, status)
	status.Records = len(items)
	if len(items) > 0 {
		status.State = "loaded"
		status.Message = "OpenCode prompts loaded (best-effort)."
	}
	return items, status
}

func openCodeMessages(db *sql.DB, table, timeCol, contentCol, roleCol string, status SourceStatus) []Interaction {
	selectCols := timeCol + ", " + contentCol
	if roleCol != "" {
		selectCols += ", " + roleCol
	}
	rows, err := db.Query("SELECT " + selectCols + " FROM " + quoteIdent(table))
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []Interaction
	for rows.Next() {
		var rawTime sql.NullString
		var content sql.NullString
		var role sql.NullString
		dest := []any{&rawTime, &content}
		if roleCol != "" {
			dest = append(dest, &role)
		}
		if rows.Scan(dest...) != nil {
			continue
		}
		if roleCol != "" && role.Valid && !strings.EqualFold(strings.TrimSpace(role.String), "user") {
			continue
		}
		ts := parseFlexibleTime(rawTime.String)
		if ts.IsZero() {
			continue
		}
		items = append(items, Interaction{
			Source:    status.Name,
			SourceID:  status.ID,
			Project:   "OpenCode",
			SessionID: fallbackSession("", status.ID, len(items)),
			Timestamp: ts,
			Chars:     len(content.String),
		})
	}
	return items
}

func sqliteColumns(db *sql.DB, table string) []string {
	rows, err := db.Query("PRAGMA table_info(" + quoteIdent(table) + ")")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name string
		var ctype sql.NullString
		var notnull int
		var dflt sql.NullString
		var pk int
		if rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk) == nil {
			cols = append(cols, name)
		}
	}
	return cols
}

func sqliteCount(db *sql.DB, table string) int {
	var n int
	if db.QueryRow("SELECT COUNT(*) FROM "+quoteIdent(table)).Scan(&n) != nil {
		return 0
	}
	return n
}

// pickColumn returns the first existing column (case-insensitive) from wants.
func pickColumn(cols []string, wants ...string) string {
	lower := make(map[string]string, len(cols))
	for _, c := range cols {
		lower[strings.ToLower(c)] = c
	}
	for _, w := range wants {
		if c, ok := lower[strings.ToLower(w)]; ok {
			return c
		}
	}
	return ""
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

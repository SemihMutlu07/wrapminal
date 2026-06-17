package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestAPIHandlersReturnJSON(t *testing.T) {
	home := seedHistory(t)
	t.Setenv("CC_LENS_HOME", home)

	tests := []struct {
		name string
		path string
		fn   http.HandlerFunc
	}{
		{name: "wrapped", path: "/api/wrapped", fn: handleWrapped},
		{name: "stats", path: "/api/stats", fn: handleStats},
		{name: "timeline", path: "/api/timeline", fn: handleTimeline},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			res := httptest.NewRecorder()

			tt.fn(res, req)

			if res.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
			}
			if got := res.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("expected application/json content type, got %q", got)
			}
			var payload any
			if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}
		})
	}
}

func seedHistory(t *testing.T) string {
	t.Helper()
	home := t.TempDir()

	writeFile(t, filepath.Join(home, ".claude", "history.jsonl"),
		`{"display":"fix bug","pastedContents":{},"timestamp":1770849392147,"project":"/tmp/project-alpha","sessionId":"claude-1"}`+"\n"+
			`{"display":"add tests","pastedContents":{"a":{"content":"long pasted content"}},"timestamp":1770935792147,"project":"/tmp/project-alpha","sessionId":"claude-1"}`+"\n")

	writeFile(t, filepath.Join(home, ".codex", "history.jsonl"),
		`{"session_id":"codex-1","text":"ship the dashboard","ts":1770883906}`+"\n")

	writeFile(t, filepath.Join(home, ".gemini", "antigravity-cli", "history.jsonl"),
		`{"display":"make a plan","timestamp":1779266023457,"workspace":"/tmp/project-beta"}`+"\n")

	writeFile(t, filepath.Join(home, ".local", "share", "opencode", "opencode.db"), "sqlite placeholder")
	writeFile(t, filepath.Join(home, ".config", "Cursor", "storage.json"), "{}")

	return home
}

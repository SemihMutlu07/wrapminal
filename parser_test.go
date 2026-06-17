package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildWrappedParsesLocalJSONHistories(t *testing.T) {
	home := seedHistory(t)
	t.Setenv("CC_LENS_HOME", home)

	wrapped, err := BuildWrapped()
	if err != nil {
		t.Fatalf("BuildWrapped returned error: %v", err)
	}

	if wrapped.Totals.Prompts != 4 {
		t.Fatalf("expected 4 prompts, got %d", wrapped.Totals.Prompts)
	}
	if wrapped.Totals.Sources != 3 {
		t.Fatalf("expected 3 loaded sources, got %d", wrapped.Totals.Sources)
	}
	if len(wrapped.Projects) != 3 {
		t.Fatalf("expected 3 project buckets, got %d", len(wrapped.Projects))
	}
	if stateFor(wrapped.Sources, "opencode") != "detected" {
		t.Fatalf("expected OpenCode to be detected")
	}
	if stateFor(wrapped.Sources, "cursor") != "detected" {
		t.Fatalf("expected Cursor to be detected")
	}
	if len(wrapped.Highlights) == 0 {
		t.Fatalf("expected highlights")
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func stateFor(sources []SourceStatus, id string) string {
	for _, source := range sources {
		if source.ID == id {
			return source.State
		}
	}
	return ""
}

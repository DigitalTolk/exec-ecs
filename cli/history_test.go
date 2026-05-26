package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func setHistoryFile(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "history")
	prev := historyFile
	historyFile = path
	t.Cleanup(func() { historyFile = prev })
	return path
}

func TestAppendAndReadHistory(t *testing.T) {
	setHistoryFile(t)

	AppendToHistory("one")
	AppendToHistory("two")
	AppendToHistory("one")
	AppendToHistory("three")

	got := GetLastUniqueHistory(5)
	want := []string{"three", "one", "two"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("history = %v want %v", got, want)
	}
}

func TestGetLastUniqueHistoryLimit(t *testing.T) {
	setHistoryFile(t)

	for _, cmd := range []string{"a", "b", "c", "d", "e", "f"} {
		AppendToHistory(cmd)
	}

	got := GetLastUniqueHistory(3)
	if len(got) != 3 {
		t.Fatalf("expected 3, got %v", got)
	}
	if !reflect.DeepEqual(got, []string{"f", "e", "d"}) {
		t.Fatalf("history = %v", got)
	}
}

func TestGetLastUniqueHistoryMissingFile(t *testing.T) {
	tmp := t.TempDir()
	prev := historyFile
	historyFile = filepath.Join(tmp, "absent")
	t.Cleanup(func() { historyFile = prev })

	if got := GetLastUniqueHistory(5); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestGetLastUniqueHistoryIgnoresBlanks(t *testing.T) {
	path := setHistoryFile(t)
	if err := os.WriteFile(path, []byte("alpha\n\n\nalpha\nbeta\n\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := GetLastUniqueHistory(5)
	want := []string{"beta", "alpha"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAppendHistoryUnwritable(t *testing.T) {
	prev := historyFile
	historyFile = "/proc/should-not-be-writable/exec-ecs-test"
	t.Cleanup(func() { historyFile = prev })

	// Should not panic, just fail silently.
	AppendToHistory("anything")
}

func TestMmWasMouseClick(t *testing.T) {
	t.Parallel()
	if mmWasMouseClick(menuModel{}) {
		t.Fatal("default menuModel should report no click")
	}
	if !mmWasMouseClick(menuModel{mouseClicked: true}) {
		t.Fatal("mouseClicked=true should report click")
	}
}

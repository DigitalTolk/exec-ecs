package cli

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestTruncateForDisplay(t *testing.T) {
	t.Parallel()
	if got := truncateForDisplay("short cmd"); got != "short cmd" {
		t.Fatalf("got %q", got)
	}
	// Internal newlines/whitespace collapse to single spaces.
	in := "aws  ecs\nexecute-command --cluster foo"
	want := "aws ecs execute-command --cluster foo"
	if got := truncateForDisplay(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// Over-limit string ends with the ellipsis and is exactly the cap.
	long := strings.Repeat("x", 200)
	got := truncateForDisplay(long)
	if len([]rune(got)) != historyDisplayMaxRune {
		t.Fatalf("len(got)=%d want %d", len([]rune(got)), historyDisplayMaxRune)
	}
	if !strings.HasSuffix(got, historyDisplayEllipsis) {
		t.Fatalf("missing ellipsis: %q", got)
	}
}

func TestCliBubbleteaHistorySelectDelegates(t *testing.T) {
	setHistoryFile(t)
	in := newScriptedKeys('\r')
	defer in.Close()
	prev := historyExtraOpts
	historyExtraOpts = []tea.ProgramOption{tea.WithInput(in), tea.WithOutput(io.Discard)}
	t.Cleanup(func() { historyExtraOpts = prev })

	c := &Cli{}
	got, err := c.BubbleteaHistorySelect("History", []string{"aws ecs exec foo"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "aws ecs exec foo" {
		t.Fatalf("got %q", got)
	}
}

func TestBubbleteaHistorySelectPicksFirst(t *testing.T) {
	setHistoryFile(t)
	in := newScriptedKeys('\r')
	defer in.Close()
	prev := historyExtraOpts
	historyExtraOpts = []tea.ProgramOption{tea.WithInput(in), tea.WithOutput(io.Discard)}
	t.Cleanup(func() { historyExtraOpts = prev })

	got, err := BubbleteaHistorySelect("History", []string{"aws ecs ... one", "aws ecs ... two"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "aws ecs ... one" {
		t.Fatalf("expected first item, got %q", got)
	}
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

package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// drainOutput is a synthetic stdout that swallows whatever the bubbletea
// renderer emits. Returning a non-nil ReadFrom keeps the renderer happy on
// systems where it tries to detect terminal capabilities.
type drainOutput struct{}

func (drainOutput) Write(p []byte) (int, error) { return len(p), nil }
func (drainOutput) Read(p []byte) (int, error)  { return 0, io.EOF }
func (drainOutput) Close() error                { return nil }
func (drainOutput) Fd() uintptr                 { return 0 }

// scriptedKeys feeds a fixed byte sequence into the bubbletea program then
// blocks indefinitely so the program can finish processing the last key
// (otherwise EOF on input can race the Enter handler).
type scriptedKeys struct {
	keys []byte
	idx  int
	done chan struct{}
}

func newScriptedKeys(keys ...byte) *scriptedKeys {
	return &scriptedKeys{keys: keys, done: make(chan struct{})}
}

func (s *scriptedKeys) Read(p []byte) (int, error) {
	if s.idx < len(s.keys) {
		n := copy(p, s.keys[s.idx:])
		s.idx += n
		return n, nil
	}
	// Park indefinitely once the script is done — tea.Quit unblocks Run()
	// before this matters.
	<-s.done
	return 0, io.EOF
}

func (s *scriptedKeys) Close() error {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	return nil
}

func TestBubbleteaSelectEnterPicksFirst(t *testing.T) {
	in := newScriptedKeys('\r') // Enter
	defer in.Close()
	out := &bytes.Buffer{}

	type result struct {
		val    string
		goBack bool
		err    error
	}
	resCh := make(chan result, 1)
	go func() {
		val, goBack, err := bubbleteaSelect("Pick", []string{"alpha", "beta"}, "", false,
			tea.WithInput(in), tea.WithOutput(out))
		resCh <- result{val, goBack, err}
	}()

	select {
	case r := <-resCh:
		if r.err != nil {
			t.Fatalf("err: %v", r.err)
		}
		if r.val != "alpha" {
			t.Fatalf("expected alpha, got %q", r.val)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("bubbletea select did not return within 3s")
	}
	_ = out
}

func TestBubbleteaSelectCtrlCExits(t *testing.T) {
	in := newScriptedKeys(0x03) // Ctrl+C
	defer in.Close()
	out := &bytes.Buffer{}

	resCh := make(chan string, 1)
	go func() {
		val, _, _ := bubbleteaSelect("Pick", []string{"alpha"}, "", false,
			tea.WithInput(in), tea.WithOutput(out))
		resCh <- val
	}()
	select {
	case val := <-resCh:
		if val != "" {
			t.Fatalf("expected empty val on ctrl-c, got %q", val)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ctrl+c did not quit")
	}
}

func TestPromptSelectEnterPicksDefault(t *testing.T) {
	in := newScriptedKeys('\r')
	defer in.Close()
	prev := promptExtraOpts
	promptExtraOpts = []tea.ProgramOption{tea.WithInput(in), tea.WithOutput(&bytes.Buffer{})}
	t.Cleanup(func() { promptExtraOpts = prev })

	c := &Cli{}
	val, goBack := c.PromptSelect("Pick", []string{"alpha", "beta"}, "beta", true)
	if goBack {
		t.Fatal("did not expect goBack")
	}
	if val != "beta" {
		t.Fatalf("got %q want beta", val)
	}
}

func TestPromptWithDefaultDelegates(t *testing.T) {
	in := newScriptedKeys('\r')
	defer in.Close()
	prev := promptExtraOpts
	promptExtraOpts = []tea.ProgramOption{tea.WithInput(in), tea.WithOutput(&bytes.Buffer{})}
	t.Cleanup(func() { promptExtraOpts = prev })

	c := &Cli{}
	val, goBack := c.PromptWithDefault("Pick", "y", []string{"x", "y"}, true)
	if goBack {
		t.Fatal("did not expect goBack")
	}
	if val != "y" {
		t.Fatalf("default did not stick: %q", val)
	}
	_ = strings.TrimSpace
}

func TestBubbleteaSelectThemePreviewBranch(t *testing.T) {
	prev := CurrentTheme
	t.Cleanup(func() { CurrentTheme = prev })

	in := newScriptedKeys('\r')
	defer in.Close()
	val, _, err := bubbleteaSelect("Select Theme", GetThemeNames(), "Matrix", false,
		tea.WithInput(in), tea.WithOutput(&bytes.Buffer{}))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if val != "Matrix" {
		t.Fatalf("expected Matrix selected, got %q", val)
	}
}

func TestPromptSelectGoBack(t *testing.T) {
	// Ctrl+B (0x02) triggers go-back.
	in := newScriptedKeys(0x02)
	defer in.Close()
	prev := promptExtraOpts
	promptExtraOpts = []tea.ProgramOption{tea.WithInput(in), tea.WithOutput(&bytes.Buffer{})}
	t.Cleanup(func() { promptExtraOpts = prev })

	c := &Cli{}
	val, goBack := c.PromptSelect("Pick", []string{"a", "b"}, "", true)
	if !goBack {
		t.Fatalf("expected goBack, got val=%q goBack=%v", val, goBack)
	}
	if val != "" {
		t.Fatalf("expected empty val on goBack, got %q", val)
	}
}

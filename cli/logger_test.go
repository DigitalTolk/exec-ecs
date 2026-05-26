package cli

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
)

func TestLogAWSCommandRespectsDebugFlag(t *testing.T) {
	var buf bytes.Buffer
	prev := cmdLogger
	cmdLogger = log.New(&buf, "[T] ", 0)
	t.Cleanup(func() { cmdLogger = prev })

	c := &Cli{Debug: false}
	c.LogAWSCommand("sts", "get-caller-identity")
	if buf.Len() != 0 {
		t.Fatalf("expected no logging when Debug=false, got %q", buf.String())
	}

	c.Debug = true
	c.LogAWSCommand("sts", "get-caller-identity", "--profile", "foo")
	if !strings.Contains(buf.String(), "aws sts get-caller-identity --profile foo") {
		t.Fatalf("missing log line, got %q", buf.String())
	}
}

func TestLogUserFriendlyError(t *testing.T) {
	var buf bytes.Buffer
	var exitCode int
	prevW, prevExit := errorWriter, exitFn
	errorWriter = &buf
	exitFn = func(c int) { exitCode = c }
	t.Cleanup(func() {
		errorWriter = prevW
		exitFn = prevExit
	})

	c := &Cli{}
	c.LogUserFriendlyError("the message", errors.New("boom"), "do the fix", "/tmp/x", 42)

	if exitCode != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode)
	}
	out := buf.String()
	for _, want := range []string{"the message", "boom", "do the fix", "/tmp/x", "42"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}

	// Empty file path and zero line number should be omitted.
	buf.Reset()
	c.LogUserFriendlyError("oops", errors.New("err"), "fix", "", 0)
	out = buf.String()
	if strings.Contains(out, "File Path") || strings.Contains(out, "Line Number") {
		t.Fatalf("expected omitted fields, got:\n%s", out)
	}
}

// `os` import unused warning hack — keep until LogUserFriendlyErrorSubprocess
// goes away. Kept for backwards compatibility with the original test scaffold.
var _ = os.Getenv

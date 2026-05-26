package cli

import (
	"flag"
	"os"
	"testing"
)

func resetFlagsAndArgs(t *testing.T, args []string) {
	t.Helper()
	origArgs := os.Args
	origCmd := flag.CommandLine
	os.Args = args
	flag.CommandLine = flag.NewFlagSet(args[0], flag.ExitOnError)
	t.Cleanup(func() {
		os.Args = origArgs
		flag.CommandLine = origCmd
	})
}

func TestParseArgsDefaults(t *testing.T) {
	resetFlagsAndArgs(t, []string{"exec-ecs"})
	c := ParseArgs()
	if c.Command != "bash" {
		t.Fatalf("Command default = %q", c.Command)
	}
	if !c.Interactive {
		t.Fatal("Interactive default should be true")
	}
	if c.Debug {
		t.Fatal("Debug default should be false")
	}
}

func TestParseArgsOverrides(t *testing.T) {
	resetFlagsAndArgs(t, []string{
		"exec-ecs",
		"-debug",
		"-pr", "prof",
		"-rg", "us-east-1",
		"-cl", "cluster-arn",
		"-se", "svc-arn",
		"-tk", "task-arn",
		"-cn", "main",
		"-command", "sh",
		"-H",
	})
	c := ParseArgs()
	if !c.Debug {
		t.Fatal("Debug should be true")
	}
	if c.Profile != "prof" || c.Region != "us-east-1" || c.ClusterArn != "cluster-arn" {
		t.Fatalf("flags not applied: %+v", c)
	}
	if c.Service != "svc-arn" || c.TaskArn != "task-arn" || c.Container != "main" {
		t.Fatalf("flags not applied: %+v", c)
	}
	if c.Command != "sh" || !c.History {
		t.Fatalf("Command/History wrong: %+v", c)
	}
}

func TestCliHistoryAdapters(t *testing.T) {
	setHistoryFile(t)
	c := &Cli{}
	c.AppendToHistory("cmd1")
	c.AppendToHistory("cmd2")
	got := c.GetLastUniqueHistory(2)
	if len(got) != 2 || got[0] != "cmd2" || got[1] != "cmd1" {
		t.Fatalf("history = %v", got)
	}
}

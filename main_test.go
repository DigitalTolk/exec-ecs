package main

import (
	"context"
	"ecs-tool/cli"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetKeyByValue(t *testing.T) {
	t.Parallel()

	m := map[string]string{"alpha": "1", "beta": "2", "gamma": "3"}
	if got := getKeyByValue(m, "2"); got != "beta" {
		t.Fatalf("expected beta, got %q", got)
	}
	if got := getKeyByValue(m, "missing"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := getKeyByValue(nil, "x"); got != "" {
		t.Fatalf("expected empty for nil map, got %q", got)
	}
}

func TestResetFrom(t *testing.T) {
	t.Parallel()

	state := stepState{
		Region: "us-east-1", ClusterArn: "c", Service: "s", TaskArn: "t", Container: "k",
	}

	clone := state
	resetFrom(&clone, stepContainer)
	if clone.Container != "" || clone.TaskArn == "" {
		t.Fatalf("only Container should clear: %+v", clone)
	}

	clone = state
	resetFrom(&clone, stepTask)
	if clone.TaskArn != "" || clone.Container != "" || clone.Service == "" {
		t.Fatalf("Task/Container should clear: %+v", clone)
	}

	clone = state
	resetFrom(&clone, stepCluster)
	if clone.ClusterArn != "" || clone.Service != "" || clone.TaskArn != "" || clone.Container != "" {
		t.Fatalf("cluster-down fields should clear: %+v", clone)
	}
	if clone.Region == "" {
		t.Fatalf("Region must be preserved when resetting from cluster: %+v", clone)
	}

	clone = state
	clone.AutoSelectedCluster = true
	resetFrom(&clone, stepCluster)
	if clone.AutoSelectedCluster {
		t.Fatalf("cluster reset should clear auto-selected flag: %+v", clone)
	}

	clone = state
	resetFrom(&clone, stepRegion)
	if clone.Region != "" {
		t.Fatalf("Region should clear: %+v", clone)
	}

	clone = state
	resetFrom(&clone, stepProfile)
	if clone.Region != "" || clone.ClusterArn != "" {
		t.Fatalf("profile reset should clear everything below: %+v", clone)
	}
}

func TestInitialSelectionStepUsesDeepestMissingField(t *testing.T) {
	t.Parallel()

	if got := initialSelectionStep(stepState{}); got != stepProfile {
		t.Fatalf("empty state step = %d, want profile", got)
	}
	if got := initialSelectionStep(stepState{Profile: "p", Region: "r"}); got != stepCluster {
		t.Fatalf("profile+region step = %d, want cluster", got)
	}
	if got := initialSelectionStep(stepState{Profile: "p", Region: "r", ClusterArn: "c", Service: "s", TaskArn: "t"}); got != stepContainer {
		t.Fatalf("task-selected step = %d, want container", got)
	}
	if got := initialSelectionStep(stepState{Profile: "p", Region: "r", ClusterArn: "c", Service: "s", TaskArn: "t", Container: "k"}); got != finalStep {
		t.Fatalf("complete state step = %d, want final", got)
	}
}

func TestPostSessionResetPreservesSelectedTask(t *testing.T) {
	t.Parallel()

	state := stepState{
		Profile: "p", Region: "r", ClusterArn: "c", Service: "s", TaskArn: "t", Container: "k", AutoSelectedCluster: true,
	}
	resetFrom(&state, stepContainer)
	if state.TaskArn != "t" || state.Container != "" || !state.AutoSelectedCluster {
		t.Fatalf("post-session reset should preserve task and clear container: %+v", state)
	}
	if got := initialSelectionStep(state); got != stepContainer {
		t.Fatalf("post-session next step = %d, want container", got)
	}
}

func TestBackFromServiceSkipsAutoSelectedCluster(t *testing.T) {
	t.Parallel()

	state := stepState{
		Profile: "p", Region: "r", ClusterArn: "c", Service: "s", AutoSelectedCluster: true,
	}
	next := serviceBackDelta(&state)
	if next != -2 {
		t.Fatalf("service back jump = %d, want -2", next)
	}
	if state.ClusterArn != "" || state.Service != "" || state.Region != "r" || state.AutoSelectedCluster {
		t.Fatalf("auto-selected service back should clear cluster and preserve region: %+v", state)
	}
}

func TestBackFromServiceWithManualClusterGoesToCluster(t *testing.T) {
	t.Parallel()

	state := stepState{
		Profile: "p", Region: "r", ClusterArn: "c", Service: "s",
	}
	next := serviceBackDelta(&state)
	if next != int(cli.ActionBack) {
		t.Fatalf("service back jump = %d, want %d", next, cli.ActionBack)
	}
	if state.ClusterArn != "c" || state.Service != "" {
		t.Fatalf("manual service back should preserve cluster and clear service: %+v", state)
	}
}

func TestNoServicesUnderAutoSelectedClusterDoesNotLoop(t *testing.T) {
	t.Parallel()

	state := stepState{
		Profile: "p", Region: "r", ClusterArn: "c", AutoSelectedCluster: true,
	}
	next := serviceBackDelta(&state)
	if next != stepRegion-stepService {
		t.Fatalf("empty auto-selected cluster service step = %d, want region jump", next)
	}
	if state.ClusterArn != "" || state.AutoSelectedCluster {
		t.Fatalf("empty auto-selected cluster should clear cluster state: %+v", state)
	}
}

func TestInitializeCLIAppliesSavedTheme(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	prevTheme := cli.CurrentTheme
	origArgs := os.Args
	origCmd := flag.CommandLine
	os.Args = []string{"exec-ecs"}
	flag.CommandLine = flag.NewFlagSet("exec-ecs", flag.ExitOnError)
	t.Cleanup(func() {
		cli.CurrentTheme = prevTheme
		os.Args = origArgs
		flag.CommandLine = origCmd
	})

	cli.SaveThemeSelection("Matrix")
	cli.CurrentTheme = cli.DraculaTheme

	_ = initializeCLI(context.Background())

	if cli.CurrentTheme.Name != "Matrix" {
		t.Fatalf("startup theme = %q, want Matrix", cli.CurrentTheme.Name)
	}
}

func TestDefaultProbeRegionUsesCLI(t *testing.T) {
	t.Parallel()

	c := &cli.Cli{Region: "eu-west-1"}
	if got := defaultProbeRegion(c); got != "eu-west-1" {
		t.Fatalf("expected eu-west-1, got %q", got)
	}
	c.Region = ""
	if got := defaultProbeRegion(c); got != "us-east-1" {
		t.Fatalf("expected us-east-1 fallback, got %q", got)
	}
}

func TestBreadcrumbForSelectionSteps(t *testing.T) {
	t.Parallel()

	state := stepState{
		Profile:    "dt",
		Region:     "eu-north-1",
		ClusterArn: "arn:aws:ecs:eu-north-1:123:cluster/prod",
		Service:    "arn:aws:ecs:eu-north-1:123:service/prod/api",
	}
	got := breadcrumbFor(state, stepTask)
	for _, want := range []string{"Profile: dt", "Region: eu-north-1", "Cluster: prod", "Service: api"} {
		if !strings.Contains(got, want) {
			t.Fatalf("breadcrumb %q missing %q", got, want)
		}
	}
}

func TestBackFromRegionPreservesProfileList(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	awsDir := filepath.Join(tmp, ".aws")
	if err := os.MkdirAll(awsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `
[profile alpha]
region = us-east-1

[profile beta]
region = eu-west-1
`
	if err := os.WriteFile(filepath.Join(awsDir, "config"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	state := stepState{Profile: "alpha", Region: "eu-north-1"}
	resetFrom(&state, stepRegion)
	if state.Profile != "alpha" {
		t.Fatalf("profile should survive region back, got %+v", state)
	}

	profiles := (&cli.Cli{}).SelectProfileList()
	if len(profiles) != 2 {
		t.Fatalf("profiles after region back = %v, want alpha/beta", profiles)
	}
}

func TestNoResultErrorsAreDistinct(t *testing.T) {
	t.Parallel()

	errs := []error{errNoRegions, errNoClusters, errNoServices, errNoTasks, errNoContainers}
	for i, err := range errs {
		for j, other := range errs {
			if i != j && err == other {
				t.Fatalf("errors %d and %d should be distinct", i, j)
			}
		}
	}
}

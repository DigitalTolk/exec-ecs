package main

import (
	"ecs-tool/cli"
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

package installer

import (
	"runtime"
	"strings"
	"testing"
)

func TestGetBinaryName(t *testing.T) {
	t.Parallel()
	name := getBinaryName()
	if !strings.HasPrefix(name, "exec-ecs_") {
		t.Fatalf("unexpected prefix: %s", name)
	}
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(name, "Darwin") {
			t.Fatalf("expected Darwin in name: %s", name)
		}
	case "linux":
		if !strings.Contains(name, "Linux") {
			t.Fatalf("expected Linux in name: %s", name)
		}
	case "windows":
		if !strings.Contains(name, "Windows") {
			t.Fatalf("expected Windows in name: %s", name)
		}
	}
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(name, ".zip") {
			t.Fatalf("expected .zip suffix on windows: %s", name)
		}
	} else {
		if !strings.HasSuffix(name, ".tar.gz") {
			t.Fatalf("expected .tar.gz suffix: %s", name)
		}
	}
}

func TestIsCommandAvailable(t *testing.T) {
	t.Parallel()
	if !isCommandAvailable("go") {
		t.Fatal("expected `go` to be available in test env")
	}
	if isCommandAvailable("this-does-not-exist-zzz-12345") {
		t.Fatal("expected missing command to report unavailable")
	}
}

func TestVersionSet(t *testing.T) {
	t.Parallel()
	if Version == "" {
		t.Fatal("Version constant must be set")
	}
	if !strings.HasPrefix(Version, "v") {
		t.Fatalf("Version should start with v: %s", Version)
	}
}

func TestRuntimeDependenciesDoNotRequireAWSCLI(t *testing.T) {
	t.Parallel()
	deps := runtimeDependencies()
	if len(deps) == 0 {
		t.Fatal("expected at least one runtime dependency")
	}
	for _, dep := range deps {
		if dep.command == "aws" {
			t.Fatal("AWS CLI should not be required for native ECS/SSO calls")
		}
	}
}

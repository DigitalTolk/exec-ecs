package installer

import (
	"runtime"
	"strings"
	"testing"
)

func TestGetBinaryName(t *testing.T) {
	t.Parallel()
	name, ok := getBinaryName()
	if !ok {
		t.Fatalf("current platform should be supported: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
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

func TestBinaryNameForSupportedArchitectures(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		goos string
		arch string
		want string
	}{
		"linux amd64":   {"linux", "amd64", "exec-ecs_Linux_x86_64.tar.gz"},
		"linux arm64":   {"linux", "arm64", "exec-ecs_Linux_arm64.tar.gz"},
		"darwin amd64":  {"darwin", "amd64", "exec-ecs_Darwin_x86_64.tar.gz"},
		"darwin arm64":  {"darwin", "arm64", "exec-ecs_Darwin_arm64.tar.gz"},
		"windows amd64": {"windows", "amd64", "exec-ecs_Windows_x86_64.zip"},
		"windows arm64": {"windows", "arm64", "exec-ecs_Windows_arm64.zip"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := binaryNameFor(tc.goos, tc.arch)
			if !ok {
				t.Fatal("expected architecture to be supported")
			}
			if got != tc.want {
				t.Fatalf("binaryNameFor() = %q want %q", got, tc.want)
			}
		})
	}
}

func TestBinaryNameForDeprecatedArchitectures(t *testing.T) {
	t.Parallel()

	for _, arch := range []string{"386", "arm"} {
		if got, ok := binaryNameFor("linux", arch); ok {
			t.Fatalf("deprecated architecture %s should not be supported, got %q", arch, got)
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

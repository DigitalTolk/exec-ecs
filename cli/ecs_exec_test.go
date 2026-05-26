package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type fakeECSExecuter struct {
	out *ecs.ExecuteCommandOutput
	err error
}

func (f *fakeECSExecuter) ExecuteCommand(ctx context.Context, params *ecs.ExecuteCommandInput, optFns ...func(*ecs.Options)) (*ecs.ExecuteCommandOutput, error) {
	return f.out, f.err
}

func TestExecECSFailsWithEmptySession(t *testing.T) {
	prevStart := startExecuteCommand
	prevStarter := sessionStarter
	t.Cleanup(func() {
		startExecuteCommand = prevStart
		sessionStarter = prevStarter
	})

	startExecuteCommand = func(ctx context.Context, _ ecsExecuteCommander, opts ExecOptions) (*ecs.ExecuteCommandOutput, error) {
		return &ecs.ExecuteCommandOutput{Session: &ecstypes.Session{}}, nil
	}
	sessionStarter = func(ctx context.Context, region string, sess *ecstypes.Session) (int, error) {
		t.Fatal("sessionStarter should not run for empty session")
		return 0, nil
	}

	setHistoryFile(t)
	c := &Cli{}
	code, err := ExecECS(context.Background(), c, aws.Config{}, ExecOptions{
		Region: "us-east-1", ClusterArn: "c", TaskArn: "t", Container: "main", Command: "bash",
	})
	if err == nil {
		t.Fatal("expected error for empty session")
	}
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
}

func TestExecECSPropagatesSDKError(t *testing.T) {
	prev := startExecuteCommand
	t.Cleanup(func() { startExecuteCommand = prev })

	startExecuteCommand = func(ctx context.Context, _ ecsExecuteCommander, opts ExecOptions) (*ecs.ExecuteCommandOutput, error) {
		return nil, errors.New("access denied")
	}
	setHistoryFile(t)
	c := &Cli{}
	if _, err := ExecECS(context.Background(), c, aws.Config{}, ExecOptions{Region: "us-east-1"}); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestExecECSInvokesStarter(t *testing.T) {
	prevStart := startExecuteCommand
	prevStarter := sessionStarter
	t.Cleanup(func() {
		startExecuteCommand = prevStart
		sessionStarter = prevStarter
	})

	startExecuteCommand = func(ctx context.Context, _ ecsExecuteCommander, opts ExecOptions) (*ecs.ExecuteCommandOutput, error) {
		return &ecs.ExecuteCommandOutput{Session: &ecstypes.Session{
			SessionId:  aws.String("s-1"),
			StreamUrl:  aws.String("wss://example/stream"),
			TokenValue: aws.String("tok"),
		}}, nil
	}
	called := false
	sessionStarter = func(ctx context.Context, region string, sess *ecstypes.Session) (int, error) {
		called = true
		if region != "eu-west-1" {
			t.Fatalf("region = %q", region)
		}
		if aws.ToString(sess.SessionId) != "s-1" {
			t.Fatalf("session id = %q", aws.ToString(sess.SessionId))
		}
		return 42, nil
	}

	setHistoryFile(t)
	c := &Cli{}
	code, err := ExecECS(context.Background(), c, aws.Config{}, ExecOptions{
		Region: "eu-west-1", ClusterArn: "c", TaskArn: "t", Container: "main", Command: "bash",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if code != 42 {
		t.Fatalf("expected starter's exit code, got %d", code)
	}
	if !called {
		t.Fatal("sessionStarter not called")
	}

	// Confirm the history was appended with a structured note rather than
	// a literal `aws ecs execute-command ...` line.
	hist := GetLastUniqueHistory(1)
	if len(hist) != 1 || !contains(hist[0], "ecs exec cluster=c") {
		t.Fatalf("history = %v", hist)
	}
}

func TestStartSessionManagerPluginRejectsNil(t *testing.T) {
	t.Parallel()
	code, err := startSessionManagerPlugin(context.Background(), "us-east-1", nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
	if code != 1 {
		t.Fatalf("exit = %d", code)
	}
}

func TestStartSessionManagerPluginNoBinary(t *testing.T) {
	// Force PATH to a directory that can't possibly contain
	// session-manager-plugin so the exec returns early. The pty.Start call
	// itself is what fails — runPTYCommand returns (1, error).
	t.Setenv("PATH", t.TempDir())
	session := &ecstypes.Session{
		SessionId:  aws.String("s"),
		StreamUrl:  aws.String("wss://x"),
		TokenValue: aws.String("t"),
	}
	code, err := startSessionManagerPlugin(context.Background(), "us-east-1", session)
	if err == nil {
		t.Fatal("expected exec failure when binary is absent")
	}
	if code != 1 {
		t.Fatalf("exit = %d", code)
	}
}

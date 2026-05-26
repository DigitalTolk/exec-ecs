package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ecsExecuteCommander is the small interface we need from the ECS SDK client
// to start an exec session. Defined here so tests can supply a fake.
type ecsExecuteCommander interface {
	ExecuteCommand(ctx context.Context, params *ecs.ExecuteCommandInput, optFns ...func(*ecs.Options)) (*ecs.ExecuteCommandOutput, error)
}

// sessionStarter spawns the session-manager-plugin process with the supplied
// session JSON. The default implementation execs the real binary; tests
// inject a stub.
var sessionStarter func(ctx context.Context, region string, session *ecstypes.Session) (int, error) = startSessionManagerPlugin

// ExecOptions captures everything ExecECS needs to launch a session.
type ExecOptions struct {
	Region     string
	ClusterArn string
	TaskArn    string
	Container  string
	Command    string
}

// ExecECS calls ecs:ExecuteCommand via the SDK, then drives the resulting
// SSM session by exec'ing session-manager-plugin directly. We never shell
// out to `aws ecs execute-command`, so the caller stays in-process and can
// loop back to the menu after the session exits.
//
// Returns the exit code of the inner session (0 on a clean shell exit, the
// plugin's exit code otherwise).
func ExecECS(ctx context.Context, c *Cli, awsCfg aws.Config, opts ExecOptions) (int, error) {
	c.LogAWSCommand("ecs", "execute-command",
		"--cluster", opts.ClusterArn,
		"--task", opts.TaskArn,
		"--container", opts.Container,
		"--interactive",
		"--command", opts.Command,
		"--profile", c.Profile,
		"--region", opts.Region,
	)

	client := ecs.NewFromConfig(awsCfg, func(o *ecs.Options) {
		o.Region = opts.Region
	})

	resp, err := startExecuteCommand(ctx, client, opts)
	if err != nil {
		return 1, fmt.Errorf("ecs:ExecuteCommand failed: %w", err)
	}
	if resp.Session == nil ||
		aws.ToString(resp.Session.SessionId) == "" ||
		aws.ToString(resp.Session.StreamUrl) == "" ||
		aws.ToString(resp.Session.TokenValue) == "" {
		return 1, errors.New("ecs:ExecuteCommand returned an empty session — is the task running with `enable-execute-command`?")
	}

	c.AppendToHistory(fmt.Sprintf("# ecs exec cluster=%s task=%s container=%s region=%s command=%q",
		opts.ClusterArn, opts.TaskArn, opts.Container, opts.Region, opts.Command))

	return sessionStarter(ctx, opts.Region, resp.Session)
}

// startExecuteCommand is the SDK call, factored out for testability.
var startExecuteCommand = func(ctx context.Context, client ecsExecuteCommander, opts ExecOptions) (*ecs.ExecuteCommandOutput, error) {
	return client.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     aws.String(opts.ClusterArn),
		Task:        aws.String(opts.TaskArn),
		Container:   aws.String(opts.Container),
		Command:     aws.String(opts.Command),
		Interactive: true,
	})
}

// sessionJSON encodes only the three fields session-manager-plugin actually
// needs from the ecs.Session. We marshal manually instead of relying on the
// SDK's noSmithyDocumentSerde-tagged struct so the wire format is stable.
type sessionJSON struct {
	SessionID  string `json:"SessionId"`
	StreamURL  string `json:"StreamUrl"`
	TokenValue string `json:"TokenValue"`
}

// startSessionManagerPlugin spawns the real plugin, wiring it up to a PTY so
// the inner shell behaves identically to running `aws ecs execute-command`.
// Returns the plugin's exit code so the caller can decide whether to loop.
func startSessionManagerPlugin(ctx context.Context, region string, session *ecstypes.Session) (int, error) {
	if session == nil {
		return 1, errors.New("nil session")
	}
	payload := sessionJSON{
		SessionID:  aws.ToString(session.SessionId),
		StreamURL:  aws.ToString(session.StreamUrl),
		TokenValue: aws.ToString(session.TokenValue),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return 1, fmt.Errorf("marshal session: %w", err)
	}

	// Argv documented at https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html
	// session-manager-plugin <session-json> <region> StartSession
	cmd := exec.CommandContext(ctx, "session-manager-plugin",
		string(data),
		region,
		"StartSession",
	)
	return runPTYCommand(cmd)
}

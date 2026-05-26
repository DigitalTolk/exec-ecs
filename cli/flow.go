package cli

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// Selector picks a value out of a list. It's the thin abstraction we use to
// decouple the picker functions from the bubbletea TUI: production wires this
// to (*Cli).PromptSelect; tests supply a deterministic stub.
type Selector interface {
	Select(label string, items []string, defaultSelected string, showGoBack bool) (string, bool)
}

// SelectorFunc adapts an ordinary function to Selector.
type SelectorFunc func(label string, items []string, defaultSelected string, showGoBack bool) (string, bool)

// Select implements the Selector interface.
func (f SelectorFunc) Select(label string, items []string, defaultSelected string, showGoBack bool) (string, bool) {
	return f(label, items, defaultSelected, showGoBack)
}

// CliSelector returns a Selector that uses (*Cli).PromptSelect.
func (c *Cli) CliSelector() Selector {
	return SelectorFunc(c.PromptSelect)
}

// State captures the picker's current selection. It's a value type so callers
// can pass it through several pick steps without sharing a mutable pointer
// inadvertently — the pick* helpers each return the updated state.
type State struct {
	Profile    string
	Region     string
	ClusterArn string
	Service    string
	TaskArn    string
	Container  string
}

// PickAction tells the caller how to advance the state machine after a step:
// move forward, retry the same step, or go one step back.
type PickAction int

const (
	// ActionAdvance moves to the next step.
	ActionAdvance PickAction = 1
	// ActionRetry stays on the current step (used when the user picks
	// "Retry" from a no-results menu).
	ActionRetry PickAction = 0
	// ActionBack rewinds to the previous step.
	ActionBack PickAction = -1
)

// PickCluster lists ECS clusters in the active region, prompts for one, and
// updates state.ClusterArn. Returns ActionRetry if the user asked to retry a
// lookup, ActionBack if they hit go-back.
func PickCluster(ctx context.Context, c *Cli, sel Selector, client ecsClusterLister, state State) (State, PickAction, error) {
	c.LogAWSCommand("ecs", "list-clusters", "--profile", c.Profile, "--region", c.Region)
	clusters, clusterArns, err := c.ListClusterNamesArns(ctx, client)
	if err != nil {
		fmt.Println("Failed to list ECS clusters:", err)
		choice, goBack := sel.Select("Cluster lookup failed. What now?", []string{"Retry", "Back"}, "Retry", true)
		if goBack || choice == "Back" {
			resetState(&state, stepIdxCluster)
			return state, ActionBack, nil
		}
		return state, ActionRetry, nil
	}
	if len(clusters) == 0 {
		fmt.Println("No ECS clusters found in region:", c.Region)
		choice, goBack := sel.Select("No clusters found. What now?", []string{"Retry", "Back"}, "Retry", true)
		if goBack || choice == "Back" {
			resetState(&state, stepIdxCluster)
			ClearRegionCache(c.Profile)
			return state, ActionBack, nil
		}
		return state, ActionRetry, nil
	}
	selected, goBack := sel.Select("Choose ECS cluster", clusters, keyForValue(clusterArns, state.ClusterArn), true)
	if goBack {
		resetState(&state, stepIdxCluster)
		return state, ActionBack, nil
	}
	state.ClusterArn = clusterArns[selected]
	c.ClusterArn = state.ClusterArn
	resetState(&state, stepIdxService)
	return state, ActionAdvance, nil
}

// PickService prompts for an ECS service inside the chosen cluster.
func PickService(ctx context.Context, c *Cli, sel Selector, client ecsServiceLister, state State) (State, PickAction, error) {
	c.LogAWSCommand("ecs", "list-services", "--cluster", state.ClusterArn, "--profile", c.Profile, "--region", c.Region)
	services, serviceArns, err := c.ListServiceNamesArns(ctx, client, state.ClusterArn)
	if err != nil {
		fmt.Println("Failed to list ECS services:", err)
		resetState(&state, stepIdxService)
		return state, ActionBack, nil
	}
	if len(services) == 0 {
		fmt.Println("No ECS services found. Going back.")
		resetState(&state, stepIdxService)
		return state, ActionBack, nil
	}
	selected, goBack := sel.Select("Choose ECS service", services, keyForValue(serviceArns, state.Service), true)
	if goBack {
		resetState(&state, stepIdxService)
		return state, ActionBack, nil
	}
	state.Service = serviceArns[selected]
	c.Service = state.Service
	resetState(&state, stepIdxTask)
	return state, ActionAdvance, nil
}

// PickTask prompts for an ECS task inside the chosen service.
func PickTask(ctx context.Context, c *Cli, sel Selector, client ecsTaskLister, state State) (State, PickAction, error) {
	c.LogAWSCommand("ecs", "list-tasks", "--cluster", state.ClusterArn, "--service-name", state.Service, "--profile", c.Profile, "--region", c.Region)
	tasks, taskArns, err := c.ListTaskNamesArns(ctx, client, state.ClusterArn, state.Service)
	if err != nil {
		fmt.Println("Failed to list ECS tasks:", err)
		resetState(&state, stepIdxTask)
		return state, ActionBack, nil
	}
	if len(tasks) == 0 {
		fmt.Println("No ECS tasks found. Going back.")
		resetState(&state, stepIdxTask)
		return state, ActionBack, nil
	}
	selected, goBack := sel.Select("Choose ECS task", tasks, keyForValue(taskArns, state.TaskArn), true)
	if goBack {
		resetState(&state, stepIdxTask)
		return state, ActionBack, nil
	}
	state.TaskArn = taskArns[selected]
	c.TaskArn = state.TaskArn
	resetState(&state, stepIdxContainer)
	return state, ActionAdvance, nil
}

// PickContainer prompts for a container inside the chosen task.
func PickContainer(ctx context.Context, c *Cli, sel Selector, client ecsTaskDescriber, state State) (State, PickAction, error) {
	c.LogAWSCommand("ecs", "describe-tasks", "--cluster", state.ClusterArn, "--tasks", state.TaskArn, "--profile", c.Profile, "--region", c.Region)
	containers, err := c.ListContainerNames(ctx, client, state.ClusterArn, state.TaskArn)
	if err != nil {
		fmt.Println("Failed to describe ECS task:", err)
		resetState(&state, stepIdxContainer)
		return state, ActionBack, nil
	}
	if len(containers) == 0 {
		fmt.Println("No containers found. Going back.")
		resetState(&state, stepIdxContainer)
		return state, ActionBack, nil
	}
	selected, goBack := sel.Select("Choose a container", containers, state.Container, true)
	if goBack {
		resetState(&state, stepIdxContainer)
		return state, ActionBack, nil
	}
	state.Container = selected
	c.Container = state.Container
	return state, ActionAdvance, nil
}

// Step indices used by resetState, mirroring main.go's constants but owned by
// the cli package so we can test the reset logic without dragging the binary
// entry point in.
const (
	stepIdxProfile   = 0
	stepIdxRegion    = 1
	stepIdxCluster   = 2
	stepIdxService   = 3
	stepIdxTask      = 4
	stepIdxContainer = 5
)

// resetState clears every State field at or below `from` so a back-navigation
// (or a fresh selection) doesn't carry stale ARNs forward.
func resetState(state *State, from int) {
	if from <= stepIdxRegion {
		state.Region = ""
	}
	if from <= stepIdxCluster {
		state.ClusterArn = ""
	}
	if from <= stepIdxService {
		state.Service = ""
	}
	if from <= stepIdxTask {
		state.TaskArn = ""
	}
	if from <= stepIdxContainer {
		state.Container = ""
	}
}

// keyForValue returns the map key whose value equals v, or "" if there is no
// match. Used to translate an ARN back to its display name when re-rendering
// the picker with the previously-selected default highlighted.
func keyForValue(m map[string]string, v string) string {
	for k, val := range m {
		if val == v {
			return k
		}
	}
	return ""
}

// ECSClient is the union of the ECS sub-interfaces the picker steps need. The
// concrete *ecs.Client (and the test fake) both satisfy it.
type ECSClient interface {
	ecsClusterLister
	ecsServiceLister
	ecsTaskLister
	ecsTaskDescriber
}

// NewECSClient is a thin constructor so callers don't import ecs directly.
func NewECSClient(cfg aws.Config, region string) ECSClient {
	return ecs.NewFromConfig(cfg, func(o *ecs.Options) { o.Region = region })
}

package main

import (
	"context"
	"ecs-tool/cli"
	"ecs-tool/installer"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/briandowns/spinner"
)

type stepState struct {
	Profile    string
	Region     string
	ClusterArn string
	Service    string
	TaskArn    string
	Container  string
}

const (
	stepProfile   = 0
	stepRegion    = 1
	stepCluster   = 2
	stepService   = 3
	stepTask      = 4
	stepContainer = 5
	finalStep     = 6
)

func main() {
	ctx := context.Background()

	installer.CheckAndInstallDependencies()

	c := initializeCLI(ctx)

	if c.History {
		showHistoryAndExecute(c)
		return
	}

	state := stepState{
		Profile:    c.Profile,
		Region:     c.Region,
		ClusterArn: c.ClusterArn,
		Service:    c.Service,
		TaskArn:    c.TaskArn,
		Container:  c.Container,
	}

	// Outer loop: after each exec session ends, drop the user back into the
	// picker (rewinding to the cluster step) so they can run another command
	// without restarting the binary. The interactive picker itself handles
	// ctrl+b back-navigation and ctrl+c clean exit.
	awsCfg := aws.Config{}
	awsCfgLoaded := false
	for {
		var err error
		awsCfg, awsCfgLoaded, err = runInteractiveSelection(ctx, c, &state, awsCfg, awsCfgLoaded)
		if err != nil {
			c.LogUserFriendlyError("Selection failed", err, "See error details above.", "", 0)
		}

		exitCode, execErr := cli.ExecECS(ctx, c, awsCfg, cli.ExecOptions{
			Region:     state.Region,
			ClusterArn: state.ClusterArn,
			TaskArn:    state.TaskArn,
			Container:  state.Container,
			Command:    c.Command,
		})
		if execErr != nil {
			fmt.Fprintln(os.Stderr, "exec-ecs:", execErr)
		}
		if exitCode != 0 {
			fmt.Fprintf(os.Stderr, "session exited with code %d\n", exitCode)
		}

		// Re-enter the picker at the cluster step (cluster/service/task/
		// container get cleared by resetFrom). Profile, region, and the
		// already-loaded AWS config are preserved so the loop is fast.
		resetFrom(&state, stepCluster)
	}
}

func runInteractiveSelection(ctx context.Context, c *cli.Cli, state *stepState, awsCfg aws.Config, awsCfgLoaded bool) (aws.Config, bool, error) {
	step := stepProfile
	// If we're re-entering with state already populated from a previous
	// session, skip forward to the first empty field.
	if state.Profile != "" {
		step = stepRegion
		if state.Region != "" {
			step = stepCluster
		}
	}
	ssoEnsured := awsCfgLoaded

	for step < finalStep {
		switch step {
		case stepProfile:
			profiles := c.SelectProfileList()
			if len(profiles) == 0 {
				return awsCfg, awsCfgLoaded, fmt.Errorf("no AWS profiles found")
			}
			selected, goBack := c.PromptSelect("Choose AWS profile", profiles, state.Profile, step > stepProfile)
			if goBack && step > stepProfile {
				resetFrom(state, stepProfile)
				awsCfgLoaded = false
				ssoEnsured = false
				step--
				continue
			}
			if state.Profile != selected {
				awsCfgLoaded = false
				ssoEnsured = false
			}
			state.Profile = selected
			c.Profile = selected
			resetFrom(state, stepRegion)
			step++

		case stepRegion:
			if !ssoEnsured {
				if err := ensureSSOLogin(ctx, c); err != nil {
					return awsCfg, awsCfgLoaded, err
				}
				ssoEnsured = true
			}

			regions, err := discoverRegions(ctx, c)
			if err != nil {
				fmt.Println("Failed to discover regions with clusters:", err)
				regions = cli.DefaultRegions
			}
			if len(regions) == 0 {
				fmt.Println("No regions with ECS clusters found for profile", c.Profile)
				choice, goBack := c.PromptSelect("No regions with clusters. What now?",
					[]string{"Refresh", "Back"}, "Refresh", true)
				if goBack || choice == "Back" {
					resetFrom(state, stepProfile)
					awsCfgLoaded = false
					ssoEnsured = false
					step--
					continue
				}
				cli.ClearRegionCache(c.Profile)
				continue
			}

			selected, goBack := c.PromptWithDefault("Choose AWS region", state.Region, regions, true)
			if goBack {
				resetFrom(state, stepRegion)
				awsCfgLoaded = false
				step--
				continue
			}
			if state.Region != selected {
				awsCfgLoaded = false
			}
			state.Region = selected
			c.Region = selected
			resetFrom(state, stepCluster)
			step++

		case stepCluster, stepService, stepTask, stepContainer:
			if !awsCfgLoaded {
				cfg, err := loadAWSConfig(ctx, c)
				if err != nil {
					return awsCfg, awsCfgLoaded, err
				}
				awsCfg = cfg
				if !ssoEnsured {
					if err := validateSSOSession(ctx, c, awsCfg); err != nil {
						return awsCfg, awsCfgLoaded, err
					}
					ssoEnsured = true
				}
				awsCfgLoaded = true
			}

			switch step {
			case stepCluster:
				next, err := pickCluster(ctx, c, awsCfg, state)
				if err != nil {
					return awsCfg, awsCfgLoaded, err
				}
				step += next
			case stepService:
				next, err := pickService(ctx, c, awsCfg, state)
				if err != nil {
					return awsCfg, awsCfgLoaded, err
				}
				step += next
			case stepTask:
				next, err := pickTask(ctx, c, awsCfg, state)
				if err != nil {
					return awsCfg, awsCfgLoaded, err
				}
				step += next
			case stepContainer:
				next, err := pickContainer(ctx, c, awsCfg, state)
				if err != nil {
					return awsCfg, awsCfgLoaded, err
				}
				step += next
			}
		}
	}
	return awsCfg, awsCfgLoaded, nil
}

func resetFrom(state *stepState, from int) {
	if from <= stepRegion {
		state.Region = ""
	}
	if from <= stepCluster {
		state.ClusterArn = ""
	}
	if from <= stepService {
		state.Service = ""
	}
	if from <= stepTask {
		state.TaskArn = ""
	}
	if from <= stepContainer {
		state.Container = ""
	}
}

// pickCluster / pickService / pickTask / pickContainer are thin wrappers that
// add the spinner UX around the testable cli.Pick* helpers (which carry the
// real branching logic) and mirror the per-step state into both the state
// struct and the persistent Cli flags.
func pickCluster(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	sp := createSpinner("Connecting to ECS...")
	client := cli.NewECSClient(awsCfg, c.Region)
	out, action, err := cli.PickCluster(ctx, c, c.CliSelector(), client, toCliState(*state))
	sp.Stop()
	*state = fromCliState(out)
	return int(action), err
}

func pickService(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	sp := createSpinner("Fetching ECS services...")
	client := cli.NewECSClient(awsCfg, c.Region)
	out, action, err := cli.PickService(ctx, c, c.CliSelector(), client, toCliState(*state))
	sp.Stop()
	*state = fromCliState(out)
	return int(action), err
}

func pickTask(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	sp := createSpinner("Fetching ECS tasks...")
	client := cli.NewECSClient(awsCfg, c.Region)
	out, action, err := cli.PickTask(ctx, c, c.CliSelector(), client, toCliState(*state))
	sp.Stop()
	*state = fromCliState(out)
	return int(action), err
}

func pickContainer(ctx context.Context, c *cli.Cli, awsCfg aws.Config, state *stepState) (int, error) {
	sp := createSpinner("Fetching ECS containers...")
	client := cli.NewECSClient(awsCfg, c.Region)
	out, action, err := cli.PickContainer(ctx, c, c.CliSelector(), client, toCliState(*state))
	sp.Stop()
	*state = fromCliState(out)
	return int(action), err
}

func toCliState(s stepState) cli.State {
	return cli.State{
		Profile: s.Profile, Region: s.Region, ClusterArn: s.ClusterArn,
		Service: s.Service, TaskArn: s.TaskArn, Container: s.Container,
	}
}

func fromCliState(s cli.State) stepState {
	return stepState{
		Profile: s.Profile, Region: s.Region, ClusterArn: s.ClusterArn,
		Service: s.Service, TaskArn: s.TaskArn, Container: s.Container,
	}
}

func discoverRegions(ctx context.Context, c *cli.Cli) ([]string, error) {
	sp := createSpinner("Discovering regions with ECS clusters...")
	defer sp.Stop()
	return cli.DiscoverRegionsWithClusters(ctx, c.Profile, cli.DefaultRegions)
}

func initializeCLI(ctx context.Context) *cli.Cli {
	c := cli.ParseArgs()
	_ = ctx
	switch {
	case c.Version:
		fmt.Println("exec-ecs version", installer.Version)
		os.Exit(0)
	case c.Upgrade:
		installer.UpgradeExecECS()
		os.Exit(0)
	}
	return &c
}

func loadAWSConfig(ctx context.Context, c *cli.Cli) (aws.Config, error) {
	sp := createSpinner("Loading AWS configuration...")
	defer sp.Stop()

	c.LogAWSCommand("configure", "get", "region", "--profile", c.Profile)
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(c.Region),
		config.WithSharedConfigProfile(c.Profile),
	)

	if err != nil {
		return aws.Config{}, fmt.Errorf("unable to load AWS configuration: %w", err)
	}

	return cfg, nil
}

// ensureSSOLogin authenticates the user up-front so that all profiles bound
// to the same sso-session piggy-back on a single login. When the profile is
// SSO-based we drive the OAuth2 device-code flow natively (no `aws` CLI
// process required); we fall back to `aws sso login --profile` only for
// legacy non-sso-session setups.
func ensureSSOLogin(ctx context.Context, c *cli.Cli) error {
	probeCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(defaultProbeRegion(c)),
		config.WithSharedConfigProfile(c.Profile),
	)
	if err != nil {
		return fmt.Errorf("unable to load AWS configuration: %w", err)
	}

	stsClient := sts.NewFromConfig(probeCfg)
	c.LogAWSCommand("sts", "get-caller-identity", "--profile", c.Profile)
	if err := c.CheckSSOSession(ctx, stsClient, c.Profile); err == nil {
		return nil
	}

	ssoCfg, err := c.LookupSSOSessionConfig(c.Profile)
	if err != nil {
		return err
	}
	if ssoCfg != nil {
		fmt.Printf("No active SSO session found. Logging in to sso-session %q (covers every profile bound to it)...\n", ssoCfg.Name)
		c.LogAWSCommand("[native]", "sso", "login", "--sso-session", ssoCfg.Name)
		return c.PerformNativeSSOLogin(ctx, ssoCfg)
	}

	fmt.Println("No sso-session block found for this profile. Falling back to `aws sso login --profile`.")
	c.LogAWSCommand("sso", "login", "--profile", c.Profile)
	cmd := exec.Command("aws", "sso", "login", "--profile", c.Profile)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws sso login failed: %w", err)
	}
	return nil
}

func defaultProbeRegion(c *cli.Cli) string {
	if c.Region != "" {
		return c.Region
	}
	return "us-east-1"
}

func validateSSOSession(ctx context.Context, c *cli.Cli, awsCfg aws.Config) error {
	sp := createSpinner("Checking AWS SSO session...")
	defer sp.Stop()

	stsClient := sts.NewFromConfig(awsCfg)
	c.LogAWSCommand("sts", "get-caller-identity", "--profile", c.Profile)
	if err := c.CheckSSOSession(ctx, stsClient, c.Profile); err != nil {
		sp.Stop()
		return ensureSSOLogin(ctx, c)
	}
	return nil
}

func createSpinner(suffix string) *spinner.Spinner {
	sp := spinner.New(spinner.CharSets[38], 100*time.Millisecond)
	sp.Suffix = " " + suffix
	sp.Start()
	return sp
}

func showHistoryAndExecute(c *cli.Cli) {
	history := c.GetLastUniqueHistory(5)
	if len(history) == 0 {
		fmt.Println("No command history found.")
		return
	}
	selected, err := c.BubbleteaHistorySelect("Command History (last 5 unique)", history)
	if err != nil || selected == "" {
		return
	}
	fmt.Println("Executing:", selected)
	cmd := exec.Command("sh", "-c", selected)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

func getKeyByValue(m map[string]string, value string) string {
	for k, v := range m {
		if v == value {
			return k
		}
	}
	return ""
}

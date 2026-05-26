package cli

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"gopkg.in/ini.v1"
)

type Cli struct {
	Interactive bool
	Profile     string
	Region      string
	ClusterArn  string
	Service     string
	TaskArn     string
	Container   string
	Command     string
	Debug       bool
	Version     bool
	Upgrade     bool
	History     bool
}

func ParseArgs() Cli {
	var (
		profile   string
		region    string
		cluster   string
		service   string
		task      string
		container string
		command   string
		debug     bool
		version   bool
		upgrade   bool
		history   bool
	)

	flag.BoolVar(&debug, "debug", false, "Enable debug mode for logging AWS commands")
	flag.BoolVar(&version, "version", false, "Show the current version")
	flag.BoolVar(&upgrade, "upgrade", false, "Upgrade to the latest version")
	flag.BoolVar(&history, "history", false, "Show last 5 unique command history")
	flag.BoolVar(&history, "H", false, "Show last 5 unique command history (shorthand)")
	flag.StringVar(&profile, "pr", "", "AWS profile to use")
	flag.StringVar(&region, "rg", "", "AWS region to use")
	flag.StringVar(&cluster, "cl", "", "ECS cluster name")
	flag.StringVar(&service, "se", "", "AWS service name")
	flag.StringVar(&task, "tk", "", "Task ARN")
	flag.StringVar(&container, "cn", "", "Container name")
	flag.StringVar(&command, "command", "bash", "Command to run in the container")
	flag.Parse()

	return Cli{
		Debug:       debug,
		Interactive: true,
		Profile:     profile,
		Region:      region,
		ClusterArn:  cluster,
		Service:     service,
		TaskArn:     task,
		Container:   container,
		Command:     command,
		Version:     version,
		Upgrade:     upgrade,
		History:     history,
	}
}

func (c *Cli) AppendToHistory(cmd string) {
	AppendToHistory(cmd)
}

func (c *Cli) GetLastUniqueHistory(n int) []string {
	return GetLastUniqueHistory(n)
}

func (c *Cli) BubbleteaHistorySelect(label string, items []string) (string, error) {
	return BubbleteaHistorySelect(label, items)
}

// Helper to get profile list only
func (c *Cli) SelectProfileList() []string {
	awsConfigPath := c.getStoredConfigPath()
	if awsConfigPath == "" {
		awsConfigPath = filepath.Join(homeDir(), ".aws", "config")
	}
	cfg, err := ini.Load(awsConfigPath)
	if err != nil {
		return nil
	}
	var profiles []string
	for _, section := range cfg.Sections() {
		if name, ok := strings.CutPrefix(section.Name(), "profile "); ok {
			profiles = append(profiles, name)
		}
	}
	return profiles
}

// Helper to get cluster display names and a map from name to ARN.
// Returns an error so callers can distinguish "API failure" from "no clusters".
func (c *Cli) ListClusterNamesArns(ctx context.Context, client ecsClusterLister) ([]string, map[string]string, error) {
	clusters, err := listAllClusterArns(ctx, client)
	if err != nil {
		return nil, nil, err
	}
	return namesAndArns(clusters), arnMap(clusters), nil
}

// Helper to get service display names and a map from name to ARN.
func (c *Cli) ListServiceNamesArns(ctx context.Context, client ecsServiceLister, clusterArn string) ([]string, map[string]string, error) {
	services, err := listAllServiceArns(ctx, client, clusterArn)
	if err != nil {
		return nil, nil, err
	}
	return namesAndArns(services), arnMap(services), nil
}

// Helper to get task display names and a map from masked name to ARN.
func (c *Cli) ListTaskNamesArns(ctx context.Context, client ecsTaskLister, clusterArn, serviceName string) ([]string, map[string]string, error) {
	taskArns, err := listAllTaskArns(ctx, client, clusterArn, serviceName)
	if err != nil {
		return nil, nil, err
	}
	nameToArn := make(map[string]string, len(taskArns))
	masked := make([]string, 0, len(taskArns))
	for _, arn := range taskArns {
		mask := maskTaskArn(arn)
		nameToArn[mask] = arn
		masked = append(masked, mask)
	}
	return masked, nameToArn, nil
}

// Helper to get container names for a given task
func (c *Cli) ListContainerNames(ctx context.Context, client ecsTaskDescriber, clusterArn, taskArn string) ([]string, error) {
	output, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &clusterArn,
		Tasks:   []string{taskArn},
	})
	if err != nil {
		return nil, err
	}
	if len(output.Tasks) == 0 {
		return nil, nil
	}
	task := output.Tasks[0]
	names := make([]string, 0, len(task.Containers))
	for _, cont := range task.Containers {
		if cont.Name != nil {
			names = append(names, *cont.Name)
		}
	}
	return names, nil
}

func namesAndArns(arns []string) []string {
	names := make([]string, 0, len(arns))
	for _, arn := range arns {
		parts := strings.Split(arn, "/")
		names = append(names, parts[len(parts)-1])
	}
	return names
}

func arnMap(arns []string) map[string]string {
	out := make(map[string]string, len(arns))
	for _, arn := range arns {
		parts := strings.Split(arn, "/")
		out[parts[len(parts)-1]] = arn
	}
	return out
}

// PromptSelect runs the interactive picker. An empty selection means the
// user quit with q/ctrl+c/esc and is treated as a clean exit (caller's
// responsibility to decide what to do); a non-nil error is fatal because it
// means the bubbletea program itself failed to run.
func (c *Cli) PromptSelect(label string, items []string, defaultSelected string, showGoBack bool) (string, bool) {
	selectedItem, goBack, err := bubbleteaSelect(label, items, defaultSelected, showGoBack)
	if err != nil {
		log.Fatalf("Selection prompt failed: %v", err)
	}
	if goBack {
		return "", true
	}
	if selectedItem == "" {
		// User explicitly quit — exit silently with status 0 rather than
		// dumping a stack trace via log.Fatalf.
		fmt.Println("Cancelled.")
		exitFn(0)
	}
	return selectedItem, false
}

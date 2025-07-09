package cli

import (
	"context"
	"flag"
	"log"
	"os"
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
		awsConfigPath = os.Getenv("HOME") + "/.aws/config"
	}
	cfg, err := ini.Load(awsConfigPath)
	if err != nil {
		return nil
	}
	var profiles []string
	for _, section := range cfg.Sections() {
		if strings.HasPrefix(section.Name(), "profile ") {
			profiles = append(profiles, strings.TrimPrefix(section.Name(), "profile "))
		}
	}
	return profiles
}

// Helper to get cluster display names and a map from name to ARN
func (c *Cli) ListClusterNamesArns(ctx context.Context, client *ecs.Client) ([]string, map[string]string) {
	output, err := client.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, nil
	}
	clusters := output.ClusterArns
	nameToArn := make(map[string]string)
	var names []string
	for _, arn := range clusters {
		parts := strings.Split(arn, "/")
		name := parts[len(parts)-1]
		nameToArn[name] = arn
		names = append(names, name)
	}
	return names, nameToArn
}

// Helper to get service display names and a map from name to ARN
func (c *Cli) ListServiceNamesArns(ctx context.Context, client *ecs.Client, clusterArn string) ([]string, map[string]string) {
	maxResults := int32(100)
	output, err := client.ListServices(ctx, &ecs.ListServicesInput{
		Cluster:    &clusterArn,
		MaxResults: &maxResults,
	})
	if err != nil {
		return nil, nil
	}
	services := output.ServiceArns
	nameToArn := make(map[string]string)
	var names []string
	for _, arn := range services {
		parts := strings.Split(arn, "/")
		name := parts[len(parts)-1]
		nameToArn[name] = arn
		names = append(names, name)
	}
	return names, nameToArn
}

// Helper to get task display names and a map from masked name to ARN
func (c *Cli) ListTaskNamesArns(ctx context.Context, client *ecs.Client, clusterArn, serviceName string) ([]string, map[string]string) {
	var (
		taskArns  []string
		nextToken *string
	)
	for {
		output, err := client.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:     &clusterArn,
			ServiceName: &serviceName,
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, nil
		}
		taskArns = append(taskArns, output.TaskArns...)
		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}
	nameToArn := make(map[string]string)
	var masked []string
	for _, arn := range taskArns {
		mask := maskTaskArn(arn)
		nameToArn[mask] = arn
		masked = append(masked, mask)
	}
	return masked, nameToArn
}

// Helper to get container names for a given task
func (c *Cli) ListContainerNames(ctx context.Context, client *ecs.Client, clusterArn, taskArn string) []string {
	output, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &clusterArn,
		Tasks:   []string{taskArn},
	})
	if err != nil || len(output.Tasks) == 0 {
		return nil
	}
	task := output.Tasks[0]
	if len(task.Containers) == 0 {
		return nil
	}
	var names []string
	for _, cont := range task.Containers {
		if cont.Name != nil {
			names = append(names, *cont.Name)
		}
	}
	return names
}

// Update PromptSelect to match new signature
func (c *Cli) PromptSelect(label string, items []string, defaultSelected string, showGoBack bool) (string, bool) {
	selectedItem, goBack, err := bubbleteaSelect(label, items, defaultSelected, showGoBack)
	if err != nil {
		log.Fatalf("Selection prompt failed: %v", err)
	}
	if goBack {
		return "", true
	}
	if selectedItem == "" {
		log.Fatalf("No selection made, exiting.")
	}
	return selectedItem, false
}

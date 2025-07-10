package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"gopkg.in/ini.v1"
)

func (c *Cli) CheckSSOSession(ctx context.Context, client *sts.Client, profile string) error {
	_, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	return err
}

func (c *Cli) getStoredConfigPath() string {
	customPathFile := os.Getenv("HOME") + "/.aws/custom_config_path"
	if data, err := os.ReadFile(customPathFile); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func (c *Cli) saveCustomConfigPath(path string) error {
	customPathFile := os.Getenv("HOME") + "/.aws/custom_config_path"
	awsDir := filepath.Dir(customPathFile)
	if err := os.MkdirAll(awsDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(customPathFile, []byte(path), 0600)
}

func (c *Cli) SelectProfile() string {
	awsConfigPath := c.getStoredConfigPath()
	if awsConfigPath == "" {
		awsConfigPath = os.Getenv("HOME") + "/.aws/config"
	}
	if _, err := os.Stat(awsConfigPath); os.IsNotExist(err) {
		fmt.Printf("AWS config file not found at %s.\n", awsConfigPath)
		fmt.Print("Please enter AWS config file path:\n> ")
		var newPath string
		_, scanErr := fmt.Scanln(&newPath)
		if scanErr != nil {
			log.Fatalf("Unable to read config path: %v", scanErr)
		}
		awsConfigPath = newPath
		if err := c.saveCustomConfigPath(newPath); err != nil {
			fmt.Printf("Warning: Failed to save custom config path: %v\n", err)
		}
	}
	cfg, err := ini.Load(awsConfigPath)
	if err != nil {
		log.Fatalf("Failed to load AWS config at %s: %v", awsConfigPath, err)
	}
	var profiles []string
	for _, section := range cfg.Sections() {
		if strings.HasPrefix(section.Name(), "profile ") {
			profiles = append(profiles, strings.TrimPrefix(section.Name(), "profile "))
		}
	}
	if len(profiles) == 0 {
		log.Fatalf("No profiles found in AWS config: %s", awsConfigPath)
	}
	selectedProfile, _ := c.PromptSelect("Choose AWS profile", profiles, "", false)
	return selectedProfile
}

func (c *Cli) SelectCluster(ctx context.Context, client *ecs.Client) (string, error) {
	output, err := client.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return "", err
	}
	clusters := output.ClusterArns
	if len(clusters) == 0 {
		return "", fmt.Errorf("no ECS clusters found")
	}
	clusterNames := make([]string, len(clusters))
	for i, arn := range clusters {
		parts := strings.Split(arn, "/")
		clusterNames[i] = parts[len(parts)-1]
	}
	selectedClusterName, _ := c.PromptSelect("Choose ECS cluster", clusterNames, "", false)
	return selectedClusterName, nil
}

func (c *Cli) SelectService(ctx context.Context, client *ecs.Client, clusterArn string) (string, error) {
	maxResults := int32(100)
	output, err := client.ListServices(ctx, &ecs.ListServicesInput{
		Cluster:    &clusterArn,
		MaxResults: &maxResults,
	})
	if err != nil {
		return "", err
	}
	services := output.ServiceArns
	if len(services) == 0 {
		return "", fmt.Errorf("no services found in ECS cluster %s", clusterArn)
	}
	serviceNames := make([]string, len(services))
	for i, arn := range services {
		parts := strings.Split(arn, "/")
		serviceNames[i] = parts[len(parts)-1]
	}
	selectedServiceName, _ := c.PromptSelect("Choose ECS service", serviceNames, "", false)
	return selectedServiceName, nil
}

func maskTaskArn(taskArn string) string {
	if len(taskArn) <= 13 {
		return taskArn
	}
	return taskArn[:3] + strings.Repeat("*", len(taskArn)-13) + taskArn[len(taskArn)-10:]
}

func (c *Cli) SelectTask(ctx context.Context, client *ecs.Client, clusterArn, serviceName string) (string, error) {
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
			return "", err
		}
		taskArns = append(taskArns, output.TaskArns...)
		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}
	if len(taskArns) == 0 {
		return "", fmt.Errorf("no tasks found for service %s", serviceName)
	}
	maskedTaskArns := make([]string, len(taskArns))
	for i, arn := range taskArns {
		maskedTaskArns[i] = maskTaskArn(arn)
	}
	selectedMaskedTask, _ := c.PromptSelect("Choose ECS task", maskedTaskArns, "", false)
	for i, maskedArn := range maskedTaskArns {
		if maskedArn == selectedMaskedTask {
			return taskArns[i], nil
		}
	}
	return "", fmt.Errorf("selected task not found")
}

func (c *Cli) SelectContainer(ctx context.Context, client *ecs.Client, clusterArn, taskArn string) (string, error) {
	output, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &clusterArn,
		Tasks:   []string{taskArn},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe tasks: %w", err)
	}
	if len(output.Tasks) == 0 {
		return "", fmt.Errorf("no tasks found for ARN %s", taskArn)
	}
	task := output.Tasks[0]
	if len(task.Containers) == 0 {
		return "", fmt.Errorf("no containers found in task %s", taskArn)
	}
	containerNames := make([]string, 0, len(task.Containers))
	for _, cont := range task.Containers {
		if cont.Name != nil {
			containerNames = append(containerNames, *cont.Name)
		}
	}
	selectedContainer, _ := c.PromptSelect("Choose a container", containerNames, "", false)
	return selectedContainer, nil
}

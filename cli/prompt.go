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
	"github.com/manifoldco/promptui"
	"gopkg.in/ini.v1"
)

func (c *Cli) CheckSSOSession(ctx context.Context, client *sts.Client, profile string) error {
	_, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	return err
}

func (c *Cli) getStoredConfigPath() string {
	customPathFile := os.Getenv("HOME") + "/.aws/custom_config_path"
	if data, err := os.ReadFile(customPathFile); err == nil {
		return string(data)
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
		prompt := promptui.Prompt{
			Label:   "Enter AWS config file path",
			Default: awsConfigPath,
		}
		newPath, err := prompt.Run()
		if err != nil {
			log.Fatalf("Prompt failed: %v", err)
		}
		awsConfigPath = newPath

		if err := c.saveCustomConfigPath(newPath); err != nil {
			fmt.Printf("Warning: Failed to save custom config path: %v\n", err)
		}
	}

	cfg, err := ini.Load(awsConfigPath)
	if err != nil {
		c.LogUserFriendlyError(
			"Failed to load AWS config",
			err,
			"Ensure your AWS config file exists and is properly formatted.",
			awsConfigPath,
			92,
		)
	}

	profiles := []string{}
	for _, section := range cfg.Sections() {
		if strings.HasPrefix(section.Name(), "profile ") {
			profiles = append(profiles, strings.TrimPrefix(section.Name(), "profile "))
		}
	}

	if len(profiles) == 0 {
		log.Fatalf("No profiles found in AWS config. Ensure you have valid profiles set up in your AWS configuration file at %s.", awsConfigPath)
	}

	return c.PromptSelect("Choose AWS profile", profiles)
}

func (c *Cli) SelectCluster(ctx context.Context, client *ecs.Client) (string, error) {
	output, err := client.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return "", err
	}
	clusters := output.ClusterArns
	if len(clusters) == 0 {
		return "", fmt.Errorf("no clusters found")
	}
	return c.PromptSelect("Choose ECS cluster", clusters), nil
}

func (c *Cli) ListServices(ctx context.Context, client *ecs.Client, clusterArn string) ([]string, error) {
	var (
		services  []string
		nextToken *string
	)

	for {
		output, err := client.ListServices(ctx, &ecs.ListServicesInput{
			Cluster:   &clusterArn,
			NextToken: nextToken,
		})
		if err != nil {
			return nil, err
		}

		services = append(services, output.ServiceArns...)

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return services, nil
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
		return "", fmt.Errorf("no tasks found")
	}

	return c.PromptSelect("Choose ECS task", taskArns), nil
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
	for _, c := range task.Containers {
		if c.Name != nil {
			containerNames = append(containerNames, *c.Name)
		}
	}

	selectedName := c.PromptSelect("Choose a container", containerNames)
	return selectedName, nil
}

func (c *Cli) PromptSelect(label string, items []string) string {
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}
	_, result, err := prompt.Run()
	if err != nil {
		log.Fatalf("Prompt failed: %v", err)
	}
	return result
}

func (c *Cli) PromptWithDefault(label, defaultValue string, items []string) string {
	items = append([]string{fmt.Sprintf("%s (default)", defaultValue)}, items...)
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}
	_, result, err := prompt.Run()
	if err != nil {
		log.Fatalf("Prompt failed: %v", err)
	}
	if strings.HasSuffix(result, "(default)") {
		return defaultValue
	}
	return result
}

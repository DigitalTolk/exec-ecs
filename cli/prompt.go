package cli

import (
	"context"
	"fmt"
	"log"
	"os"
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

func (c *Cli) SelectProfile() string {
	awsConfigPath := os.Getenv("HOME") + "/.aws/config"
	cfg, err := ini.Load(awsConfigPath)
	if err != nil {
		c.LogUserFriendlyError("Failed to load AWS config", err, "Ensure your AWS config file exists and is properly formatted.", awsConfigPath, 92)
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
	output, err := client.ListServices(ctx, &ecs.ListServicesInput{Cluster: &clusterArn})
	if err != nil {
		return nil, err
	}
	return output.ServiceArns, nil
}

func (c *Cli) SelectTask(ctx context.Context, client *ecs.Client, clusterArn, serviceName string) (string, error) {
	output, err := client.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     &clusterArn,
		ServiceName: &serviceName,
	})
	if err != nil {
		return "", err
	}
	if len(output.TaskArns) == 0 {
		return "", fmt.Errorf("no tasks found")
	}
	return c.PromptSelect("Choose ECS task", output.TaskArns), nil
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

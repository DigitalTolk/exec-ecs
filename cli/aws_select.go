package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"gopkg.in/ini.v1"
)

// stsCallerIdentity is the minimal interface CheckSSOSession needs from the
// real STS client, captured so tests can supply a stub.
type stsCallerIdentity interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

func (c *Cli) CheckSSOSession(ctx context.Context, client stsCallerIdentity, profile string) error {
	_ = profile
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

// AWSConfigPath returns the path the tool should read for AWS profile data,
// preferring an explicit override stored alongside ~/.aws/config.
func (c *Cli) AWSConfigPath() string {
	if p := c.getStoredConfigPath(); p != "" {
		return p
	}
	return os.Getenv("HOME") + "/.aws/config"
}

// LookupSSOSessionForProfile reads the AWS shared config and returns the
// sso_session name configured for the given profile, if any. This lets the
// caller issue a single `aws sso login --sso-session <name>` that covers every
// profile bound to the same SSO session, instead of forcing the user to log in
// per-profile.
func (c *Cli) LookupSSOSessionForProfile(profile string) string {
	cfg, err := ini.Load(c.AWSConfigPath())
	if err != nil {
		return ""
	}
	section, err := cfg.GetSection("profile " + profile)
	if err != nil {
		return ""
	}
	if !section.HasKey("sso_session") {
		return ""
	}
	return strings.TrimSpace(section.Key("sso_session").String())
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
		if name, ok := strings.CutPrefix(section.Name(), "profile "); ok {
			profiles = append(profiles, name)
		}
	}
	if len(profiles) == 0 {
		log.Fatalf("No profiles found in AWS config: %s", awsConfigPath)
	}
	selectedProfile, _ := c.PromptSelect("Choose AWS profile", profiles, "", false)
	return selectedProfile
}

func (c *Cli) SelectCluster(ctx context.Context, client *ecs.Client) (string, error) {
	clusters, err := listAllClusterArns(ctx, client)
	if err != nil {
		return "", err
	}
	if len(clusters) == 0 {
		return "", errors.New("no ECS clusters found")
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
	services, err := listAllServiceArns(ctx, client, clusterArn)
	if err != nil {
		return "", err
	}
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
	taskArns, err := listAllTaskArns(ctx, client, clusterArn, serviceName)
	if err != nil {
		return "", err
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

// Small interfaces over the AWS SDK ECS client. We define them at the call
// site (instead of importing the SDK's huge surface) so tests can supply
// fakes without depending on the real SDK.

type ecsClusterLister interface {
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
}

type ecsServiceLister interface {
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
}

type ecsTaskLister interface {
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
}

type ecsTaskDescriber interface {
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

// listAllClusterArns paginates ECS ListClusters so users with more than the
// default page size of clusters still see every cluster.
func listAllClusterArns(ctx context.Context, client ecsClusterLister) ([]string, error) {
	var (
		arns      []string
		nextToken *string
	)
	for {
		out, err := client.ListClusters(ctx, &ecs.ListClustersInput{
			MaxResults: aws.Int32(100),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, err
		}
		arns = append(arns, out.ClusterArns...)
		if out.NextToken == nil || *out.NextToken == "" {
			return arns, nil
		}
		nextToken = out.NextToken
	}
}

func listAllServiceArns(ctx context.Context, client ecsServiceLister, clusterArn string) ([]string, error) {
	var (
		arns      []string
		nextToken *string
	)
	for {
		out, err := client.ListServices(ctx, &ecs.ListServicesInput{
			Cluster:    &clusterArn,
			MaxResults: aws.Int32(100),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, err
		}
		arns = append(arns, out.ServiceArns...)
		if out.NextToken == nil || *out.NextToken == "" {
			return arns, nil
		}
		nextToken = out.NextToken
	}
}

func listAllTaskArns(ctx context.Context, client ecsTaskLister, clusterArn, serviceName string) ([]string, error) {
	var (
		arns      []string
		nextToken *string
	)
	for {
		out, err := client.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:     &clusterArn,
			ServiceName: &serviceName,
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, err
		}
		arns = append(arns, out.TaskArns...)
		if out.NextToken == nil || *out.NextToken == "" {
			return arns, nil
		}
		nextToken = out.NextToken
	}
}

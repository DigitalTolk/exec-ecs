package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
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
	customPathFile := filepath.Join(homeDir(), ".aws", "custom_config_path")
	if data, err := os.ReadFile(customPathFile); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func (c *Cli) saveCustomConfigPath(path string) error {
	customPathFile := filepath.Join(homeDir(), ".aws", "custom_config_path")
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
	return filepath.Join(homeDir(), ".aws", "config")
}

func maskTaskArn(taskArn string) string {
	if len(taskArn) <= 13 {
		return taskArn
	}
	return taskArn[:3] + strings.Repeat("*", len(taskArn)-13) + taskArn[len(taskArn)-10:]
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

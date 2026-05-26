package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func TestMaskTaskArn(t *testing.T) {
	t.Parallel()

	long := "arn:aws:ecs:us-east-1:123456789012:task/foo/abcdef0123456789"
	tests := []struct {
		name string
		arn  string
		want string
	}{
		{"short returned verbatim", "short", "short"},
		{"boundary 13 chars verbatim", "1234567890123", "1234567890123"},
		{"masks middle of long arn",
			long,
			long[:3] + strings.Repeat("*", len(long)-13) + long[len(long)-10:]},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := maskTaskArn(tc.arn); got != tc.want {
				t.Fatalf("maskTaskArn(%q) = %q want %q", tc.arn, got, tc.want)
			}
		})
	}
}

type fakeECS struct {
	clustersPages   [][]string
	clusterErr      error
	clusterCalls    int
	servicesPages   [][]string
	serviceErr      error
	serviceCalls    int
	tasksPages      [][]string
	taskErr         error
	taskCalls       int
	describeTasks   []ecstypes.Task
	describeTaskErr error
}

func (f *fakeECS) ListClusters(ctx context.Context, params *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	if f.clusterErr != nil {
		return nil, f.clusterErr
	}
	idx := f.clusterCalls
	f.clusterCalls++
	if idx >= len(f.clustersPages) {
		return &ecs.ListClustersOutput{}, nil
	}
	var next *string
	if idx+1 < len(f.clustersPages) {
		tok := "tok"
		next = &tok
	}
	return &ecs.ListClustersOutput{ClusterArns: f.clustersPages[idx], NextToken: next}, nil
}

func (f *fakeECS) ListServices(ctx context.Context, params *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	if f.serviceErr != nil {
		return nil, f.serviceErr
	}
	idx := f.serviceCalls
	f.serviceCalls++
	if idx >= len(f.servicesPages) {
		return &ecs.ListServicesOutput{}, nil
	}
	var next *string
	if idx+1 < len(f.servicesPages) {
		tok := "tok"
		next = &tok
	}
	return &ecs.ListServicesOutput{ServiceArns: f.servicesPages[idx], NextToken: next}, nil
}

func (f *fakeECS) ListTasks(ctx context.Context, params *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	if f.taskErr != nil {
		return nil, f.taskErr
	}
	idx := f.taskCalls
	f.taskCalls++
	if idx >= len(f.tasksPages) {
		return &ecs.ListTasksOutput{}, nil
	}
	var next *string
	if idx+1 < len(f.tasksPages) {
		tok := "tok"
		next = &tok
	}
	return &ecs.ListTasksOutput{TaskArns: f.tasksPages[idx], NextToken: next}, nil
}

func (f *fakeECS) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	if f.describeTaskErr != nil {
		return nil, f.describeTaskErr
	}
	return &ecs.DescribeTasksOutput{Tasks: f.describeTasks}, nil
}

func TestListAllClusterArnsPaginates(t *testing.T) {
	t.Parallel()

	f := &fakeECS{clustersPages: [][]string{{"a", "b"}, {"c"}}}
	arns, err := listAllClusterArns(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(arns, want) {
		t.Fatalf("listAllClusterArns = %v want %v", arns, want)
	}
	if f.clusterCalls != 2 {
		t.Fatalf("want 2 page calls, got %d", f.clusterCalls)
	}
}

func TestListAllClusterArnsError(t *testing.T) {
	t.Parallel()

	f := &fakeECS{clusterErr: errors.New("boom")}
	if _, err := listAllClusterArns(context.Background(), f); err == nil {
		t.Fatal("expected error")
	}
}

func TestListAllServiceArnsPaginates(t *testing.T) {
	t.Parallel()

	f := &fakeECS{servicesPages: [][]string{{"s1"}, {"s2", "s3"}}}
	arns, err := listAllServiceArns(context.Background(), f, "cluster")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []string{"s1", "s2", "s3"}
	if !reflect.DeepEqual(arns, want) {
		t.Fatalf("got %v want %v", arns, want)
	}
}

func TestListAllServiceArnsError(t *testing.T) {
	t.Parallel()

	f := &fakeECS{serviceErr: errors.New("boom")}
	if _, err := listAllServiceArns(context.Background(), f, "cluster"); err == nil {
		t.Fatal("expected error")
	}
}

func TestListAllTaskArnsPaginates(t *testing.T) {
	t.Parallel()

	f := &fakeECS{tasksPages: [][]string{{"t1"}, {"t2"}}}
	arns, err := listAllTaskArns(context.Background(), f, "c", "s")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []string{"t1", "t2"}
	if !reflect.DeepEqual(arns, want) {
		t.Fatalf("got %v want %v", arns, want)
	}
}

func TestListAllTaskArnsError(t *testing.T) {
	t.Parallel()

	f := &fakeECS{taskErr: errors.New("boom")}
	if _, err := listAllTaskArns(context.Background(), f, "c", "s"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCliListClusterNamesArns(t *testing.T) {
	t.Parallel()
	c := &Cli{}
	f := &fakeECS{clustersPages: [][]string{{
		"arn:aws:ecs:us-east-1:111111111111:cluster/foo",
		"arn:aws:ecs:us-east-1:111111111111:cluster/bar",
	}}}
	names, m, err := c.ListClusterNamesArns(context.Background(), f)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !reflect.DeepEqual(names, []string{"foo", "bar"}) {
		t.Fatalf("names = %v", names)
	}
	if m["foo"] == "" || m["bar"] == "" {
		t.Fatalf("map = %v", m)
	}

	errf := &fakeECS{clusterErr: errors.New("boom")}
	if _, _, err := c.ListClusterNamesArns(context.Background(), errf); err == nil {
		t.Fatal("expected error")
	}
}

func TestCliListServiceNamesArns(t *testing.T) {
	t.Parallel()
	c := &Cli{}
	f := &fakeECS{servicesPages: [][]string{{
		"arn:aws:ecs:us-east-1:1:service/cluster1/svc-a",
		"arn:aws:ecs:us-east-1:1:service/cluster1/svc-b",
	}}}
	names, m, err := c.ListServiceNamesArns(context.Background(), f, "cluster1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !reflect.DeepEqual(names, []string{"svc-a", "svc-b"}) {
		t.Fatalf("names = %v", names)
	}
	if m["svc-a"] == "" {
		t.Fatalf("svc-a missing")
	}
	if _, _, err := c.ListServiceNamesArns(context.Background(), &fakeECS{serviceErr: errors.New("boom")}, "cluster"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCliListTaskNamesArns(t *testing.T) {
	t.Parallel()
	c := &Cli{}
	f := &fakeECS{tasksPages: [][]string{{
		"arn:aws:ecs:us-east-1:111111111111:task/cluster/abcdef0123456789",
	}}}
	names, m, err := c.ListTaskNamesArns(context.Background(), f, "cluster", "svc")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(names) != 1 {
		t.Fatalf("names = %v", names)
	}
	if !strings.Contains(names[0], "*") {
		t.Fatalf("expected masked task name, got %q", names[0])
	}
	if m[names[0]] == "" {
		t.Fatalf("masked->arn missing")
	}
	if _, _, err := c.ListTaskNamesArns(context.Background(), &fakeECS{taskErr: errors.New("boom")}, "c", "s"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCliListContainerNames(t *testing.T) {
	t.Parallel()
	c := &Cli{}
	mainName, sidecar := "main", "sidecar"
	f := &fakeECS{describeTasks: []ecstypes.Task{{
		Containers: []ecstypes.Container{
			{Name: &mainName},
			{Name: &sidecar},
			{Name: nil},
		},
	}}}
	names, err := c.ListContainerNames(context.Background(), f, "cluster", "task")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !reflect.DeepEqual(names, []string{"main", "sidecar"}) {
		t.Fatalf("names = %v", names)
	}

	// No tasks returned -> nil names.
	emptyF := &fakeECS{}
	if names, err := c.ListContainerNames(context.Background(), emptyF, "c", "t"); err != nil || names != nil {
		t.Fatalf("expected nil names with no error, got %v %v", names, err)
	}

	// Error path.
	errF := &fakeECS{describeTaskErr: errors.New("boom")}
	if _, err := c.ListContainerNames(context.Background(), errF, "c", "t"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLookupSSOSessionForProfile(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config")
	body := `
[profile foo]
sso_session = main
sso_account_id = 1
sso_role_name = R
region = us-east-1

[profile bar]
region = eu-west-1

[sso-session main]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
`
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	customPathFile := filepath.Join(tmp, "custom_path")
	t.Setenv("HOME", tmp)
	if err := os.MkdirAll(filepath.Join(tmp, ".aws"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".aws", "custom_config_path"), []byte(cfgPath), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = customPathFile

	c := &Cli{}
	if got := c.LookupSSOSessionForProfile("foo"); got != "main" {
		t.Fatalf("foo sso = %q want main", got)
	}
	if got := c.LookupSSOSessionForProfile("bar"); got != "" {
		t.Fatalf("bar sso = %q want empty", got)
	}
	if got := c.LookupSSOSessionForProfile("missing"); got != "" {
		t.Fatalf("missing sso = %q want empty", got)
	}
}

func TestAWSConfigPathFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	c := &Cli{}
	if got := c.AWSConfigPath(); got != filepath.Join(tmp, ".aws/config") {
		t.Fatalf("default AWSConfigPath = %q", got)
	}

	if err := os.MkdirAll(filepath.Join(tmp, ".aws"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".aws", "custom_config_path"), []byte("/tmp/other"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := c.AWSConfigPath(); got != "/tmp/other" {
		t.Fatalf("override AWSConfigPath = %q", got)
	}
}

func TestSelectProfileList(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	awsDir := filepath.Join(tmp, ".aws")
	if err := os.MkdirAll(awsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `
[profile alpha]
region = us-east-1

[profile beta]
region = eu-west-1

[default]
region = us-east-1
`
	if err := os.WriteFile(filepath.Join(awsDir, "config"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	c := &Cli{}
	got := c.SelectProfileList()
	want := map[string]bool{"alpha": true, "beta": true}
	if len(got) != len(want) {
		t.Fatalf("profiles = %v want size %d", got, len(want))
	}
	for _, p := range got {
		if !want[p] {
			t.Fatalf("unexpected profile %q", p)
		}
	}
}

func TestSelectProfileListMissingConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	c := &Cli{}
	if got := c.SelectProfileList(); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestSaveCustomConfigPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	c := &Cli{}
	if err := c.saveCustomConfigPath("/some/where"); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".aws", "custom_config_path"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "/some/where" {
		t.Fatalf("contents = %q", string(data))
	}

	if c.getStoredConfigPath() != "/some/where" {
		t.Fatalf("getStoredConfigPath mismatch")
	}
}

func TestDefaultECSClientShim(t *testing.T) {
	t.Parallel()
	// Compile-time guarantee that *ecs.Client satisfies our small interfaces.
	var _ ecsClusterLister = (*ecs.Client)(nil)
	var _ ecsServiceLister = (*ecs.Client)(nil)
	var _ ecsTaskLister = (*ecs.Client)(nil)
	var _ ecsTaskDescriber = (*ecs.Client)(nil)
	_ = aws.Int32(0)
}

type fakeSTSCaller struct {
	err error
}

func (f *fakeSTSCaller) GetCallerIdentity(ctx context.Context, _ *sts.GetCallerIdentityInput, _ ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{}, f.err
}

func TestCheckSSOSession(t *testing.T) {
	t.Parallel()
	c := &Cli{}
	if err := c.CheckSSOSession(context.Background(), &fakeSTSCaller{}, "prof"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := c.CheckSSOSession(context.Background(), &fakeSTSCaller{err: errors.New("expired")}, "prof"); err == nil {
		t.Fatal("expected error")
	}
}

package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func awsZeroCfg() aws.Config { return aws.Config{} }

// stubSelector deterministically returns canned answers, in order. The last
// answer is reused for any further calls — handy for tests where you don't
// want to count picks exactly.
type stubSelector struct {
	answers []stubAnswer
	calls   int
}

type stubAnswer struct {
	selection string
	goBack    bool
}

func (s *stubSelector) Select(_ string, items []string, _ string, _ bool) (string, bool) {
	idx := s.calls
	if idx >= len(s.answers) {
		idx = len(s.answers) - 1
	}
	s.calls++
	a := s.answers[idx]
	if a.goBack {
		return "", true
	}
	// If the canned selection isn't in the offered items, return it anyway
	// so the test can verify behaviour against arbitrary strings.
	_ = items
	return a.selection, false
}

func TestSelectorFuncImplementsSelector(t *testing.T) {
	t.Parallel()
	var sel Selector = SelectorFunc(func(_ string, _ []string, _ string, _ bool) (string, bool) {
		return "x", false
	})
	v, back := sel.Select("l", nil, "", false)
	if v != "x" || back {
		t.Fatalf("got %q %v", v, back)
	}
}

func TestCliSelectorReturnsCallable(t *testing.T) {
	t.Parallel()
	c := &Cli{}
	if c.CliSelector() == nil {
		t.Fatal("nil selector")
	}
}

func TestResetState(t *testing.T) {
	t.Parallel()
	full := State{Region: "r", ClusterArn: "c", Service: "s", TaskArn: "t", Container: "k"}

	s := full
	resetState(&s, stepIdxContainer)
	if s.TaskArn == "" || s.Container != "" {
		t.Fatalf("container reset wrong: %+v", s)
	}

	s = full
	resetState(&s, stepIdxTask)
	if s.TaskArn != "" || s.Container != "" || s.Service == "" {
		t.Fatalf("task reset wrong: %+v", s)
	}

	s = full
	resetState(&s, stepIdxCluster)
	if s.ClusterArn != "" || s.Service != "" || s.TaskArn != "" || s.Container != "" {
		t.Fatalf("cluster reset wrong: %+v", s)
	}
	if s.Region == "" {
		t.Fatalf("region should survive cluster reset: %+v", s)
	}

	s = full
	resetState(&s, stepIdxRegion)
	if s.Region != "" {
		t.Fatalf("region should clear: %+v", s)
	}
}

func TestKeyForValue(t *testing.T) {
	t.Parallel()
	m := map[string]string{"a": "1", "b": "2"}
	if got := keyForValue(m, "2"); got != "b" {
		t.Fatalf("got %q", got)
	}
	if got := keyForValue(m, "missing"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestPickClusterAdvances(t *testing.T) {
	c := &Cli{Profile: "p", Region: "us-east-1"}
	f := &fakeECS{clustersPages: [][]string{{"arn:aws:ecs:us-east-1:1:cluster/foo"}}}
	sel := &stubSelector{answers: []stubAnswer{{selection: "foo"}}}

	out, action, err := PickCluster(context.Background(), c, sel, f, State{Region: "us-east-1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionAdvance {
		t.Fatalf("action = %v", action)
	}
	if out.ClusterArn == "" || c.ClusterArn != out.ClusterArn {
		t.Fatalf("cluster not set: %+v", out)
	}
}

func TestPickClusterGoBack(t *testing.T) {
	c := &Cli{}
	f := &fakeECS{clustersPages: [][]string{{"arn:aws:ecs:us-east-1:1:cluster/foo"}}}
	sel := &stubSelector{answers: []stubAnswer{{goBack: true}}}

	out, action, err := PickCluster(context.Background(), c, sel, f, State{Region: "r", ClusterArn: "old"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
	if out.ClusterArn != "" {
		t.Fatalf("cluster should have been cleared: %+v", out)
	}
}

func TestPickClusterNoneFoundBack(t *testing.T) {
	setRegionCacheFile(t)
	c := &Cli{Profile: "p", Region: "r"}
	f := &fakeECS{} // no clusters
	sel := &stubSelector{answers: []stubAnswer{{selection: "Back"}}}

	out, action, err := PickCluster(context.Background(), c, sel, f, State{Region: "r"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
	_ = out
}

func TestPickClusterNoneFoundRetry(t *testing.T) {
	c := &Cli{Profile: "p", Region: "r"}
	f := &fakeECS{} // no clusters
	sel := &stubSelector{answers: []stubAnswer{{selection: "Retry"}}}

	_, action, err := PickCluster(context.Background(), c, sel, f, State{Region: "r"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionRetry {
		t.Fatalf("action = %v", action)
	}
}

func TestPickClusterLookupFailedBack(t *testing.T) {
	c := &Cli{Profile: "p", Region: "r"}
	f := &fakeECS{clusterErr: errors.New("denied")}
	sel := &stubSelector{answers: []stubAnswer{{selection: "Back"}}}

	_, action, err := PickCluster(context.Background(), c, sel, f, State{Region: "r"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
}

func TestPickClusterLookupFailedRetry(t *testing.T) {
	c := &Cli{Profile: "p", Region: "r"}
	f := &fakeECS{clusterErr: errors.New("denied")}
	sel := &stubSelector{answers: []stubAnswer{{selection: "Retry"}}}

	_, action, err := PickCluster(context.Background(), c, sel, f, State{Region: "r"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionRetry {
		t.Fatalf("action = %v", action)
	}
}

func TestPickServiceAdvances(t *testing.T) {
	c := &Cli{Profile: "p", Region: "r"}
	f := &fakeECS{servicesPages: [][]string{{"arn:aws:ecs:::service/c/svc1"}}}
	sel := &stubSelector{answers: []stubAnswer{{selection: "svc1"}}}

	out, action, err := PickService(context.Background(), c, sel, f, State{ClusterArn: "c"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionAdvance {
		t.Fatalf("action = %v", action)
	}
	if out.Service == "" || c.Service == "" {
		t.Fatalf("service not set: %+v", out)
	}
}

func TestPickServiceError(t *testing.T) {
	c := &Cli{}
	f := &fakeECS{serviceErr: errors.New("boom")}
	sel := &stubSelector{answers: []stubAnswer{{selection: "x"}}}
	_, action, err := PickService(context.Background(), c, sel, f, State{ClusterArn: "c"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
}

func TestPickServiceEmpty(t *testing.T) {
	c := &Cli{}
	f := &fakeECS{}
	sel := &stubSelector{answers: []stubAnswer{{selection: "x"}}}
	_, action, err := PickService(context.Background(), c, sel, f, State{ClusterArn: "c"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
}

func TestPickServiceGoBack(t *testing.T) {
	c := &Cli{}
	f := &fakeECS{servicesPages: [][]string{{"arn:aws:ecs:::service/c/svc1"}}}
	sel := &stubSelector{answers: []stubAnswer{{goBack: true}}}
	_, action, err := PickService(context.Background(), c, sel, f, State{ClusterArn: "c"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
}

func TestPickTaskAdvances(t *testing.T) {
	c := &Cli{Profile: "p", Region: "r"}
	f := &fakeECS{tasksPages: [][]string{{"arn:aws:ecs:us-east-1:111111111111:task/cluster/abcdef0123456789"}}}
	// The picker exposes the masked task ARN; emulate that by selecting
	// whatever it offers via the stubSelector indirection: we use the
	// "first item" trick by passing the same string maskTaskArn produces.
	masked := maskTaskArn("arn:aws:ecs:us-east-1:111111111111:task/cluster/abcdef0123456789")
	sel := &stubSelector{answers: []stubAnswer{{selection: masked}}}
	out, action, err := PickTask(context.Background(), c, sel, f, State{ClusterArn: "c", Service: "s"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionAdvance {
		t.Fatalf("action = %v", action)
	}
	if out.TaskArn == "" {
		t.Fatal("task arn not set")
	}
}

func TestPickTaskError(t *testing.T) {
	c := &Cli{}
	f := &fakeECS{taskErr: errors.New("boom")}
	sel := &stubSelector{answers: []stubAnswer{{selection: "x"}}}
	_, action, _ := PickTask(context.Background(), c, sel, f, State{ClusterArn: "c", Service: "s"})
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
}

func TestPickTaskEmpty(t *testing.T) {
	c := &Cli{}
	f := &fakeECS{}
	sel := &stubSelector{answers: []stubAnswer{{selection: "x"}}}
	_, action, _ := PickTask(context.Background(), c, sel, f, State{ClusterArn: "c", Service: "s"})
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
}

func TestPickTaskGoBack(t *testing.T) {
	c := &Cli{}
	f := &fakeECS{tasksPages: [][]string{{"arn:aws:ecs:us-east-1:111111111111:task/cluster/abcdef0123456789"}}}
	sel := &stubSelector{answers: []stubAnswer{{goBack: true}}}
	_, action, _ := PickTask(context.Background(), c, sel, f, State{ClusterArn: "c", Service: "s"})
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
}

func TestPickContainerAdvances(t *testing.T) {
	c := &Cli{Profile: "p", Region: "r"}
	mainName := "main"
	f := &fakeECS{describeTasks: []ecstypes.Task{{Containers: []ecstypes.Container{{Name: &mainName}}}}}
	sel := &stubSelector{answers: []stubAnswer{{selection: "main"}}}
	out, action, err := PickContainer(context.Background(), c, sel, f, State{ClusterArn: "c", TaskArn: "t"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if action != ActionAdvance {
		t.Fatalf("action = %v", action)
	}
	if out.Container != "main" || c.Container != "main" {
		t.Fatalf("container not set: %+v", out)
	}
}

func TestPickContainerError(t *testing.T) {
	c := &Cli{}
	f := &fakeECS{describeTaskErr: errors.New("boom")}
	sel := &stubSelector{answers: []stubAnswer{{selection: "x"}}}
	_, action, _ := PickContainer(context.Background(), c, sel, f, State{ClusterArn: "c", TaskArn: "t"})
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
}

func TestPickContainerEmpty(t *testing.T) {
	c := &Cli{}
	f := &fakeECS{describeTasks: []ecstypes.Task{{Containers: nil}}}
	sel := &stubSelector{answers: []stubAnswer{{selection: "x"}}}
	_, action, _ := PickContainer(context.Background(), c, sel, f, State{ClusterArn: "c", TaskArn: "t"})
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
}

func TestPickContainerGoBack(t *testing.T) {
	c := &Cli{}
	mainName := "main"
	f := &fakeECS{describeTasks: []ecstypes.Task{{Containers: []ecstypes.Container{{Name: &mainName}}}}}
	sel := &stubSelector{answers: []stubAnswer{{goBack: true}}}
	_, action, _ := PickContainer(context.Background(), c, sel, f, State{ClusterArn: "c", TaskArn: "t"})
	if action != ActionBack {
		t.Fatalf("action = %v", action)
	}
}

func TestNewECSClientReturnsConcrete(t *testing.T) {
	t.Parallel()
	// Constructor must accept a zero config and produce a non-nil client.
	client := NewECSClient(awsZeroCfg(), "us-east-1")
	if _, ok := client.(*ecs.Client); !ok {
		t.Fatalf("expected *ecs.Client, got %T", client)
	}
}

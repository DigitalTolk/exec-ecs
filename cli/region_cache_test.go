package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func setRegionCacheFile(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "region_cache.json")
	t.Setenv("EXEC_ECS_REGION_CACHE_PATH", path)
	return path
}

func TestRegionCacheRoundTrip(t *testing.T) {
	setRegionCacheFile(t)

	if _, ok := LookupCachedRegions("missing"); ok {
		t.Fatal("empty cache should miss")
	}

	if err := StoreCachedRegions("p1", []string{"us-east-1", "eu-west-1"}); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, ok := LookupCachedRegions("p1")
	if !ok {
		t.Fatal("expected hit")
	}
	if !reflect.DeepEqual(got, []string{"us-east-1", "eu-west-1"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRegionCacheExpires(t *testing.T) {
	setRegionCacheFile(t)
	prev := RegionCacheTTL
	RegionCacheTTL = 1 * time.Millisecond
	t.Cleanup(func() { RegionCacheTTL = prev })

	if err := StoreCachedRegions("p1", []string{"us-east-1"}); err != nil {
		t.Fatalf("store: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, ok := LookupCachedRegions("p1"); ok {
		t.Fatal("expected expired miss")
	}
}

func TestClearRegionCache(t *testing.T) {
	setRegionCacheFile(t)
	_ = StoreCachedRegions("p1", []string{"us-east-1"})
	ClearRegionCache("p1")
	if _, ok := LookupCachedRegions("p1"); ok {
		t.Fatal("expected miss after clear")
	}
}

func TestLoadRegionCacheCorrupt(t *testing.T) {
	path := setRegionCacheFile(t)
	if err := writeFile(path, "not json"); err != nil {
		t.Fatal(err)
	}
	c := loadRegionCache()
	if c == nil || c.Profiles == nil {
		t.Fatal("expected non-nil empty cache")
	}
}

func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o600)
}

func TestDiscoverRegionsWithClusters(t *testing.T) {
	setRegionCacheFile(t)

	var calls int32
	prevProber := regionProber
	regionProber = func(ctx context.Context, profile, region string) (bool, error) {
		atomic.AddInt32(&calls, 1)
		return region == "us-east-1" || region == "eu-west-1", nil
	}
	t.Cleanup(func() { regionProber = prevProber })

	regions, err := DiscoverRegionsWithClusters(context.Background(), "p", []string{"us-east-1", "us-west-2", "eu-west-1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	sort.Strings(regions)
	want := []string{"eu-west-1", "us-east-1"}
	if !reflect.DeepEqual(regions, want) {
		t.Fatalf("got %v want %v", regions, want)
	}

	// Second call should hit the cache and not invoke the prober again.
	prevCalls := atomic.LoadInt32(&calls)
	_, err = DiscoverRegionsWithClusters(context.Background(), "p", []string{"us-east-1", "us-west-2", "eu-west-1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if atomic.LoadInt32(&calls) != prevCalls {
		t.Fatalf("expected cache hit, prober called again")
	}
}

func TestDiscoverRegionsAllFail(t *testing.T) {
	setRegionCacheFile(t)
	prevProber := regionProber
	regionProber = func(ctx context.Context, profile, region string) (bool, error) {
		return false, errors.New("denied")
	}
	t.Cleanup(func() { regionProber = prevProber })

	_, err := DiscoverRegionsWithClusters(context.Background(), "p2", []string{"us-east-1", "eu-west-1"})
	if err == nil {
		t.Fatal("expected error when every probe fails")
	}
}

func TestDiscoverRegionsEmpty(t *testing.T) {
	setRegionCacheFile(t)
	prevProber := regionProber
	regionProber = func(ctx context.Context, profile, region string) (bool, error) {
		return false, nil
	}
	t.Cleanup(func() { regionProber = prevProber })

	regions, err := DiscoverRegionsWithClusters(context.Background(), "p3", []string{"us-east-1", "eu-west-1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(regions) != 0 {
		t.Fatalf("expected empty, got %v", regions)
	}
}

func TestRegionCachePathDefault(t *testing.T) {
	tmp := t.TempDir()
	prev := configDirOverride
	configDirOverride = tmp
	t.Cleanup(func() { configDirOverride = prev })
	t.Setenv("EXEC_ECS_REGION_CACHE_PATH", "")
	want := filepath.Join(tmp, "region-cache.json")
	if got := regionCachePath(); got != want {
		t.Fatalf("default path = %q want %q", got, want)
	}
}

func TestDefaultRegionProberFailsWithoutCreds(t *testing.T) {
	// Force LoadDefaultConfig to choose a region that doesn't resolve, with
	// a non-existent profile, so the function takes its error branch.
	prev := baseAWSConfigForProbe
	baseAWSConfigForProbe = func(_ context.Context, _ string) (aws.Config, error) {
		return aws.Config{}, errors.New("no creds")
	}
	t.Cleanup(func() {
		baseAWSConfigForProbe = prev
		probeBaseConfig = &probeConfigCache{} // reset shared state
	})

	if _, err := defaultRegionProber(context.Background(), "no-such-profile", "us-east-1"); err == nil {
		t.Fatal("expected error when base config load fails")
	}
}

func TestDefaultRegionsNonEmpty(t *testing.T) {
	t.Parallel()
	if len(DefaultRegions) == 0 {
		t.Fatal("DefaultRegions empty")
	}
}

func TestProbeConfigCacheGetSet(t *testing.T) {
	c := &probeConfigCache{}
	if _, ok := c.get("any"); ok {
		t.Fatal("empty cache should miss")
	}

	prev := baseAWSConfigForProbe
	t.Cleanup(func() { baseAWSConfigForProbe = prev })

	calls := 0
	baseAWSConfigForProbe = func(_ context.Context, _ string) (aws.Config, error) {
		calls++
		return aws.Config{}, nil
	}

	c.set(context.Background(), "p1")
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
	// Same profile → cached.
	c.set(context.Background(), "p1")
	if calls != 1 {
		t.Fatalf("expected still 1 call after re-set, got %d", calls)
	}
	if _, ok := c.get("p1"); !ok {
		t.Fatal("get(p1) miss after set")
	}
	if _, ok := c.get("p2"); ok {
		t.Fatal("get(p2) hit but never set")
	}
	// Different profile → reload.
	c.set(context.Background(), "p2")
	if calls != 2 {
		t.Fatalf("expected 2 calls after profile switch, got %d", calls)
	}
}


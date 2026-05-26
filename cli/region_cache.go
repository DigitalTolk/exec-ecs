package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// DefaultRegions is the canonical list of AWS regions the tool probes when
// discovering which regions contain ECS clusters for a given profile.
var DefaultRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"af-south-1",
	"ap-east-1", "ap-south-1", "ap-south-2",
	"ap-southeast-1", "ap-southeast-2", "ap-southeast-3", "ap-southeast-4",
	"ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
	"ca-central-1",
	"eu-central-1", "eu-central-2",
	"eu-west-1", "eu-west-2", "eu-west-3",
	"eu-north-1",
	"eu-south-1", "eu-south-2",
	"me-south-1", "me-central-1",
	"sa-east-1",
}

// RegionCacheTTL is how long discovered regions remain valid before we
// re-probe. Exposed as a var so tests can shorten it.
var RegionCacheTTL = 5 * time.Minute

type regionCacheEntry struct {
	Regions   []string  `json:"regions"`
	UpdatedAt time.Time `json:"updated_at"`
}

type regionCacheFile struct {
	Profiles map[string]regionCacheEntry `json:"profiles"`
}

// regionCachePath returns the cache file path. Overridable via env for tests.
func regionCachePath() string { return regionCacheFilePath() }

func loadRegionCache() *regionCacheFile {
	cache := &regionCacheFile{Profiles: map[string]regionCacheEntry{}}
	data, err := os.ReadFile(regionCachePath())
	if err != nil {
		return cache
	}
	if err := json.Unmarshal(data, cache); err != nil || cache.Profiles == nil {
		return &regionCacheFile{Profiles: map[string]regionCacheEntry{}}
	}
	return cache
}

func saveRegionCache(cache *regionCacheFile) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	path := regionCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// Write to a tempfile in the same directory and rename into place so
	// concurrent invocations cannot read a half-written file, and an
	// interrupted write cannot leave behind a corrupt cache.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".region-cache-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// LookupCachedRegions returns the cached regions for a profile if still valid.
func LookupCachedRegions(profile string) ([]string, bool) {
	cache := loadRegionCache()
	entry, ok := cache.Profiles[profile]
	if !ok {
		return nil, false
	}
	if time.Since(entry.UpdatedAt) > RegionCacheTTL {
		return nil, false
	}
	return entry.Regions, true
}

// StoreCachedRegions writes the regions for a profile to the on-disk cache.
func StoreCachedRegions(profile string, regions []string) error {
	cache := loadRegionCache()
	cache.Profiles[profile] = regionCacheEntry{
		Regions:   regions,
		UpdatedAt: time.Now(),
	}
	return saveRegionCache(cache)
}

// ClearRegionCache removes any cached region data for the profile.
func ClearRegionCache(profile string) {
	cache := loadRegionCache()
	delete(cache.Profiles, profile)
	_ = saveRegionCache(cache)
}

// regionProber is the function used to probe a region. Overridable in tests.
var regionProber = defaultRegionProber

// probeTimeout caps how long any single region probe is allowed to take. A
// stuck endpoint must not hold the entire discovery hostage; the user can
// always retry. Tuned generous enough for cold cross-region TLS handshakes
// but tight enough that the whole sweep completes in well under a minute.
var probeTimeout = 5 * time.Second

// probeConcurrency is the maximum number of in-flight ListClusters calls. AWS
// SDK clients are cheap to clone per-region — what's expensive is sequential
// network. With 27 default regions, 12-way fan-out keeps the worst case under
// ~3 × probeTimeout.
var probeConcurrency = 12

// baseAWSConfigForProbe loads the AWS shared-config-derived credentials once
// per discovery sweep. We then clone it per-region instead of re-parsing the
// config (and re-touching the SSO token cache) on every probe.
//
// Exposed as a var for tests.
var baseAWSConfigForProbe = func(ctx context.Context, profile string) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		// Region is set per probe; we use a placeholder so config-load
		// doesn't bail when the profile omits a default region.
		config.WithRegion("us-east-1"),
	)
}

// probeBaseConfig is a per-sweep cache of the loaded AWS config. Without this
// every region probe re-parses ~/.aws/config from disk, multiplying total
// discovery time by N.
type probeConfigCache struct {
	mu      sync.Mutex
	cfg     aws.Config
	profile string
	loaded  bool
}

var probeBaseConfig = &probeConfigCache{}

func (c *probeConfigCache) set(ctx context.Context, profile string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded && c.profile == profile {
		return
	}
	cfg, err := baseAWSConfigForProbe(ctx, profile)
	if err != nil {
		c.loaded = false
		return
	}
	c.cfg = cfg
	c.profile = profile
	c.loaded = true
}

func (c *probeConfigCache) get(profile string) (aws.Config, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded && c.profile == profile {
		return c.cfg, true
	}
	return aws.Config{}, false
}

func defaultRegionProber(ctx context.Context, profile, region string) (bool, error) {
	cfg, ok := probeBaseConfig.get(profile)
	if !ok {
		var err error
		cfg, err = baseAWSConfigForProbe(ctx, profile)
		if err != nil {
			return false, err
		}
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	client := ecs.NewFromConfig(cfg, func(o *ecs.Options) {
		o.Region = region
		// One try per probe — retrying a region that's already this slow
		// only makes the discovery total worse.
		o.RetryMaxAttempts = 1
	})
	out, err := client.ListClusters(probeCtx, &ecs.ListClustersInput{
		MaxResults: aws.Int32(1),
	})
	if err != nil {
		return false, err
	}
	return len(out.ClusterArns) > 0, nil
}

// DiscoverRegionsWithClusters probes the candidate regions in parallel and
// returns those that hold at least one ECS cluster. The result is cached for
// RegionCacheTTL.
//
// Concurrency is bounded by probeConcurrency, and the semaphore is acquired
// *before* the goroutine starts so we don't eagerly build N AWS clients and
// N outstanding requests on a slow connection. Context cancellation is
// honoured promptly.
func DiscoverRegionsWithClusters(ctx context.Context, profile string, candidates []string) ([]string, error) {
	if cached, ok := LookupCachedRegions(profile); ok {
		return cached, nil
	}

	// Pre-warm the shared AWS config so the per-region probes don't each
	// re-parse ~/.aws/config from disk. Failures here mean we'll fall back
	// to defaultRegionProber's own config load (cheaper to skip the
	// optimisation than to fail discovery entirely).
	probeBaseConfig.set(ctx, profile)

	var (
		mu       sync.Mutex
		found    []string
		errCount int
		lastErr  error
		wg       sync.WaitGroup
	)

	sem := make(chan struct{}, probeConcurrency)
	for _, region := range candidates {
		// Acquire the slot first so we never have more than `cap(sem)` goroutines
		// in flight at once. Honour cancellation here so a Ctrl-C aborts the
		// remaining queue without spinning up more work.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return nil, ctx.Err()
		}

		wg.Add(1)
		go func(r string) {
			defer wg.Done()
			defer func() { <-sem }()

			// Re-check before doing work in case we were cancelled while queued.
			if ctx.Err() != nil {
				return
			}

			has, err := regionProber(ctx, profile, r)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errCount++
				lastErr = err
				return
			}
			if has {
				found = append(found, r)
			}
		}(region)
	}
	wg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if len(found) == 0 && errCount == len(candidates) && lastErr != nil {
		return nil, errors.New("unable to probe any region: " + lastErr.Error())
	}

	sort.Strings(found)
	_ = StoreCachedRegions(profile, found)
	return found, nil
}

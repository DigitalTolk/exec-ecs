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
func regionCachePath() string {
	if v := os.Getenv("EXEC_ECS_REGION_CACHE_PATH"); v != "" {
		return v
	}
	return filepath.Join(homeDir(), ".exec-ecs-region-cache.json")
}

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

func defaultRegionProber(ctx context.Context, profile, region string) (bool, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		return false, err
	}
	client := ecs.NewFromConfig(cfg)
	out, err := client.ListClusters(ctx, &ecs.ListClustersInput{
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
// Concurrency is bounded to 8 outstanding probes, acquired *before* the
// goroutine starts so we don't eagerly build 27 AWS clients and 27 outstanding
// requests on a slow connection. Context cancellation is honoured promptly.
func DiscoverRegionsWithClusters(ctx context.Context, profile string, candidates []string) ([]string, error) {
	if cached, ok := LookupCachedRegions(profile); ok {
		return cached, nil
	}

	var (
		mu       sync.Mutex
		found    []string
		errCount int
		lastErr  error
		wg       sync.WaitGroup
	)

	sem := make(chan struct{}, 8)
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

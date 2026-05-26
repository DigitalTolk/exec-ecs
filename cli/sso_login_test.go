package cli

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
)

func TestSplitCSV(t *testing.T) {
	t.Parallel()
	got := splitCSV("sso:account:access, custom:scope ,, sso:other ")
	want := []string{"sso:account:access", "custom:scope", "sso:other"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if len(splitCSV("")) != 0 {
		t.Fatal("empty input should produce empty slice")
	}
}

func TestSSOCachePath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	expected := filepath.Join(tmp, ".aws", "sso", "cache", hashSession("main")+".json")
	if got := ssoCachePath("main"); got != expected {
		t.Fatalf("got %s want %s", got, expected)
	}
}

func hashSession(name string) string {
	sum := sha1.Sum([]byte(name))
	return hex.EncodeToString(sum[:])
}

func TestLoadCachedSSOTokenMissingOrCorrupt(t *testing.T) {
	tmp := t.TempDir()
	if got := loadCachedSSOToken(filepath.Join(tmp, "absent.json")); got != nil {
		t.Fatal("expected nil for missing file")
	}
	bad := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(bad, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadCachedSSOToken(bad); got != nil {
		t.Fatal("expected nil for corrupt file")
	}
}

func TestWriteAndReadCachedSSOToken(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "deep", "sso.json")
	want := &ssoTokenCache{
		AccessToken:           "abc",
		ExpiresAt:             time.Now().Add(time.Hour).UTC().Truncate(time.Second),
		Region:                "us-east-1",
		StartURL:              "https://example.awsapps.com/start",
		ClientID:              "client",
		ClientSecret:          "secret",
		RegistrationExpiresAt: time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second),
	}
	if err := writeCachedSSOToken(path, want); err != nil {
		t.Fatalf("write: %v", err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if stat.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %v", stat.Mode().Perm())
	}
	got := loadCachedSSOToken(path)
	if got == nil {
		t.Fatal("expected token")
	}
	if got.AccessToken != want.AccessToken || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("roundtrip mismatch: got=%+v want=%+v", got, want)
	}
}

func TestSSOCacheIsValid(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if SSOCacheIsValid("nope") {
		t.Fatal("missing cache should be invalid")
	}
	tok := &ssoTokenCache{
		AccessToken: "x",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	if err := writeCachedSSOToken(ssoCachePath("main"), tok); err != nil {
		t.Fatal(err)
	}
	if !SSOCacheIsValid("main") {
		t.Fatal("fresh token should be valid")
	}

	expired := &ssoTokenCache{AccessToken: "x", ExpiresAt: time.Now().Add(-time.Hour)}
	if err := writeCachedSSOToken(ssoCachePath("main"), expired); err != nil {
		t.Fatal(err)
	}
	if SSOCacheIsValid("main") {
		t.Fatal("expired token should be invalid")
	}

	soon := &ssoTokenCache{AccessToken: "x", ExpiresAt: time.Now().Add(10 * time.Second)}
	if err := writeCachedSSOToken(ssoCachePath("main"), soon); err != nil {
		t.Fatal(err)
	}
	if SSOCacheIsValid("main") {
		t.Fatal("near-expiry token should be invalid (60s safety margin)")
	}
}

func TestLookupSSOSessionConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	awsDir := filepath.Join(tmp, ".aws")
	if err := os.MkdirAll(awsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `
[profile p1]
sso_session = main
sso_account_id = 1
sso_role_name = R

[profile p2]
region = us-east-1

[profile p3]
sso_session = ghost

[sso-session main]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_registration_scopes = sso:account:access
`
	if err := os.WriteFile(filepath.Join(awsDir, "config"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	c := &Cli{}
	cfg, err := c.LookupSSOSessionConfig("p1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cfg == nil || cfg.Name != "main" || cfg.StartURL == "" || cfg.Region == "" {
		t.Fatalf("p1 cfg = %+v", cfg)
	}

	cfg, err = c.LookupSSOSessionConfig("p2")
	if err != nil || cfg != nil {
		t.Fatalf("p2 should have nil cfg without error: cfg=%+v err=%v", cfg, err)
	}

	_, err = c.LookupSSOSessionConfig("p3")
	if err == nil {
		t.Fatal("expected error when sso_session references missing block")
	}

	cfg, err = c.LookupSSOSessionConfig("missing")
	if err != nil || cfg != nil {
		t.Fatal("missing profile should return nil cfg with nil err")
	}
}

type fakeOIDCRegisterer struct {
	out *ssooidc.RegisterClientOutput
	err error
}

func (f *fakeOIDCRegisterer) RegisterClient(ctx context.Context, params *ssooidc.RegisterClientInput, optFns ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

func TestEnsureSSOClientRegistrationUsesCacheWhenValid(t *testing.T) {
	t.Parallel()
	url := "https://example.awsapps.com/start"
	cached := &ssoTokenCache{
		ClientID:              "cid",
		ClientSecret:          "secret",
		StartURL:              url,
		RegistrationExpiresAt: time.Now().Add(time.Hour),
	}
	id, secret, exp, err := ensureSSOClientRegistration(context.Background(), &fakeOIDCRegisterer{}, &SSOSessionConfig{StartURL: url}, cached)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "cid" || secret != "secret" || exp != cached.RegistrationExpiresAt {
		t.Fatalf("expected cache reuse, got id=%q secret=%q exp=%v", id, secret, exp)
	}
}

func TestEnsureSSOClientRegistrationReregistersOnStartURLChange(t *testing.T) {
	t.Parallel()
	cached := &ssoTokenCache{
		ClientID:              "old",
		ClientSecret:          "old",
		StartURL:              "https://old.awsapps.com/start",
		RegistrationExpiresAt: time.Now().Add(time.Hour),
	}
	exp := time.Now().Add(24 * time.Hour).Unix()
	reg := &fakeOIDCRegisterer{out: &ssooidc.RegisterClientOutput{
		ClientId:              aws.String("new"),
		ClientSecret:          aws.String("new-secret"),
		ClientSecretExpiresAt: exp,
	}}
	id, _, _, err := ensureSSOClientRegistration(context.Background(), reg, &SSOSessionConfig{StartURL: "https://new.awsapps.com/start"}, cached)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "new" {
		t.Fatalf("expected re-registration when start URL changes, got %s", id)
	}
}

func TestEnsureSSOClientRegistrationRegistersWhenExpired(t *testing.T) {
	t.Parallel()
	stale := &ssoTokenCache{
		ClientID:              "old",
		ClientSecret:          "old",
		RegistrationExpiresAt: time.Now().Add(-time.Hour),
	}
	exp := time.Now().Add(24 * time.Hour).Unix()
	reg := &fakeOIDCRegisterer{out: &ssooidc.RegisterClientOutput{
		ClientId:              aws.String("new-id"),
		ClientSecret:          aws.String("new-secret"),
		ClientSecretExpiresAt: exp,
	}}
	id, secret, _, err := ensureSSOClientRegistration(context.Background(), reg, &SSOSessionConfig{Scopes: "sso:account:access"}, stale)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != "new-id" || secret != "new-secret" {
		t.Fatalf("got id=%q secret=%q", id, secret)
	}
}

func TestEnsureSSOClientRegistrationError(t *testing.T) {
	t.Parallel()
	reg := &fakeOIDCRegisterer{err: errors.New("boom")}
	if _, _, _, err := ensureSSOClientRegistration(context.Background(), reg, &SSOSessionConfig{}, nil); err == nil {
		t.Fatal("expected error from RegisterClient")
	}
}

func TestPerformNativeSSOLoginRejectsIncompleteConfig(t *testing.T) {
	t.Parallel()
	c := &Cli{}
	if err := c.PerformNativeSSOLogin(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil cfg")
	}
	if err := c.PerformNativeSSOLogin(context.Background(), &SSOSessionConfig{Name: "x"}); err == nil {
		t.Fatal("expected error for missing url/region")
	}
}

func TestSSOTokenCacheJSONFields(t *testing.T) {
	t.Parallel()
	tok := ssoTokenCache{
		AccessToken: "abc",
		ExpiresAt:   time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	data, err := json.Marshal(tok)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{`"accessToken":"abc"`, `"expiresAt"`} {
		if !contains(s, want) {
			t.Fatalf("JSON missing %q: %s", want, s)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestOpenBrowserSkipsEmptyURL(t *testing.T) {
	t.Parallel()
	if err := openBrowser(""); err != nil {
		t.Fatalf("openBrowser(\"\") should be a no-op: %v", err)
	}
}

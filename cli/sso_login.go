package cli

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	ssooidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"gopkg.in/ini.v1"
)

// ssoClientName is reported to IAM Identity Center when we register as a
// public OAuth2 client. It shows up in the SSO UI alongside the device code.
const ssoClientName = "exec-ecs"

// SSOSessionConfig captures the subset of an `~/.aws/config` `[sso-session …]`
// block we need to drive an OIDC device-code login.
type SSOSessionConfig struct {
	Name     string
	StartURL string
	Region   string
	Scopes   string
}

// LookupSSOSessionConfig returns the full sso-session block referenced by a
// profile, or nil if the profile does not use SSO.
func (c *Cli) LookupSSOSessionConfig(profile string) (*SSOSessionConfig, error) {
	cfg, err := ini.Load(c.AWSConfigPath())
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	section, err := cfg.GetSection("profile " + profile)
	if err != nil {
		return nil, nil
	}
	sessionName := strings.TrimSpace(section.Key("sso_session").String())
	if sessionName == "" {
		return nil, nil
	}
	sessSection, err := cfg.GetSection("sso-session " + sessionName)
	if err != nil {
		return nil, fmt.Errorf("sso-session %q referenced by profile %q is missing from config", sessionName, profile)
	}
	return &SSOSessionConfig{
		Name:     sessionName,
		StartURL: strings.TrimSpace(sessSection.Key("sso_start_url").String()),
		Region:   strings.TrimSpace(sessSection.Key("sso_region").String()),
		Scopes:   strings.TrimSpace(sessSection.Key("sso_registration_scopes").String()),
	}, nil
}

// ssoTokenCache matches the on-disk JSON shape that the AWS SDK and AWS CLI
// both read for cached SSO tokens. Field names are PascalCase here but the
// JSON tags use camelCase to match the SDK.
type ssoTokenCache struct {
	AccessToken           string    `json:"accessToken"`
	ExpiresAt             time.Time `json:"expiresAt"`
	Region                string    `json:"region,omitzero"`
	StartURL              string    `json:"startUrl,omitzero"`
	ClientID              string    `json:"clientId,omitzero"`
	ClientSecret          string    `json:"clientSecret,omitzero"`
	RegistrationExpiresAt time.Time `json:"registrationExpiresAt,omitzero"`
}

// ssoCachePath returns the path the AWS SDK looks at for the cached token.
// For new-style sso-session configs the cache key is the SHA1 of the session
// name (matching what aws-sdk-go-v2/credentials/ssocreds does internally).
func ssoCachePath(sessionName string) string {
	sum := sha1.Sum([]byte(sessionName))
	name := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(homeDir(), ".aws", "sso", "cache", name)
}

func loadCachedSSOToken(path string) *ssoTokenCache {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var t ssoTokenCache
	if err := json.Unmarshal(data, &t); err != nil {
		return nil
	}
	return &t
}

func writeCachedSSOToken(path string, t *ssoTokenCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// browserOpener is overridable in tests so we don't actually launch a browser.
var browserOpener = openBrowser

// pollWait blocks for d while honouring context cancellation. Overridable in
// tests so the polling loop completes quickly.
var pollWait = func(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// PerformNativeSSOLogin drives the OAuth2 device-authorisation flow against
// IAM Identity Center's OIDC endpoint and writes the resulting token to the
// same on-disk cache slot the AWS SDK reads from, so subsequent SDK calls in
// this process and from `aws` CLI invocations pick it up transparently.
func (c *Cli) PerformNativeSSOLogin(ctx context.Context, sso *SSOSessionConfig) error {
	if sso == nil || sso.StartURL == "" || sso.Region == "" {
		return errors.New("incomplete sso-session config")
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(sso.Region))
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}
	oidc := ssooidc.NewFromConfig(awsCfg)

	cachePath := ssoCachePath(sso.Name)
	clientID, clientSecret, regExpiresAt, err := ensureSSOClientRegistration(ctx, oidc, sso, loadCachedSSOToken(cachePath))
	if err != nil {
		return err
	}

	devAuth, err := oidc.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     aws.String(clientID),
		ClientSecret: aws.String(clientSecret),
		StartUrl:     aws.String(sso.StartURL),
	})
	if err != nil {
		return fmt.Errorf("start device auth: %w", err)
	}

	verifyURL := aws.ToString(devAuth.VerificationUriComplete)
	if verifyURL == "" {
		verifyURL = aws.ToString(devAuth.VerificationUri)
	}
	fmt.Printf("\nOpen this URL to complete SSO login for session %q:\n  %s\n", sso.Name, verifyURL)
	fmt.Printf("If prompted, enter code: %s\n\n", aws.ToString(devAuth.UserCode))
	_ = browserOpener(verifyURL)

	interval := time.Duration(devAuth.Interval) * time.Second
	if interval < time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(devAuth.ExpiresIn) * time.Second)

	fmt.Println("Waiting for SSO authorisation to complete...")

	for {
		if time.Now().After(deadline) {
			return errors.New("sso login timed out")
		}
		// Sleep interruptibly. time.Sleep would swallow Ctrl-C for up to
		// `interval` (default 5 s, can grow on SlowDown), making the
		// program feel unresponsive on cancellation.
		if err := pollWait(ctx, interval); err != nil {
			return err
		}

		tokenOut, err := oidc.CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     aws.String(clientID),
			ClientSecret: aws.String(clientSecret),
			DeviceCode:   devAuth.DeviceCode,
			GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
		})
		if err != nil {
			var pending *ssooidctypes.AuthorizationPendingException
			if errors.As(err, &pending) {
				continue
			}
			var slowDown *ssooidctypes.SlowDownException
			if errors.As(err, &slowDown) {
				interval += 5 * time.Second
				continue
			}
			return fmt.Errorf("token poll: %w", err)
		}

		token := &ssoTokenCache{
			AccessToken:           aws.ToString(tokenOut.AccessToken),
			ExpiresAt:             time.Now().Add(time.Duration(tokenOut.ExpiresIn) * time.Second).UTC(),
			Region:                sso.Region,
			StartURL:              sso.StartURL,
			ClientID:              clientID,
			ClientSecret:          clientSecret,
			RegistrationExpiresAt: regExpiresAt.UTC(),
		}
		if err := writeCachedSSOToken(cachePath, token); err != nil {
			return fmt.Errorf("write token cache: %w", err)
		}
		fmt.Println("SSO login successful.")
		return nil
	}
}

// ssoOIDCRegisterer is the subset of ssooidc.Client used by registration.
type ssoOIDCRegisterer interface {
	RegisterClient(ctx context.Context, params *ssooidc.RegisterClientInput, optFns ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error)
}

func ensureSSOClientRegistration(ctx context.Context, oidc ssoOIDCRegisterer, sso *SSOSessionConfig, cached *ssoTokenCache) (string, string, time.Time, error) {
	// Reuse a cached registration only if it is fresh AND was issued for the
	// same sso-session (start URL identifies the IAM Identity Center
	// instance). A stale start URL means the user re-pointed their config
	// at a different SSO instance — re-registering is mandatory.
	if cached != nil &&
		cached.ClientID != "" &&
		cached.ClientSecret != "" &&
		cached.StartURL == sso.StartURL &&
		time.Until(cached.RegistrationExpiresAt) > time.Minute {
		return cached.ClientID, cached.ClientSecret, cached.RegistrationExpiresAt, nil
	}
	scopes := []string{"sso:account:access"}
	if sso.Scopes != "" {
		scopes = splitCSV(sso.Scopes)
	}
	out, err := oidc.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: aws.String(ssoClientName),
		ClientType: aws.String("public"),
		Scopes:     scopes,
	})
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("register sso client: %w", err)
	}
	return aws.ToString(out.ClientId), aws.ToString(out.ClientSecret), time.Unix(out.ClientSecretExpiresAt, 0).UTC(), nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SSOCacheIsValid reports whether a non-expired cached token exists for the
// given sso-session.
func SSOCacheIsValid(sessionName string) bool {
	t := loadCachedSSOToken(ssoCachePath(sessionName))
	if t == nil {
		return false
	}
	// Treat tokens within 60 s of expiry as already expired so we refresh
	// before the SDK fails halfway through a call.
	return time.Until(t.ExpiresAt) > 60*time.Second
}

func openBrowser(url string) error {
	if url == "" {
		return nil
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform for auto-open: %s", runtime.GOOS)
	}
	return cmd.Start()
}

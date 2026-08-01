package routes_auth

import (
	"aura/config"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// ErrOIDCNotEnabled is returned when an OIDC endpoint is reached with OIDC turned off.
var ErrOIDCNotEnabled = errors.New("OIDC is not enabled")

// oidcClient bundles everything derived from one OIDC configuration.
type oidcClient struct {
	verifier      *oidc.IDTokenVerifier
	oauth2Config  oauth2.Config
	endSessionURL string
	fingerprint   string
}

var (
	oidcMu     sync.Mutex
	oidcCached *oidcClient
)

// getOIDCClient returns a client for the current configuration, running provider discovery
// the first time it is needed.
//
// Discovery is deliberately lazy: the staged startup must not block on the identity
// provider being reachable, and an IdP that is down should fail sign-in rather than boot.
// The cache is keyed on the configuration itself, so a config update is picked up without
// any explicit invalidation hook.
func getOIDCClient(ctx context.Context) (*oidcClient, error) {
	cfg := config.Current.Auth.OIDC
	if !config.Current.Auth.Enabled || !cfg.Enabled {
		return nil, ErrOIDCNotEnabled
	}

	fingerprint := oidcFingerprint(cfg)

	oidcMu.Lock()
	defer oidcMu.Unlock()

	if oidcCached != nil && oidcCached.fingerprint == fingerprint {
		return oidcCached, nil
	}

	provider, err := oidc.NewProvider(ctx, strings.TrimSpace(cfg.IssuerURL))
	if err != nil {
		return nil, fmt.Errorf("provider discovery failed: %w", err)
	}

	// Not part of the standard provider metadata struct, so it has to be pulled out of the
	// raw discovery document.
	var extra struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&extra); err != nil {
		return nil, fmt.Errorf("could not read provider metadata: %w", err)
	}

	client := &oidcClient{
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth2Config: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.ScopesOrDefault(),
		},
		endSessionURL: extra.EndSessionEndpoint,
		fingerprint:   fingerprint,
	}

	oidcCached = client
	return client, nil
}

// oidcFingerprint identifies a configuration so a cached client is dropped when any value
// it was built from changes.
func oidcFingerprint(cfg config.Config_OIDC) string {
	return strings.Join([]string{
		cfg.IssuerURL,
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.RedirectURL,
		strings.Join(cfg.ScopesOrDefault(), " "),
	}, "\x00")
}

package remote

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/stream"
	jsoniter "github.com/json-iterator/go"
)

type safeString struct {
	*sync.RWMutex
	value string
}

func (s *safeString) Set(value string) {
	s.Lock()
	defer s.Unlock()
	s.value = value
}

func (s *safeString) Get() string {
	s.RLock()
	defer s.RUnlock()
	return s.value
}

// Config is the type that fetches config from Harness SaaS
type Config struct {
	key               string
	token             *safeString
	clusterIdentifier string
	proxyConfig       []domain.ProxyConfig
	ClientService     domain.ClientService
	stream            stream.Stream
	accountID         string
}

// NewConfig creates a new Config
func NewConfig(key string, cs domain.ClientService, s stream.Stream) *Config {
	c := &Config{
		token:         &safeString{RWMutex: &sync.RWMutex{}, value: ""},
		key:           key,
		ClientService: cs,
		stream:        s,
	}
	return c
}

// Token returns the authToken that the Config uses to communicate with Harness SaaS
func (c *Config) Token() string {
	return c.token.Get()
}

// AccountID returns the accountID for the account the Proxy is configured to work with
func (c *Config) AccountID() string {
	return c.accountID
}

func (c *Config) RefreshToken() (string, error) {
	authResp, err := authenticate(c.key, c.ClientService)
	if err != nil {
		return "", err
	}

	c.token.Set(authResp.Token)
	return c.token.Get(), nil
}

// ClusterIdentifier returns the identifier of the cluster that the Config authenticated against
func (c *Config) ClusterIdentifier() string {
	if c.clusterIdentifier == "" {
		return "1"
	}
	return c.clusterIdentifier
}

// Key returns proxyKey
func (c *Config) Key() string {
	return c.key
}

// SetProxyConfig sets the proxy config member
func (c *Config) SetProxyConfig(proxyConfig []domain.ProxyConfig) {
	c.proxyConfig = proxyConfig
}

// FetchAndPopulate Fetches and populates repositories with the config
func (c *Config) FetchAndPopulate(ctx context.Context, inventory domain.InventoryRepo, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {

	authResp, err := authenticate(c.key, c.ClientService)
	if err != nil {
		return err
	}
	c.token.Set(authResp.Token)
	c.clusterIdentifier = authResp.ClusterIdentifier

	proxyConfig, err := retrieveConfig(c.key, authResp.Token, authResp.ClusterIdentifier, c.ClientService)
	if err != nil {
		return err
	}

	// It's not the end of the world if we fail to
	// get the accountID from the auth token
	c.accountID, _ = parseAuthToken(authResp.Token)

	// TODO we probably should defer that
	// compare new and old config assets and delete difference.
	notificationsToSend, err := inventory.Cleanup(ctx, c.key, proxyConfig)
	if err != nil {
		return err
	}

	err = c.notifySDKs(ctx, notificationsToSend)
	if err != nil {
		return err
	}

	c.proxyConfig = proxyConfig
	return c.Populate(ctx, authRepo, flagRepo, segmentRepo)
}

func (c *Config) notifySDKs(ctx context.Context, notificationsToSend []domain.SSEMessage) error {
	// send the notifications
	for _, v := range notificationsToSend {
		err := c.stream.Publish(ctx, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// Populate populates repositories with the config
func (c *Config) Populate(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {
	var wg sync.WaitGroup
	errchan := make(chan error)
	semaphore := make(chan struct{}, 1000)

	for _, cfg := range c.proxyConfig {
		for _, targetEnv := range cfg.Environments {
			wg.Add(1)
			go func(env domain.Environments) {
				defer func() {
					wg.Done()
					<-semaphore
				}()
				semaphore <- struct{}{}
				authConfig := make([]domain.AuthConfig, 0, len(env.APIKeys))
				apiKeys := make([]string, 0, len(env.APIKeys))

				for _, apiKey := range env.APIKeys {
					apiKeys = append(apiKeys, string(domain.NewAuthAPIKey(apiKey)))

					authConfig = append(authConfig, domain.AuthConfig{
						APIKey:        domain.NewAuthAPIKey(apiKey),
						EnvironmentID: domain.EnvironmentID(env.ID.String()),
					})
				}
				err := populate(ctx, authRepo, flagRepo, segmentRepo, apiKeys, authConfig, env)
				errchan <- err
			}(targetEnv)
		}
	}

	go func() {
		wg.Wait()
		close(errchan)
		close(semaphore)
	}()

	for e := range errchan {
		if e != nil {
			return e
		}
	}
	return nil
}

// func extracted to satisfy lint complexity metrics.
func populate(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo, apiKeys []string, authConfig []domain.AuthConfig, env domain.Environments) error {

	// check for len is important to ensure we do not insert empty keys.
	// add apiKeys to cache.
	if len(apiKeys) > 0 {
		if err := authRepo.Add(ctx, authConfig...); err != nil {
			return fmt.Errorf("failed to add auth config to cache: %s", err)
		}
	}

	// add list of apiKeys for environment
	if len(authConfig) > 0 {
		if err := authRepo.AddAPIConfigsForEnvironment(ctx, env.ID.String(), apiKeys); err != nil {
			return fmt.Errorf("failed to add auth config to cache: %s", err)
		}
	}

	if len(env.FeatureConfigs) > 0 {
		if err := flagRepo.Add(ctx, domain.FlagConfig{
			EnvironmentID:  env.ID.String(),
			FeatureConfigs: env.FeatureConfigs,
		}); err != nil {
			return fmt.Errorf("failed to add flag config to cache: %s", err)
		}
	}
	if len(env.Segments) > 0 {
		if err := segmentRepo.Add(ctx, domain.SegmentConfig{
			EnvironmentID: env.ID.String(),
			Segments:      env.Segments,
		}); err != nil {
			return fmt.Errorf("failed to add segment config to cache: %s", err)
		}
	}
	return nil
}

func authenticate(key string, cs domain.ClientService) (domain.AuthenticateProxyKeyResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cs.AuthenticateProxyKey(ctx, key)
	if err != nil {
		return domain.AuthenticateProxyKeyResponse{}, err
	}

	return resp, nil
}

func retrieveConfig(key string, authToken string, clusterIdentifier string, cs domain.ClientService) ([]domain.ProxyConfig, error) {
	if clusterIdentifier == "" {
		clusterIdentifier = "1"
	}
	input := domain.GetProxyConfigInput{
		Key:               key,
		EnvID:             "",
		AuthToken:         authToken,
		ClusterIdentifier: clusterIdentifier,
		PageNumber:        0,
		PageSize:          10,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return cs.PageProxyConfig(ctx, input)
}

// parseAuthToken extracts the accountID from the auth token.
// If we need to extract more than the accountID in the future we
// can modify this function to return more than just the accountID string
func parseAuthToken(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("cannot parse empty token")
	}

	payloadIndex := 1
	payload := strings.Split(token, ".")[payloadIndex]
	payloadData, err := jwt.DecodeSegment(payload)
	if err != nil {
		return "", err
	}

	var claims map[string]interface{}
	if err = jsoniter.Unmarshal(payloadData, &claims); err != nil {
		return "", err
	}

	if accountID, ok := claims["account"].(string); ok {
		return accountID, nil
	}

	return "", fmt.Errorf("accountID not present in auth token")
}

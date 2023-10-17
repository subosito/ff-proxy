package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

var (
	// ErrUnexpectedMessageDomain is the error returned when an SSE message has a message domain we aren't expecting
	ErrUnexpectedMessageDomain = errors.New("unexpected message domain")

	// ErrUnexpectedEventType is the error returned when an SSE message has an event type we aren't expecting
	ErrUnexpectedEventType = errors.New("unexpected event type")
)

// Refresher is a type for handling SSE events from Harness Saas
type Refresher struct {
	proxyKey          string
	authToken         string
	clusterIdentifier string
	log               log.Logger
	clientService     domain.ClientService
	authRepo          domain.AuthRepo
	flagRepo          domain.FlagRepo
	segmentRepo       domain.SegmentRepo
}

// NewRefresher creates a Refresher
func NewRefresher(l log.Logger, proxyKey, authToken, clusterIdentifier string, client domain.ClientService, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) Refresher {
	l = l.With("component", "Refresher")
	return Refresher{log: l, proxyKey: proxyKey, authToken: authToken, clusterIdentifier: clusterIdentifier, clientService: client, authRepo: authRepo, flagRepo: flagRepo, segmentRepo: segmentRepo}
}

// HandleMessage makes Refresher implement the MessageHandler interface
func (s Refresher) HandleMessage(ctx context.Context, msg domain.SSEMessage) error {
	switch msg.Domain {
	case domain.MsgDomainFeature:
		return handleFeatureMessage(ctx, msg)
	case domain.MsgDomainSegment:
		return handleSegmentMessage(ctx, msg)
	case domain.MsgDomainProxy:
		return s.handleProxyMessage(ctx, msg)
	default:
		return fmt.Errorf("%w: %s", ErrUnexpectedMessageDomain, msg.Domain)
	}

}

func handleFeatureMessage(_ context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventDelete:
		// delete from the cache
	case domain.EventPatch, domain.EventCreate:

	default:
		return fmt.Errorf("%w %q for FeatureMessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

func handleSegmentMessage(_ context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventDelete:
	case domain.EventPatch, domain.EventCreate:

	default:
		return fmt.Errorf("%w %q for SegmentMessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

func (s Refresher) handleProxyMessage(ctx context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventProxyKeyDeleted:
		// todo
		return nil
	case domain.EventEnvironmentAdded:
		if err := s.handleAddEnvironmentEvent(ctx, msg.Environments); err != nil {
			s.log.Error("failed to handle addEnvironmentEvent", "err", err)
			return err
		}
	case domain.EventEnvironmentRemoved:
		// todo
		return nil
	case domain.EventAPIKeyAdded:
		// todo
		return nil
	case domain.EventAPIKeyRemoved:
		// todo
		return nil
	default:
		return fmt.Errorf("%w %q for Proxymessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

// handleAddEnvironmentEvent fetches proxyConfig for all added environments and sets them on.
func (s Refresher) handleAddEnvironmentEvent(ctx context.Context, environments []string) error {
	for _, env := range environments {
		input := domain.GetProxyConfigInput{
			Key:               s.proxyKey,
			EnvID:             env,
			AuthToken:         s.authToken,
			ClusterIdentifier: s.clusterIdentifier,
			PageNumber:        0,
			PageSize:          10,
		}

		proxyConfig, err := s.clientService.PageProxyConfig(ctx, input)
		if err != nil {
			s.log.Error("unable to fetch config for the environment", "environment", env)
			return err
		}

		authConfig := make([]domain.AuthConfig, 0, len(proxyConfig))
		flagConfig := make([]domain.FlagConfig, 0, len(proxyConfig))
		segmentConfig := make([]domain.SegmentConfig, 0, len(proxyConfig))

		for _, cfg := range proxyConfig {
			for _, environment := range cfg.Environments {
				for _, apiKey := range environment.APIKeys {
					authConfig = append(authConfig, domain.AuthConfig{
						APIKey:        domain.NewAuthAPIKey(apiKey),
						EnvironmentID: domain.EnvironmentID(environment.ID.String()),
					})
				}

				flagConfig = append(flagConfig, domain.FlagConfig{
					EnvironmentID:  environment.ID.String(),
					FeatureConfigs: environment.FeatureConfigs,
				})
				segmentConfig = append(segmentConfig, domain.SegmentConfig{
					EnvironmentID: environment.ID.String(),
					Segments:      environment.Segments,
				})
			}
			if err := s.authRepo.Add(ctx, authConfig...); err != nil {
				return fmt.Errorf("failed to add auth config to cache: %s", err)
			}

			if err := s.flagRepo.Add(ctx, flagConfig...); err != nil {
				return fmt.Errorf("failed to add flag config to cache: %s", err)
			}

			if err := s.segmentRepo.Add(ctx, segmentConfig...); err != nil {
				return fmt.Errorf("failed to add segment config to cache: %s", err)
			}
		}
	}
	return nil
}

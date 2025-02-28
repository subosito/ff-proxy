package stream

import (
	"context"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

// pollFn defines the function that polls Harness SaaS for changes
type pollFn func() error

// getConnectedStreamsFn defines the function that returns the names of open streams between the Proxy & SDKs
type getConnectedStreamsFn func() map[string]interface{}

type pollingStatus interface {
	Polling()
	NotPolling()
}

// SaasStreamOnDisconnect is called anytime we disconnect or fail to reconnect to the SaaS SSE stream and does the following
// - Sets the status of the SaaS stream in the cache to unhealthy, this means any new /stream requests to writer or read proxy's will be rejects
// - Polls saas for the latest config and refreshes the cache with any changes
// - Closes any 'Write Replica' Proxy -> SDK streams
// - Notifies 'read replica' proxy's that there's been a disconnection between the 'Write replica' and SaaS
func SaasStreamOnDisconnect(l log.Logger, streamHealth Health, pp Pushpin, redisSSEStream Stream, streams getConnectedStreamsFn, pollFn pollFn, pollingStatus pollingStatus) func() {
	return func() {
		l.Info("disconnected from Harness SaaS SSE Stream")

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Set to false so the ProxyService will reject any /stream requests from SDKs until we've reconnected
		_ = streamHealth.SetUnhealthy(ctx)
		pollingStatus.Polling()

		// Poll latest config from SaaS, this is to make sure we don't miss any changes that could have
		// happened while the stream was disconnected
		l.Info("polling Harness Saas for changes")
		if err := pollFn(); err != nil {
			l.Error("SSE stream disconnected, failed to poll for new config", "err", err)
		} else {
			l.Info("successfully polled Harness SaaS for changes")
		}

		// Close any open stream between this Proxy and SDKs. This is to force SDKs to poll the Proxy for
		// changes until we've a healthy SaaS -> Proxy stream to make sure they don't miss out on changes
		// the Proxy may have pulled down while the Proxy -> Saas stream was down.
		for streamID := range streams() {
			if err := pp.Close(streamID); err != nil {
				l.Error("failed to close Proxy->SDK stream", "streamID", streamID, "err", err)
			}
		}

		// Reset context timeout for publishing to the redis stream
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Publish an event to the redis stream that the read replica proxy's are listening on to let them
		// know we've disconnected from SaaS.
		l.Info("publishing disconnected message for replicas")
		if err := redisSSEStream.Publish(ctx, domain.SSEMessage{Event: "stream_action", Domain: domain.StreamStateDisconnected.String()}); err != nil {
			l.Error("failed to publish stream disconnected message to redis", "err", err)
			return
		}

		l.Info("successfully published disconnected message for replicas")
	}
}

// SaasStreamOnConnect sets the status of the SaaS stream to healthy in the cache
func SaasStreamOnConnect(l log.Logger, streamHealth Health, reloadConfig func() error, redisSSEStream Stream, pollingStatus pollingStatus) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		status, err := streamHealth.Status(ctx)
		if err != nil {
			l.Error("SaasOnConnectHandler failed to get stream state from cache", "err", err)
		}

		// If the previous streamStatus was "DISCONNECT" and we've successfully reconnected we should
		// do one final poll in case we missed any changes made between the last poll and reconnecting
		if status.State == domain.StreamStateDisconnected {
			l.Info("SaasOnConnectHandler polling for config changes")

			if err := reloadConfig(); err != nil {
				l.Error("SaasOnConnectHandler failed to poll for changes", "err", err)
			}
			l.Info("SaasOnConnectHandler successfully polled for config changes")
		}

		// Reset context timeout for the SetHealthy call
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		l.Info("connected to Harness SaaS SSE Stream")
		pollingStatus.NotPolling()
		if err := streamHealth.SetHealthy(ctx); err != nil {
			l.Error("failed to update SaaS stream status in cache", "err", err)
		}

		// Reset context timeout for the publishing to the stream
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Publish an event to the redis stream that the read replica proxy's are listening on to let them
		// know we've connected to SaaS.
		l.Info("publishing stream connected message for replicas")
		if err := redisSSEStream.Publish(ctx, domain.SSEMessage{Event: "stream_action", Domain: domain.StreamStateConnected.String()}); err != nil {
			l.Error("failed to publish stream connect message to redis", "err", err)
			return
		}

		l.Info("successfully published stream connected message for replicas")
	}
}

// ReadReplicaSSEStreamOnDisconnect is called whenever the read replica disconnects from a redis stream
func ReadReplicaSSEStreamOnDisconnect(l log.Logger, topic string) func() {
	return func() {
		l.Error("read replica disconnected from stream", "stream_name", topic)
	}
}

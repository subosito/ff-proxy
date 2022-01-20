package domain

// AuthRequest contains the fields sent in an authentication request
type AuthRequest struct {
	APIKey string
	Target Target
}

// AuthResponse contains the fields returned in an authentication response
type AuthResponse struct {
	AuthToken string `json:"authToken"`
}

// HealthResponse contains the fields returned in a healthcheck response
type HealthResponse map[string]string

// FeatureConfigRequest contains the fields sent in a GET /client/env/{environmentUUID}/feature-configs
type FeatureConfigRequest struct {
	EnvironmentID string
}

// FeatureConfigByIdentifierRequest contains the fields sent in a GET /client/env/{environmentUUID}/feature-configs/{identifier}
type FeatureConfigByIdentifierRequest struct {
	EnvironmentID string
	Identifier    string
}

// TargetSegmentsRequest contains the fields sent in a GET /client/env/{environmentUUID}/target-segments
type TargetSegmentsRequest struct {
	EnvironmentID string
}

// TargetSegmentsByIdentifierRequest contains the fields sent in a GET /client/env/{environmentUUID}/target-segments/{identifier}
type TargetSegmentsByIdentifierRequest struct {
	EnvironmentID string
	Identifier    string
}

// EvaluationsRequest contains the fields sent in a GET /client/env/{environmentUUID}/target/{target}/evaluations
type EvaluationsRequest struct {
	EnvironmentID    string
	TargetIdentifier string
}

// EvaluationsByFeatureRequest contains the fields sent in a GET /client/env/{environmentUUID}/target/{target}/evaluations/{feature} request
type EvaluationsByFeatureRequest struct {
	EnvironmentID     string
	TargetIdentifier  string
	FeatureIdentifier string
}

// StreamRequest contains the fields sent in a GET /stream request
type StreamRequest struct {
	APIKey string `json:"api_key"`
}

// StreamResponse contains the fields returned by a Stream request
type StreamResponse struct {
	GripChannel string
}

// MetricsRequest contains the fields sent in a POST /metrics request
type MetricsRequest struct {
	EnvironmentID string        `json:"environment_id"`
	TargetData    []TargetData  `json:"TargetData"`
	MetricsData   []MetricsData `json:"MetricsData"`
}

type TargetData struct {
	Identifier string                   `json:"identifier"`
	Name       string                   `json:"name"`
	Attributes []KeyValue               `json:"attributes"`
}

type MetricsData struct {
	Timestamp   int64                    `json:"timestamp"`
	Count       int                      `json:"count"`
	MetricsType string                   `json:"metricsType"`
	Attributes  []KeyValue               `json:"attributes"`
}

// KeyValue defines model for KeyValue.
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

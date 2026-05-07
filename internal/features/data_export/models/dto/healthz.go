package dto

// HealthzResponse — GET /healthz response.
type HealthzResponse struct {
	Status    string `json:"status"`             // ok | degraded | down
	DB        string `json:"db"`                 // ok | unreachable
	Version   string `json:"version,omitempty"`
	BuildTime string `json:"build_time,omitempty"`
}

package seaweed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type MasterClient struct {
	baseURL string
	client  *http.Client
}

func NewMasterClient(endpoint string) (*MasterClient, error) {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid Master URL")
	}
	return &MasterClient{baseURL: strings.TrimRight(endpoint, "/"), client: &http.Client{Timeout: 5 * time.Second}}, nil
}

type MasterCheck struct {
	Reachable  bool           `json:"reachable"`
	HTTPStatus int            `json:"http_status,omitempty"`
	Endpoint   string         `json:"endpoint"`
	LatencyMS  int64          `json:"latency_ms"`
	Details    map[string]any `json:"details,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// Check probes the Master health endpoint without mutating cluster state.
func (m *MasterClient) Check(ctx context.Context) MasterCheck {
	out := MasterCheck{Endpoint: m.baseURL}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL+"/healthz", nil)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	resp, err := m.client.Do(req)
	out.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		out.Error = err.Error()
		return out
	}
	defer resp.Body.Close()
	out.HTTPStatus = resp.StatusCode
	out.Reachable = resp.StatusCode >= 200 && resp.StatusCode < 300
	var payload map[string]any
	if json.NewDecoder(resp.Body).Decode(&payload) == nil {
		out.Details = payload
	}
	if !out.Reachable {
		out.Error = fmt.Sprintf("Master returned HTTP %d", resp.StatusCode)
	}
	return out
}

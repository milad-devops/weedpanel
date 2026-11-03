package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type metricQuery struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Unit  string `json:"unit"`
	Query string `json:"query"`
}

type prometheusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string    `json:"metric"`
			Values [][2]json.RawMessage `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

var monitoringQueries = []metricQuery{
	{Key: "targets", Title: "Scrape targets", Unit: "state", Query: "up"},
	{Key: "memory", Title: "Process memory", Unit: "bytes", Query: "process_resident_memory_bytes"},
	{Key: "goroutines", Title: "Go goroutines", Unit: "count", Query: "go_goroutines"},
	{Key: "cpu", Title: "CPU usage", Unit: "cores", Query: "rate(process_cpu_seconds_total[5m])"},
}

func (s *Server) clusterMetrics(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadCluster(r)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	if err != nil {
		writeError(w, 500, "database_error", "could not load cluster")
		return
	}
	if strings.TrimSpace(c.PrometheusURL) == "" {
		writeError(w, 400, "endpoint_missing", "Prometheus URL is not configured for this cluster")
		return
	}
	base, err := url.Parse(strings.TrimRight(c.PrometheusURL, "/"))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		writeError(w, 400, "invalid_endpoint", "configured Prometheus URL is invalid")
		return
	}
	period := r.URL.Query().Get("range")
	duration, step := monitoringRange(period)
	end := time.Now().UTC()
	start := end.Add(-duration)

	type series struct {
		Metric map[string]string `json:"metric"`
		Values []metricPoint     `json:"values"`
	}
	type result struct {
		Key    string   `json:"key"`
		Title  string   `json:"title"`
		Unit   string   `json:"unit"`
		Series []series `json:"series"`
	}
	results := make([]result, 0, len(monitoringQueries))
	for _, q := range monitoringQueries {
		payload, err := queryPrometheus(r.Context(), base, q.Query, start, end, step)
		if err != nil {
			writeError(w, 502, "prometheus_error", fmt.Sprintf("%s: %v", q.Title, err))
			return
		}
		item := result{Key: q.Key, Title: q.Title, Unit: q.Unit, Series: []series{}}
		for _, raw := range payload.Data.Result {
			points := make([]metricPoint, 0, len(raw.Values))
			for _, pair := range raw.Values {
				var ts float64
				var value string
				if len(pair) != 2 || json.Unmarshal(pair[0], &ts) != nil || json.Unmarshal(pair[1], &value) != nil {
					continue
				}
				points = append(points, metricPoint{Timestamp: int64(ts), Value: value})
			}
			item.Series = append(item.Series, series{Metric: raw.Metric, Values: points})
		}
		results = append(results, item)
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"range": periodOrDefault(period), "start": start, "end": end, "step": step.String(), "metrics": results}})
}

type metricPoint struct {
	Timestamp int64  `json:"timestamp"`
	Value     string `json:"value"`
}

func periodOrDefault(p string) string {
	if p == "6h" || p == "24h" || p == "7d" {
		return p
	}
	return "1h"
}

func monitoringRange(p string) (time.Duration, time.Duration) {
	switch p {
	case "6h":
		return 6 * time.Hour, time.Minute * 2
	case "24h":
		return 24 * time.Hour, time.Minute * 5
	case "7d":
		return 7 * 24 * time.Hour, time.Minute * 30
	default:
		return time.Hour, time.Minute
	}
}

func queryPrometheus(ctx context.Context, base *url.URL, query string, start, end time.Time, step time.Duration) (*prometheusResponse, error) {
	u := *base
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v1/query_range"
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%.0f", float64(start.UnixNano())/1e9))
	params.Set("end", fmt.Sprintf("%.0f", float64(end.UnixNano())/1e9))
	params.Set("step", step.String())
	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var out prometheusResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("invalid Prometheus response: %w", err)
	}
	if out.Status != "success" {
		return nil, fmt.Errorf("query failed: %s", out.Error)
	}
	return &out, nil
}

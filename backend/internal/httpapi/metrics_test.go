package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestMonitoringRange(t *testing.T) {
	cases := []struct {
		name           string
		duration, step time.Duration
	}{
		{"1h", time.Hour, time.Minute},
		{"6h", 6 * time.Hour, 2 * time.Minute},
		{"24h", 24 * time.Hour, 5 * time.Minute},
		{"7d", 7 * 24 * time.Hour, 30 * time.Minute},
		{"default", time.Hour, time.Minute},
	}
	for _, tc := range cases {
		d, s := monitoringRange(tc.name)
		if d != tc.duration || s != tc.step {
			t.Errorf("monitoringRange(%q) = (%s,%s)", tc.name, d, s)
		}
	}
}

func TestQueryPrometheus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prom/api/v1/query_range" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "up" || r.URL.Query().Get("step") != "1m0s" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "matrix", "result": []any{map[string]any{"metric": map[string]string{"job": "seaweed"}, "values": [][]any{{float64(123), "1"}}}}}})
	}))
	defer server.Close()
	base, _ := url.Parse(server.URL + "/prom")
	out, err := queryPrometheus(context.Background(), base, "up", time.Unix(100, 0), time.Unix(200, 0), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Data.Result) != 1 || len(out.Data.Result[0].Values) != 1 {
		t.Fatalf("unexpected result: %#v", out.Data)
	}
}

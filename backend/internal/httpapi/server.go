package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/weedpanel/weedpanel/backend/internal/auth"
	"github.com/weedpanel/weedpanel/backend/internal/config"
	"github.com/weedpanel/weedpanel/backend/internal/seaweed"
	"github.com/weedpanel/weedpanel/backend/internal/security"
)

type Server struct {
	db    *pgxpool.Pool
	cfg   config.Config
	auth  *auth.Service
	vault *security.Vault
}

func New(db *pgxpool.Pool, cfg config.Config) http.Handler {
	s := &Server{db: db, cfg: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /api/v1/system/info", s.systemInfo)
	secure := cfg.Environment == "production"
	s.auth = auth.New(db, secure)
	s.auth.RegisterRoutes(mux)
	mux.Handle("GET /api/v1/clusters", s.auth.RequirePermission("clusters.read", http.HandlerFunc(s.listClusters)))
	mux.Handle("POST /api/v1/clusters", s.auth.RequirePermission("clusters.write", http.HandlerFunc(s.createCluster)))
	mux.Handle("GET /api/v1/clusters/{id}", s.auth.RequirePermission("clusters.read", http.HandlerFunc(s.getCluster)))
	mux.Handle("PATCH /api/v1/clusters/{id}", s.auth.RequirePermission("clusters.write", http.HandlerFunc(s.updateCluster)))
	mux.Handle("DELETE /api/v1/clusters/{id}", s.auth.RequirePermission("clusters.delete", http.HandlerFunc(s.deleteCluster)))
	mux.Handle("POST /api/v1/clusters/{id}/connection-test", s.auth.RequirePermission("clusters.connect", http.HandlerFunc(s.testCluster)))
	mux.Handle("POST /api/v1/clusters/{id}/credentials", s.auth.RequirePermission("clusters.write", http.HandlerFunc(s.storeCredential)))
	mux.Handle("GET /api/v1/clusters/{id}/live", s.auth.RequirePermission("clusters.read", http.HandlerFunc(s.clusterLive)))
	mux.Handle("GET /api/v1/clusters/{id}/metrics", s.auth.RequirePermission("clusters.read", http.HandlerFunc(s.clusterMetrics)))
	mux.Handle("GET /api/v1/clusters/{id}/files", s.auth.RequirePermission("clusters.read", http.HandlerFunc(s.listFiles)))
	mux.Handle("GET /api/v1/clusters/{id}/file", s.auth.RequirePermission("clusters.read", http.HandlerFunc(s.downloadFile)))
	mux.Handle("POST /api/v1/clusters/{id}/files", s.auth.RequirePermission("clusters.write", http.HandlerFunc(s.uploadFile)))
	mux.Handle("POST /api/v1/clusters/{id}/directories", s.auth.RequirePermission("clusters.write", http.HandlerFunc(s.createDirectory)))
	mux.Handle("DELETE /api/v1/clusters/{id}/files", s.auth.RequirePermission("clusters.write", http.HandlerFunc(s.deleteFile)))
	mux.Handle("GET /api/v1/admin/users", s.auth.RequirePermission("users.manage", http.HandlerFunc(s.listUsers)))
	if cfg.EncryptionKey != "" {
		s.vault, _ = security.NewVault(cfg.EncryptionKey)
	}
	return requestLogger(mux)
}
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(r.Context()); err != nil {
		writeError(w, 503, "not_ready", "database unavailable")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ready", "database": "ok"})
}
func (s *Server) systemInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"name": "WeedPanel", "version": "0.4.0", "environment": s.cfg.Environment, "api_version": "v1", "capabilities": []string{"multi-cluster", "direct-connect", "local-auth", "master-connection-test", "master-live-status", "filer-file-browser", "filer-file-operations", "s3-read-only", "prometheus-monitoring", "admin-user-list", "oidc-planned", "agent-planned"}})
}

type cluster struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	ConnectionMode string     `json:"connection_mode"`
	MasterURLs     []string   `json:"master_urls"`
	FilerURL       string     `json:"filer_url"`
	S3URL          string     `json:"s3_url"`
	PrometheusURL  string     `json:"prometheus_url"`
	Status         string     `json:"status"`
	LastSeenAt     *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

const clusterSelect = `SELECT id::text,name,description,connection_mode,master_urls,filer_url,s3_url,prometheus_url,status,last_seen_at,created_at,updated_at FROM clusters`

func scanCluster(row pgx.Row) (cluster, error) {
	var c cluster
	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.ConnectionMode, &c.MasterURLs, &c.FilerURL, &c.S3URL, &c.PrometheusURL, &c.Status, &c.LastSeenAt, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
func (s *Server) listClusters(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), clusterSelect+" ORDER BY name")
	if err != nil {
		writeError(w, 500, "database_error", "could not list clusters")
		return
	}
	defer rows.Close()
	items := []cluster{}
	for rows.Next() {
		c, e := scanCluster(rows)
		if e != nil {
			writeError(w, 500, "database_error", "could not decode clusters")
			return
		}
		items = append(items, c)
	}
	if rows.Err() != nil {
		writeError(w, 500, "database_error", "could not read clusters")
		return
	}
	writeJSON(w, 200, map[string]any{"data": items})
}

type clusterRequest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	ConnectionMode string   `json:"connection_mode"`
	MasterURLs     []string `json:"master_urls"`
	FilerURL       string   `json:"filer_url"`
	S3URL          string   `json:"s3_url"`
	PrometheusURL  string   `json:"prometheus_url"`
}

func validateCluster(req *clusterRequest) string {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return "name is required"
	}
	if req.ConnectionMode == "" {
		req.ConnectionMode = "direct"
	}
	if req.ConnectionMode != "direct" && req.ConnectionMode != "agent" {
		return "connection_mode must be direct or agent"
	}
	for _, ep := range append(append([]string{}, req.MasterURLs...), req.FilerURL, req.S3URL, req.PrometheusURL) {
		if ep == "" {
			continue
		}
		u, e := url.ParseRequestURI(ep)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return "endpoints must be valid HTTP(S) URLs"
		}
	}
	if req.ConnectionMode == "direct" && len(req.MasterURLs) == 0 {
		return "at least one Master URL is required for direct mode"
	}
	return ""
}
func (s *Server) createCluster(w http.ResponseWriter, r *http.Request) {
	var req clusterRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req) != nil {
		writeError(w, 400, "invalid_json", "request body must be valid JSON")
		return
	}
	if msg := validateCluster(&req); msg != "" {
		writeError(w, 400, "validation_error", msg)
		return
	}
	c, err := scanCluster(s.db.QueryRow(r.Context(), `INSERT INTO clusters(id,name,description,connection_mode,master_urls,filer_url,s3_url,prometheus_url) VALUES($1,$2,$3,$4,$5::jsonb,$6,$7,$8) RETURNING `+clusterSelectColumns, uuid.New(), req.Name, req.Description, req.ConnectionMode, mustJSON(req.MasterURLs), req.FilerURL, req.S3URL, req.PrometheusURL))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, 409, "cluster_exists", "cluster name already exists")
		} else {
			writeError(w, 500, "database_error", "could not create cluster")
		}
		return
	}
	writeJSON(w, 201, map[string]any{"data": c})
}

const clusterSelectColumns = `id::text,name,description,connection_mode,master_urls,filer_url,s3_url,prometheus_url,status,last_seen_at,created_at,updated_at`

func (s *Server) getCluster(w http.ResponseWriter, r *http.Request) {
	c, e := scanCluster(s.db.QueryRow(r.Context(), clusterSelect+" WHERE id=$1", r.PathValue("id")))
	if errors.Is(e, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	if e != nil {
		writeError(w, 500, "database_error", "could not load cluster")
		return
	}
	writeJSON(w, 200, map[string]any{"data": c})
}
func (s *Server) updateCluster(w http.ResponseWriter, r *http.Request) {
	var req clusterRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req) != nil {
		writeError(w, 400, "invalid_json", "request body must be valid JSON")
		return
	}
	if msg := validateCluster(&req); msg != "" {
		writeError(w, 400, "validation_error", msg)
		return
	}
	c, e := scanCluster(s.db.QueryRow(r.Context(), `UPDATE clusters SET name=$2,description=$3,connection_mode=$4,master_urls=$5::jsonb,filer_url=$6,s3_url=$7,prometheus_url=$8,updated_at=NOW() WHERE id=$1 RETURNING `+clusterSelectColumns, r.PathValue("id"), req.Name, req.Description, req.ConnectionMode, mustJSON(req.MasterURLs), req.FilerURL, req.S3URL, req.PrometheusURL))
	if errors.Is(e, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	if e != nil {
		writeError(w, 409, "update_failed", "could not update cluster")
		return
	}
	writeJSON(w, 200, map[string]any{"data": c})
}
func (s *Server) deleteCluster(w http.ResponseWriter, r *http.Request) {
	tag, e := s.db.Exec(r.Context(), `DELETE FROM clusters WHERE id=$1`, r.PathValue("id"))
	if e != nil {
		writeError(w, 500, "database_error", "could not delete cluster")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]bool{"deleted": true}})
}
func (s *Server) testCluster(w http.ResponseWriter, r *http.Request) {
	var endpoints []string
	e := s.db.QueryRow(r.Context(), `SELECT master_urls FROM clusters WHERE id=$1`, r.PathValue("id")).Scan(&endpoints)
	if errors.Is(e, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	if e != nil {
		writeError(w, 500, "database_error", "could not load cluster")
		return
	}
	results := make([]seaweed.MasterCheck, 0, len(endpoints))
	healthy := false
	for _, endpoint := range endpoints {
		client, err := seaweed.NewMasterClient(endpoint)
		if err != nil {
			results = append(results, seaweed.MasterCheck{Endpoint: endpoint, Error: err.Error()})
			continue
		}
		res := client.Check(r.Context())
		results = append(results, res)
		healthy = healthy || res.Reachable
	}
	status := "unreachable"
	if healthy {
		status = "healthy"
	}
	_, _ = s.db.Exec(r.Context(), `UPDATE clusters SET status=$2,last_seen_at=CASE WHEN $2='healthy' THEN NOW() ELSE last_seen_at END,updated_at=NOW() WHERE id=$1`, r.PathValue("id"), status)
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": status, "master_checks": results}})
}

func (s *Server) storeCredential(w http.ResponseWriter, r *http.Request) {
	if s.vault == nil {
		writeError(w, 503, "vault_unavailable", "APP_ENCRYPTION_KEY is not configured")
		return
	}
	var req struct {
		Type   string `json:"type"`
		Secret string `json:"secret"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req) != nil || strings.TrimSpace(req.Type) == "" || req.Secret == "" {
		writeError(w, 400, "validation_error", "type and secret are required")
		return
	}
	if req.Type != "s3_access_key" && req.Type != "s3_secret_key" && req.Type != "filer_auth" && req.Type != "master_auth" {
		writeError(w, 400, "validation_error", "unsupported credential type")
		return
	}
	var exists bool
	if err := s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM clusters WHERE id=$1)`, r.PathValue("id")).Scan(&exists); err != nil || !exists {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	ciphertext, err := s.vault.Encrypt([]byte(req.Secret))
	if err != nil {
		writeError(w, 500, "encryption_failed", "could not encrypt credential")
		return
	}
	_, err = s.db.Exec(r.Context(), `INSERT INTO cluster_credentials(id,cluster_id,credential_type,secret_ciphertext) VALUES($1,$2,$3,$4)`, uuid.New(), r.PathValue("id"), req.Type, ciphertext)
	if err != nil {
		writeError(w, 500, "database_error", "could not store credential")
		return
	}
	writeJSON(w, 201, map[string]any{"data": map[string]string{"type": req.Type, "status": "stored"}})
}

// clusterLive reads live SeaweedFS endpoints through the authenticated control plane.
func (s *Server) loadCluster(r *http.Request) (cluster, error) {
	return scanCluster(s.db.QueryRow(r.Context(), clusterSelect+" WHERE id=$1", r.PathValue("id")))
}

func (s *Server) clusterLive(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadCluster(r)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	if err != nil {
		writeError(w, 500, "database_error", "could not load cluster")
		return
	}
	section := r.URL.Query().Get("section")
	base, endpoint := "", ""
	switch section {
	case "topology":
		if len(c.MasterURLs) > 0 {
			base = c.MasterURLs[0]
		}
		endpoint = "/cluster/status?pretty=y"
	case "storage":
		if len(c.MasterURLs) > 0 {
			base = c.MasterURLs[0]
		}
		endpoint = "/dir/status?pretty=y"
	case "volumes":
		if len(c.MasterURLs) > 0 {
			base = c.MasterURLs[0]
		}
		endpoint = "/vol/status?pretty=y"
	case "s3":
		base = c.S3URL
		endpoint = "/"
	default:
		writeError(w, 400, "invalid_section", "section must be topology, storage, volumes or s3")
		return
	}
	if base == "" {
		writeError(w, 400, "endpoint_missing", "This cluster has no endpoint configured for the requested service")
		return
	}
	proxyRead(w, r, base+endpoint)
}

func safeFilerPath(raw string) string {
	clean := path.Clean("/" + strings.TrimSpace(raw))
	if clean == "/" {
		return "/"
	}
	return clean
}

func filerTarget(base, raw string) string {
	u, _ := url.Parse(strings.TrimRight(base, "/"))
	u.Path = strings.TrimRight(u.Path, "/") + safeFilerPath(raw)
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func filerURL(base, raw string) string {
	u, _ := url.Parse(filerTarget(base, raw))
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	q := u.Query()
	q.Set("pretty", "y")
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadCluster(r)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	if err != nil {
		writeError(w, 500, "database_error", "could not load cluster")
		return
	}
	if c.FilerURL == "" {
		writeError(w, 400, "endpoint_missing", "Filer URL is not configured")
		return
	}
	proxyRead(w, r, filerURL(c.FilerURL, r.URL.Query().Get("path")))
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadCluster(r)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	if err != nil {
		writeError(w, 500, "database_error", "could not load cluster")
		return
	}
	if c.FilerURL == "" {
		writeError(w, 400, "endpoint_missing", "Filer URL is not configured")
		return
	}
	filePath := safeFilerPath(r.URL.Query().Get("path"))
	if filePath == "/" {
		writeError(w, 400, "invalid_path", "a file path is required")
		return
	}
	target := filerTarget(c.FilerURL, filePath)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		writeError(w, 400, "invalid_path", "invalid file path")
		return
	}
	resp, err := (&http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		writeError(w, 502, "filer_unavailable", err.Error())
		return
	}
	defer resp.Body.Close()
	for _, key := range []string{"Content-Type", "Content-Disposition", "ETag", "Last-Modified"} {
		if v := resp.Header.Get(key); v != "" {
			w.Header().Set(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 256<<20))
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadCluster(r)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	if err != nil {
		writeError(w, 500, "database_error", "could not load cluster")
		return
	}
	if c.FilerURL == "" {
		writeError(w, 400, "endpoint_missing", "Filer URL is not configured")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		writeError(w, 400, "invalid_name", "A simple file name is required")
		return
	}
	dir := safeFilerPath(r.URL.Query().Get("path"))
	target := filerTarget(c.FilerURL, strings.TrimRight(dir, "/")+"/"+name)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, http.MaxBytesReader(w, r.Body, 64<<20))
	if err != nil {
		writeError(w, 400, "invalid_request", "invalid upload")
		return
	}
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		writeError(w, 502, "filer_unavailable", err.Error())
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 1<<20))
}

func (s *Server) createDirectory(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadCluster(r)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	if err != nil {
		writeError(w, 500, "database_error", "could not load cluster")
		return
	}
	if c.FilerURL == "" {
		writeError(w, 400, "endpoint_missing", "Filer URL is not configured")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		writeError(w, 400, "invalid_name", "A simple directory name is required")
		return
	}
	dir := safeFilerPath(r.URL.Query().Get("path"))
	target := strings.TrimRight(filerTarget(c.FilerURL, strings.TrimRight(dir, "/")+"/"+name), "/") + "/"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, nil)
	if err != nil {
		writeError(w, 400, "invalid_request", "invalid directory path")
		return
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		writeError(w, 502, "filer_unavailable", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, resp.StatusCode, "filer_error", fmt.Sprintf("Filer returned HTTP %d", resp.StatusCode))
		return
	}
	writeJSON(w, 201, map[string]any{"data": map[string]bool{"created": true}})
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadCluster(r)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "not_found", "cluster not found")
		return
	}
	if err != nil {
		writeError(w, 500, "database_error", "could not load cluster")
		return
	}
	if c.FilerURL == "" {
		writeError(w, 400, "endpoint_missing", "Filer URL is not configured")
		return
	}
	filePath := safeFilerPath(r.URL.Query().Get("path"))
	if filePath == "/" {
		writeError(w, 400, "invalid_path", "root cannot be deleted")
		return
	}
	target := filerTarget(c.FilerURL, filePath)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodDelete, target, nil)
	if err != nil {
		writeError(w, 400, "invalid_request", "invalid path")
		return
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		writeError(w, 502, "filer_unavailable", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, resp.StatusCode, "filer_error", fmt.Sprintf("Filer returned HTTP %d", resp.StatusCode))
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]bool{"deleted": true}})
}

func proxyRead(w http.ResponseWriter, r *http.Request, target string) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		writeError(w, 400, "invalid_endpoint", "invalid endpoint")
		return
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		writeError(w, 502, "upstream_unavailable", err.Error())
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		writeError(w, 502, "upstream_read_failed", "could not read upstream response")
		return
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `SELECT u.id::text,u.email,u.display_name,u.is_active,u.created_at,COALESCE(string_agg(ro.name,', ' ORDER BY ro.name),'') FROM users u LEFT JOIN user_roles ur ON ur.user_id=u.id LEFT JOIN roles ro ON ro.id=ur.role_id GROUP BY u.id ORDER BY u.created_at`)
	if err != nil {
		writeError(w, 500, "database_error", "could not list users")
		return
	}
	defer rows.Close()
	type item struct {
		ID          string    `json:"id"`
		Email       string    `json:"email"`
		DisplayName string    `json:"display_name"`
		Active      bool      `json:"active"`
		CreatedAt   time.Time `json:"created_at"`
		Roles       string    `json:"roles"`
	}
	items := []item{}
	for rows.Next() {
		var x item
		if rows.Scan(&x.ID, &x.Email, &x.DisplayName, &x.Active, &x.CreatedAt, &x.Roles) != nil {
			writeError(w, 500, "database_error", "could not decode users")
			return
		}
		items = append(items, x)
	}
	writeJSON(w, 200, map[string]any{"data": items})
}

func mustJSON(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		return []byte("[]")
	}
	return b
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, r) })
}

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userIDKey contextKey = "user_id"

// All registration requests use the same PostgreSQL advisory lock. This makes
// the first-user check and insert atomic across concurrent requests and app instances.
const registrationLockKey1 int32 = 20261103
const registrationLockKey2 int32 = 1

type Service struct {
	DB           *pgxpool.Pool
	CookieSecure bool
}

func New(db *pgxpool.Pool, secure bool) *Service {
	return &Service{DB: db, CookieSecure: secure}
}

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", s.register)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.Handle("GET /api/v1/auth/me", s.RequireAuth(http.HandlerFunc(s.me)))
}

func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("weedpanel_session")
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		sum := sha256.Sum256([]byte(c.Value))
		var userID string
		err = s.DB.QueryRow(r.Context(), `SELECT user_id::text FROM sessions WHERE token_hash=$1 AND expires_at>NOW()`, hex.EncodeToString(sum[:])).Scan(&userID)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "invalid_session")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) RequirePermission(permission string, next http.Handler) http.Handler {
	return s.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, _ := r.Context().Value(userIDKey).(string)
		var allowed bool
		err := s.DB.QueryRow(r.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM user_roles ur
				JOIN role_permissions rp ON rp.role_id=ur.role_id
				JOIN permissions p ON p.id=rp.permission_id
				WHERE ur.user_id=$1 AND p.code=$2
			)`, userID, permission).Scan(&allowed)
		if err != nil || !allowed {
			writeAuthError(w, http.StatusForbidden, "permission_denied")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// register permits creation of the initial administrator only. Once any user
// exists, registration remains closed. The advisory transaction lock prevents
// two simultaneous requests from both observing an empty users table.
func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}

	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in) != nil || !strings.Contains(in.Email, "@") || len(in.Password) < 12 {
		writeAuthError(w, http.StatusBadRequest, "email_required_and_password_min_12")
		return
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || !strings.Contains(email, "@") {
		writeAuthError(w, http.StatusBadRequest, "email_required_and_password_min_12")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "password_hash_failed")
		return
	}

	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "database_error")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	// Serialize initial-registration attempts across all backend replicas.
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock($1, $2)`, registrationLockKey1, registrationLockKey2); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "database_error")
		return
	}

	var count int
	if err = tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "database_error")
		return
	}
	if count > 0 {
		writeAuthError(w, http.StatusForbidden, "registration_closed")
		return
	}

	id := newID()
	_, err = tx.Exec(r.Context(), `
		INSERT INTO users(id,email,display_name,password_hash,auth_provider)
		VALUES($1,$2,$3,$4,'local')`, id, email, strings.TrimSpace(in.DisplayName), string(hash))
	if err != nil {
		writeAuthError(w, http.StatusConflict, "user_exists")
		return
	}

	_, err = tx.Exec(r.Context(), `
		INSERT INTO user_roles(user_id,role_id)
		VALUES($1,'00000000-0000-4000-8000-000000000001')`, id)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "role_assignment_failed")
		return
	}

	if err = tx.Commit(r.Context()); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "database_error")
		return
	}

	if err = s.createSession(w, r, id); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "session_creation_failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]string{
		"id": id, "email": email, "display_name": strings.TrimSpace(in.DisplayName),
	}})
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in) != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	var id, hash string
	err := s.DB.QueryRow(r.Context(), `SELECT id::text,password_hash FROM users WHERE email=$1 AND is_active=true AND auth_provider='local'`, strings.ToLower(strings.TrimSpace(in.Email))).Scan(&id, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		writeAuthError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	if err := s.createSession(w, r, id); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "session_creation_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"user_id": id}})
}

func (s *Service) createSession(w http.ResponseWriter, r *http.Request, userID string) error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	expiresAt := time.Now().Add(12 * time.Hour)

	if _, err := s.DB.Exec(r.Context(), `INSERT INTO sessions(id,user_id,token_hash,expires_at) VALUES($1,$2,$3,$4)`, newID(), userID, hex.EncodeToString(sum[:]), expiresAt); err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name: "weedpanel_session", Value: token, Path: "/", HttpOnly: true,
		Secure: s.CookieSecure, SameSite: http.SameSiteLaxMode,
		Expires: expiresAt, MaxAge: 43200,
	})
	return nil
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("weedpanel_session"); err == nil {
		sum := sha256.Sum256([]byte(c.Value))
		_, _ = s.DB.Exec(r.Context(), `DELETE FROM sessions WHERE token_hash=$1`, hex.EncodeToString(sum[:]))
	}
	http.SetCookie(w, &http.Cookie{
		Name: "weedpanel_session", Value: "", Path: "/", HttpOnly: true,
		Secure: s.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"logged_out": true}})
}

func (s *Service) me(w http.ResponseWriter, r *http.Request) {
	id, _ := r.Context().Value(userIDKey).(string)
	var email, name string
	if err := s.DB.QueryRow(r.Context(), `SELECT email,display_name FROM users WHERE id=$1`, id).Scan(&email, &name); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "database_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"id": id, "email": email, "display_name": name}})
}

func writeAuthError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return formatUUID(b)
}

func formatUUID(b []byte) string {
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

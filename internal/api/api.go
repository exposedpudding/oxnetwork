package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/exposedpudding/oxnetwork/internal/db"
	"github.com/exposedpudding/oxnetwork/internal/domain"
	"github.com/exposedpudding/oxnetwork/internal/inventory"
	"github.com/exposedpudding/oxnetwork/internal/network"
)

type Server struct {
	DB          *sql.DB
	Repo        inventory.Repository
	WebDir      string
	SiteName    string
	AuthEnabled bool
	Log         *slog.Logger
	Now         func() time.Time
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ready(ctx, s.DB); err != nil {
			writeError(w, r, http.StatusServiceUnavailable, "NOT_READY", "Database is not ready")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/v1/system", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"name": "oxnet.work", "site_name": s.SiteName, "version": "0.1.0-dev", "server_time": s.now(), "features": []string{"demo_provider"}, "demo_mode": true, "auth_required": s.AuthEnabled})
	})
	mux.HandleFunc("GET /api/v1/nodes", func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := s.snapshot(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": snapshot.Nodes})
	})
	mux.HandleFunc("GET /api/v1/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := s.snapshot(w, r)
		if !ok {
			return
		}
		for _, node := range snapshot.Nodes {
			if node.ID == r.PathValue("id") {
				writeJSON(w, http.StatusOK, node)
				return
			}
		}
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Node not found")
	})
	mux.HandleFunc("GET /api/v1/services", func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := s.snapshot(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": snapshot.Services})
	})
	mux.HandleFunc("GET /api/v1/services/{id}", func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := s.snapshot(w, r)
		if !ok {
			return
		}
		for _, service := range snapshot.Services {
			if service.ID == r.PathValue("id") {
				writeJSON(w, http.StatusOK, service)
				return
			}
		}
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Service not found")
	})
	mux.HandleFunc("GET /api/v1/activity", func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := s.snapshot(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": snapshot.Activity})
	})
	mux.HandleFunc("GET /api/v1/nodes/{id}/metrics", func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := s.snapshot(w, r)
		if !ok {
			return
		}
		for _, node := range snapshot.Nodes {
			if node.ID == r.PathValue("id") {
				writeJSON(w, http.StatusOK, node.Metrics)
				return
			}
		}
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Node not found")
	})
	mux.HandleFunc("GET /api/v1/compute/workers", func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := s.snapshot(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": workers(snapshot.Nodes)})
	})
	mux.HandleFunc("GET /api/v1/compute/summary", func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := s.snapshot(w, r)
		if !ok {
			return
		}
		workers := workers(snapshot.Nodes)
		total := 0
		observedAt := time.Time{}
		freshness := "unknown"
		for _, worker := range workers {
			total += worker.VCPUTotal
			if worker.Health.ObservedAt.After(observedAt) {
				observedAt = worker.Health.ObservedAt
			}
			if worker.Freshness == "stale" {
				freshness = "stale"
			} else if freshness != "stale" {
				freshness = worker.Freshness
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"total_vcpu": total, "used_vcpu": 0, "reserved_vcpu": 0, "active_jobs": 0, "queued_jobs": 0, "observed_at": observedAt, "freshness": freshness, "simulated": true})
	})
	mux.HandleFunc("GET /api/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
	})
	mux.HandleFunc("GET /api/v1/network/graph", func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := s.snapshot(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, network.FromSnapshot(snapshot))
	})
	mux.HandleFunc("GET /api/v1/network/connections", func(w http.ResponseWriter, r *http.Request) {
		snapshot, ok := s.snapshot(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": network.FromSnapshot(snapshot).Edges})
	})
	mux.HandleFunc("GET /api/v1/events", s.events)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not found")
	})
	mux.HandleFunc("/", s.static)
	return s.withMiddleware(mux)
}

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) (domain.Snapshot, bool) {
	snapshot, err := s.Repo.Snapshot(r.Context(), s.now())
	if err != nil {
		if s.Log != nil {
			s.Log.Error("inventory read failed", "error", err)
		}
		writeError(w, r, http.StatusServiceUnavailable, "INVENTORY_UNAVAILABLE", "Inventory is unavailable")
		return domain.Snapshot{}, false
	}
	return snapshot, true
}

type worker struct {
	ID        string        `json:"id"`
	NodeID    string        `json:"node_id"`
	Name      string        `json:"name"`
	State     string        `json:"state"`
	VCPUTotal int           `json:"vcpu_total"`
	VCPUUsed  int           `json:"vcpu_used"`
	Health    domain.Health `json:"health"`
	Freshness string        `json:"freshness"`
	Simulated bool          `json:"simulated"`
}

func workers(nodes []domain.Node) []worker {
	result := []worker{}
	for _, node := range nodes {
		found := false
		for _, capability := range node.Capabilities {
			if capability == "compute" {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		state := "online"
		if node.Freshness != "fresh" || node.Health.State != "healthy" {
			state = "offline"
		}
		capacity := 0
		if node.Metrics.VCPU != nil {
			capacity = *node.Metrics.VCPU
		}
		result = append(result, worker{ID: node.ID, NodeID: node.ID, Name: node.Name, State: state, VCPUTotal: capacity, Health: node.Health, Freshness: node.Freshness, Simulated: node.Simulated})
	}
	return result
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw [12]byte
		if _, err := rand.Read(raw[:]); err != nil {
			http.Error(w, "request ID unavailable", 500)
			return
		}
		id := hex.EncodeToString(raw[:])
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id))
		start := time.Now()
		next.ServeHTTP(w, r)
		if s.Log != nil {
			s.Log.Info("request", "id", id, "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
		}
	})
}

type requestIDKey struct{}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	id, _ := r.Context().Value(requestIDKey{}).(string)
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": id}})
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, 500, "STREAM_UNAVAILABLE", "Streaming unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	_, _ = io.WriteString(w, "event: connected\ndata: {\"mode\":\"demo\"}\n\n")
	flusher.Flush()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, "event: node.updated\ndata: {\"refetch\":true,\"at\":%q}\n\n", s.now().Format(time.RFC3339))
			flusher.Flush()
		}
	}
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.WebDir == "" {
		http.Error(w, "web assets unavailable", 503)
		return
	}
	clean := path.Clean("/" + r.URL.Path)
	file := filepath.Join(s.WebDir, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
	if info, err := os.Stat(file); err == nil && !info.IsDir() {
		http.ServeFile(w, r, file)
		return
	}
	if strings.Contains(path.Base(clean), ".") {
		http.NotFound(w, r)
		return
	}
	index := filepath.Join(s.WebDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		http.Error(w, "web assets unavailable", 503)
		return
	}
	http.ServeFile(w, r, index)
}

package api

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/exposedpudding/oxnetwork/internal/inventory"
)

type inventoryContainerReport = inventory.ContainerReport

// maxContainerReportBytes bounds one container report: 300 containers at well under
// 1 KiB each.
const maxContainerReportBytes = 256 << 10

// dockerReport accepts a node's container list from the opt-in Docker reporter
// (ADR-019). It uses its own token directory, so a leaked host-metrics token cannot
// forge container state. Node identity comes from the path and must already exist in
// inventory; the body only carries the container list.
func (s *Server) dockerReport(w http.ResponseWriter, r *http.Request) {
	if s.DockerAgentTokensDir == "" || s.sourceMode() != "tailscale" {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not available")
		return
	}
	nodeID := r.PathValue("id")
	if !hostNodeIDPattern.MatchString(nodeID) {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Endpoint not available")
		return
	}
	secret, err := os.ReadFile(filepath.Join(s.DockerAgentTokensDir, nodeID))
	token := strings.TrimSpace(string(secret))
	if err != nil || len(token) < 32 {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Agent credential required")
		return
	}
	header := r.Header.Get("Authorization")
	provided := strings.TrimPrefix(header, "Bearer ")
	if provided == "" || provided == header || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Agent credential required")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxContainerReportBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var report inventoryContainerReport
	if err := decoder.Decode(&report); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REPORT", "Invalid container report")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, r, http.StatusBadRequest, "INVALID_REPORT", "Invalid container report")
		return
	}
	if err := report.Validate(); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REPORT", "Invalid container report")
		return
	}
	if err := s.Repo.StoreContainerReport(r.Context(), nodeID, report, s.now()); err != nil {
		if s.Log != nil {
			s.Log.Error("container report storage failed", "error", err)
		}
		writeError(w, r, http.StatusServiceUnavailable, "REPORT_UNAVAILABLE", "Container report unavailable")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// nodeContainers returns one node's latest container list and how fresh it is. A node
// that never reported returns reported=false rather than an error, so the UI can say
// that no reporter is installed.
func (s *Server) nodeContainers(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.snapshot(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	found := false
	for _, node := range snapshot.Nodes {
		if node.ID == id {
			found = true
			break
		}
	}
	if !found {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "Node not found")
		return
	}
	result, err := s.Repo.NodeContainers(r.Context(), id, s.now())
	if err != nil {
		if s.Log != nil {
			s.Log.Error("container read failed", "error", err)
		}
		writeError(w, r, http.StatusServiceUnavailable, "INVENTORY_UNAVAILABLE", "Containers are unavailable")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

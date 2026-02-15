package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/machinemon/machinemon/internal/models"
)

func (s *Server) handleListClients(w http.ResponseWriter, r *http.Request) {
	clients, err := s.store.ListClients()
	if err != nil {
		s.logger.Error("failed to list clients", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if clients == nil {
		clients = []models.ClientWithMetrics{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"clients": clients})
}

func (s *Server) handleGetClient(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	client, err := s.store.GetClient(id)
	if err != nil {
		s.logger.Error("failed to get client", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if client == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "client not found"})
		return
	}

	// Get latest metrics
	metrics, _ := s.store.GetLatestMetrics(id)
	// Get watched processes
	procs, _ := s.store.GetLatestProcessSnapshots(id)
	if procs == nil {
		procs = []models.ProcessSnapshot{}
	}
	// Get latest check snapshots
	checks, _ := s.store.GetLatestCheckSnapshots(id)
	if checks == nil {
		checks = []models.CheckSnapshot{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"client":    client,
		"metrics":   metrics,
		"processes": procs,
		"checks":    checks,
	})
}

func (s *Server) handleDeleteClient(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteClient(id); err != nil {
		s.logger.Error("failed to delete client", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleSetThresholds(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var t models.Thresholds
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := s.store.SetClientThresholds(id, &t); err != nil {
		s.logger.Error("failed to set thresholds", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

type muteRequest struct {
	Muted           bool   `json:"muted"`
	DurationMinutes int    `json:"duration_minutes"`
	Reason          string `json:"reason"`
}

func (s *Server) handleSetMute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req muteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	var until *time.Time
	if req.Muted && req.DurationMinutes > 0 {
		t := time.Now().Add(time.Duration(req.DurationMinutes) * time.Minute)
		until = &t
	}

	if err := s.store.SetClientMute(id, req.Muted, until, req.Reason); err != nil {
		s.logger.Error("failed to set mute", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()
	limit := 500

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	metrics, err := s.store.GetMetrics(id, from, to, limit)
	if err != nil {
		s.logger.Error("failed to get metrics", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if metrics == nil {
		metrics = []models.Metric{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"metrics": metrics})
}

func (s *Server) handleGetProcesses(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	watched, err := s.store.GetWatchedProcesses(id)
	if err != nil {
		s.logger.Error("failed to get watched processes", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if watched == nil {
		watched = []models.WatchedProcess{}
	}

	snapshots, err := s.store.GetLatestProcessSnapshots(id)
	if err != nil {
		s.logger.Error("failed to get process snapshots", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if snapshots == nil {
		snapshots = []models.ProcessSnapshot{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"watched":   watched,
		"snapshots": snapshots,
	})
}

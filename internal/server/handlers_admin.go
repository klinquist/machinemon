package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
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
	alertMutes, _ := s.store.ListClientAlertMutes(id)
	if alertMutes == nil {
		alertMutes = []models.ClientAlertMute{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"client":      client,
		"metrics":     metrics,
		"processes":   procs,
		"checks":      checks,
		"alert_mutes": alertMutes,
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
	if t.OfflineThresholdEnabled != nil && *t.OfflineThresholdEnabled {
		if t.OfflineThresholdMinutes == nil || *t.OfflineThresholdMinutes < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "offline_threshold_minutes must be >= 1 when offline_threshold_enabled is true"})
			return
		}
	}
	if t.OfflineThresholdEnabled == nil && t.OfflineThresholdMinutes != nil && *t.OfflineThresholdMinutes < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "offline_threshold_minutes must be >= 1"})
		return
	}
	if t.MetricConsecutiveEnabled != nil && *t.MetricConsecutiveEnabled {
		if t.MetricConsecutiveCheckins == nil || *t.MetricConsecutiveCheckins < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "metric_consecutive_checkins must be >= 1 when metric_consecutive_enabled is true"})
			return
		}
	}
	if t.MetricConsecutiveEnabled == nil && t.MetricConsecutiveCheckins != nil && *t.MetricConsecutiveCheckins < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "metric_consecutive_checkins must be >= 1"})
		return
	}

	if err := s.store.SetClientThresholds(id, &t); err != nil {
		s.logger.Error("failed to set thresholds", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleClearThresholds(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.SetClientThresholds(id, nil); err != nil {
		s.logger.Error("failed to clear thresholds", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

type setClientNameRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleSetClientName(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req setClientNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if len(name) > 120 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name too long (max 120 chars)"})
		return
	}

	if err := s.store.SetClientCustomName(id, name); err != nil {
		s.logger.Error("failed to set client name", "id", id, "err", err)
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

type scopedMuteRequest struct {
	Muted  bool   `json:"muted"`
	Scope  string `json:"scope"`
	Target string `json:"target"`
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

func (s *Server) handleSetScopedMute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req scopedMuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	scope := strings.TrimSpace(req.Scope)
	target := strings.TrimSpace(req.Target)
	switch scope {
	case "cpu", "memory", "disk":
		target = ""
	case "process", "check":
		if target == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target is required for process/check scope"})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid scope"})
		return
	}

	if err := s.store.SetClientAlertMute(id, scope, target, req.Muted); err != nil {
		s.logger.Error("failed to set scoped mute", "id", id, "scope", scope, "target", target, "muted", req.Muted, "err", err)
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

func (s *Server) handleDeleteProcess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	friendlyName := strings.TrimSpace(r.URL.Query().Get("friendly_name"))
	if friendlyName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "friendly_name is required"})
		return
	}

	if err := s.store.DeleteWatchedProcess(id, friendlyName); err != nil {
		s.logger.Error("failed to delete watched process", "id", id, "friendly_name", friendlyName, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleDeleteCheck(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	friendlyName := strings.TrimSpace(r.URL.Query().Get("friendly_name"))
	checkType := strings.TrimSpace(r.URL.Query().Get("check_type"))
	if friendlyName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "friendly_name is required"})
		return
	}

	if err := s.store.DeleteCheckSnapshots(id, friendlyName, checkType); err != nil {
		s.logger.Error("failed to delete check snapshots", "id", id, "friendly_name", friendlyName, "check_type", checkType, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

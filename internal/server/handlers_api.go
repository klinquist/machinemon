package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/machinemon/machinemon/internal/models"
)

func (s *Server) handleCheckIn(w http.ResponseWriter, r *http.Request) {
	var req models.CheckInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Hostname == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hostname is required"})
		return
	}

	clientID, wasOffline, sessionChanged, err := s.store.UpsertClient(req)
	if err != nil {
		s.logger.Error("failed to upsert client", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := s.store.InsertMetrics(clientID, req.Metrics); err != nil {
		s.logger.Error("failed to insert metrics", "client_id", clientID, "err", err)
	}

	// Always sync watched processes so removed processes stop being monitored.
	if err := s.store.UpsertWatchedProcesses(clientID, req.Processes); err != nil {
		s.logger.Error("failed to upsert watched processes", "client_id", clientID, "err", err)
	}
	if len(req.Processes) > 0 {
		if err := s.store.InsertProcessSnapshots(clientID, req.Processes); err != nil {
			s.logger.Error("failed to insert process snapshots", "client_id", clientID, "err", err)
		}
	}

	if len(req.Checks) > 0 {
		if err := s.store.InsertCheckSnapshots(clientID, req.Checks); err != nil {
			s.logger.Error("failed to insert check snapshots", "client_id", clientID, "err", err)
		}
	}

	// If client was offline, mark it online and notify alert engine
	if wasOffline {
		s.logger.Info("client came back online", "client_id", clientID, "hostname", req.Hostname)
	}

	// Notify alert engine
	if s.alerts != nil {
		s.alerts.NotifyCheckIn(clientID)
		if sessionChanged {
			s.logger.Info("client session changed (restart detected)", "client_id", clientID, "hostname", req.Hostname)
			s.alerts.NotifyRestart(clientID, req.Hostname)
		}
	}

	writeJSON(w, http.StatusOK, models.CheckInResponse{
		ClientID:           clientID,
		NextCheckInSeconds: 120,
		ServerTime:         time.Now().UTC(),
	})
}

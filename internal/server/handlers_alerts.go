package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/machinemon/machinemon/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	severity := r.URL.Query().Get("severity")
	limit := 100
	offset := 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	alerts, total, err := s.store.ListAlerts(clientID, severity, limit, offset)
	if err != nil {
		s.logger.Error("failed to list alerts", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if alerts == nil {
		alerts = []models.Alert{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := s.store.ListProviders()
	if err != nil {
		s.logger.Error("failed to list providers", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if providers == nil {
		providers = []models.AlertProvider{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"providers": providers})
}

func (s *Server) handleCreateProvider(w http.ResponseWriter, r *http.Request) {
	var p models.AlertProvider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if p.Type == "" || p.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type and name are required"})
		return
	}
	if err := s.store.CreateProvider(&p); err != nil {
		s.logger.Error("failed to create provider", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleUpdateProvider(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider id"})
		return
	}

	var p models.AlertProvider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	p.ID = id

	if err := s.store.UpdateProvider(&p); err != nil {
		s.logger.Error("failed to update provider", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider id"})
		return
	}
	if err := s.store.DeleteProvider(id); err != nil {
		s.logger.Error("failed to delete provider", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleTestProvider(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider id"})
		return
	}
	_ = id
	// TODO: Phase 4 - dispatch a test alert through the provider
	writeJSON(w, http.StatusOK, map[string]string{"status": "test alert sent"})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.GetAllSettings()
	if err != nil {
		s.logger.Error("failed to get settings", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	// Don't expose password hashes
	delete(settings, "admin_password_hash")
	delete(settings, "client_password_hash")
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings map[string]string
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Don't allow setting password hashes directly
	delete(settings, "admin_password_hash")
	delete(settings, "client_password_hash")

	for k, v := range settings {
		if err := s.store.SetSetting(k, v); err != nil {
			s.logger.Error("failed to set setting", "key", k, "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

type changePasswordRequest struct {
	Type     string `json:"type"`     // "admin" or "client"
	Password string `json:"password"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password is required"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("failed to hash password", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	switch req.Type {
	case "admin":
		s.cfg.AdminPasswordHash = string(hash)
	case "client":
		s.cfg.ClientPasswordHash = string(hash)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type must be 'admin' or 'client'"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

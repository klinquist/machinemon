package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/machinemon/machinemon/internal/store"
)

// AlertNotifier is implemented by the alert engine to receive check-in notifications.
type AlertNotifier interface {
	NotifyCheckIn(clientID string)
}

type Server struct {
	cfg         *Config
	store       store.Store
	router      chi.Router
	alerts      AlertNotifier
	logger      *slog.Logger
	rateLimiter *rateLimiter
}

func New(cfg *Config, st store.Store, alerts AlertNotifier, logger *slog.Logger) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Allow 30 check-ins per minute per IP (generous for multi-client hosts)
	rl := newRateLimiter(2*time.Second, 30)

	s := &Server{
		cfg:         cfg,
		store:       st,
		router:      r,
		alerts:      alerts,
		logger:      logger,
		rateLimiter: rl,
	}

	// Client API
	r.Route("/api/v1", func(r chi.Router) {
		r.With(rl.middleware, s.clientPasswordAuth).Post("/checkin", s.handleCheckIn)

		// Admin API
		r.Route("/admin", func(r chi.Router) {
			r.Use(s.adminBasicAuth)

			// Clients
			r.Get("/clients", s.handleListClients)
			r.Get("/clients/{id}", s.handleGetClient)
			r.Delete("/clients/{id}", s.handleDeleteClient)
			r.Put("/clients/{id}/thresholds", s.handleSetThresholds)
			r.Put("/clients/{id}/mute", s.handleSetMute)
			r.Get("/clients/{id}/metrics", s.handleGetMetrics)
			r.Get("/clients/{id}/processes", s.handleGetProcesses)

			// Alerts
			r.Get("/alerts", s.handleListAlerts)

			// Providers
			r.Get("/providers", s.handleListProviders)
			r.Post("/providers", s.handleCreateProvider)
			r.Put("/providers/{id}", s.handleUpdateProvider)
			r.Delete("/providers/{id}", s.handleDeleteProvider)
			r.Post("/providers/{id}/test", s.handleTestProvider)

			// Settings
			r.Get("/settings", s.handleGetSettings)
			r.Put("/settings", s.handleUpdateSettings)
			r.Put("/password", s.handleChangePassword)
		})
	})

	// Health check (no auth)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Binary downloads (no auth — public so install scripts work)
	r.Route("/download", func(r chi.Router) {
		r.Get("/", s.handleListDownloads)
		r.Get("/install.sh", s.handleDownloadInstallScript)
		r.Get("/{filename}", s.handleDownloadBinary)
	})

	// SPA (serves React dashboard)
	r.Get("/*", s.serveSPA)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) ListenAndServe() error {
	s.logger.Info("starting server", "addr", s.cfg.ListenAddr, "tls", s.cfg.TLSMode)
	return http.ListenAndServe(s.cfg.ListenAddr, s.router)
}

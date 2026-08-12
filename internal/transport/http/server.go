// Package httptransport provides the HTTP REST gateway and observability endpoints.
package httptransport

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/chirag127/go-vault/internal/domain"
	"github.com/chirag127/go-vault/internal/metrics"
	"github.com/chirag127/go-vault/internal/service"
)

// Server wraps the HTTP mux and its dependencies.
type Server struct {
	svc *service.ShortenerService
	log *slog.Logger
}

// New returns a configured HTTP server.
func New(svc *service.ShortenerService, log *slog.Logger) *Server {
	return &Server{svc: svc, log: log}
}

// Handler returns the root http.Handler.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(s.metricsMiddleware)

	// Observability
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/healthz", s.healthz)
	r.Get("/readyz", s.readyz)

	// REST API
	r.Route("/v1", func(r chi.Router) {
		r.Post("/links", s.createLink)
		r.Get("/links", s.listLinks)
		r.Get("/links/{code}", s.getLink)
		r.Delete("/links/{code}", s.deleteLink)
		r.Get("/links/{code}/stats", s.stats)
	})

	// Redirect shortcut
	r.Get("/r/{code}", s.resolve)

	return r
}

func (s *Server) createLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OriginalURL string `json:"original_url"`
		CustomCode  string `json:"custom_code,omitempty"`
		TTLSeconds  int64  `json:"ttl_seconds,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	link, err := s.svc.CreateLink(r.Context(), req.OriginalURL, req.CustomCode, req.TTLSeconds)
	if err != nil {
		writeFromDomain(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (s *Server) getLink(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	link, err := s.svc.GetLink(r.Context(), code)
	if err != nil {
		writeFromDomain(w, err)
		return
	}
	writeJSON(w, http.StatusOK, link)
}

func (s *Server) resolve(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	ip := strings.Split(r.RemoteAddr, ":")[0]
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = strings.Split(xff, ",")[0]
	}
	originalURL, err := s.svc.Resolve(r.Context(), code, ip)
	if err != nil {
		writeFromDomain(w, err)
		return
	}
	http.Redirect(w, r, originalURL, http.StatusFound)
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	link, err := s.svc.Stats(r.Context(), code)
	if err != nil {
		writeFromDomain(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code":          link.Code,
		"click_count":   link.ClickCount,
		"last_accessed": link.CreatedAt,
	})
}

func (s *Server) listLinks(w http.ResponseWriter, r *http.Request) {
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	pageToken := r.URL.Query().Get("page_token")
	page, err := s.svc.ListLinks(r.Context(), pageSize, pageToken)
	if err != nil {
		writeFromDomain(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) deleteLink(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if err := s.svc.DeleteLink(r.Context(), code); err != nil {
		writeFromDomain(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		metrics.HTTPRequestDuration.
			WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(ww.Status())).
			Observe(time.Since(start).Seconds())
	})
}

// writeJSON encodes v as JSON to w.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// writeFromDomain maps domain errors to HTTP status codes.
func writeFromDomain(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrExpired):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrCodeConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidURL):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrRateLimited):
		writeError(w, http.StatusTooManyRequests, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

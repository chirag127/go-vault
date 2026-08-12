package httptransport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/chirag127/go-vault/internal/cache"
	"github.com/chirag127/go-vault/internal/config"
	"github.com/chirag127/go-vault/internal/repository"
	"github.com/chirag127/go-vault/internal/service"

	"log/slog"
	"os"
)

func newTestHTTPServer(t *testing.T) *Server {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := cache.New(rdb, 5*time.Minute)
	cfg := &config.Config{
		CodeLength:      7,
		RateLimitMax:    100,
		RateLimitWindow: time.Minute,
	}
	svc := service.New(repository.NewMemory(), c, cfg)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return New(svc, log)
}

func TestHTTP_CreateAndGetLink(t *testing.T) {
	srv := newTestHTTPServer(t)
	h := srv.Handler()

	// POST /v1/links
	body, _ := json.Marshal(map[string]interface{}{"original_url": "https://example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var link map[string]interface{}
	_ = json.NewDecoder(rr.Body).Decode(&link)
	code := link["Code"].(string)
	if code == "" {
		t.Fatal("expected non-empty code in response")
	}

	// GET /v1/links/{code}
	req2 := httptest.NewRequest(http.MethodGet, "/v1/links/"+code, nil)
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("get: want 200, got %d", rr2.Code)
	}
}

func TestHTTP_GetLink_NotFound(t *testing.T) {
	srv := newTestHTTPServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/links/doesnotexist", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

func TestHTTP_DeleteLink(t *testing.T) {
	srv := newTestHTTPServer(t)
	h := srv.Handler()

	body, _ := json.Marshal(map[string]interface{}{"original_url": "https://example.com", "custom_code": "deltst"})
	req := httptest.NewRequest(http.MethodPost, "/v1/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatal("create failed")
	}

	req2 := httptest.NewRequest(http.MethodDelete, "/v1/links/deltst", nil)
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusNoContent {
		t.Errorf("delete: want 204, got %d", rr2.Code)
	}
}

func TestHTTP_Healthz(t *testing.T) {
	srv := newTestHTTPServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
}

func TestHTTP_InvalidURL(t *testing.T) {
	srv := newTestHTTPServer(t)
	body, _ := json.Marshal(map[string]interface{}{"original_url": "not-a-url"})
	req := httptest.NewRequest(http.MethodPost, "/v1/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

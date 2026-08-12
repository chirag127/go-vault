package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/chirag127/go-vault/internal/cache"
	"github.com/chirag127/go-vault/internal/config"
	"github.com/chirag127/go-vault/internal/domain"
	"github.com/chirag127/go-vault/internal/repository"
)

func newTestService(t *testing.T) *ShortenerService {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := cache.New(rdb, 5*time.Minute)
	cfg := &config.Config{
		CodeLength:      7,
		RateLimitMax:    100,
		RateLimitWindow: time.Minute,
	}
	return New(repository.NewMemory(), c, cfg)
}

func TestCreateLink(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	link, err := svc.CreateLink(ctx, "https://example.com", "", 0)
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if link.Code == "" {
		t.Error("expected non-empty code")
	}
	if link.OriginalURL != "https://example.com" {
		t.Errorf("got url %q, want %q", link.OriginalURL, "https://example.com")
	}
	if !link.ExpiresAt.IsZero() {
		t.Errorf("expected zero expiry, got %v", link.ExpiresAt)
	}
}

func TestCreateLink_CustomCode(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	link, err := svc.CreateLink(ctx, "https://example.com", "mycode", 0)
	if err != nil {
		t.Fatalf("CreateLink custom: %v", err)
	}
	if link.Code != "mycode" {
		t.Errorf("got %q, want mycode", link.Code)
	}
}

func TestCreateLink_DuplicateCode(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, err := svc.CreateLink(ctx, "https://example.com", "dup", 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreateLink(ctx, "https://other.com", "dup", 0)
	if err != domain.ErrCodeConflict {
		t.Errorf("want ErrCodeConflict, got %v", err)
	}
}

func TestCreateLink_InvalidURL(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	tests := []string{"not-a-url", "ftp://bad.com", "", "http://"}
	for _, u := range tests {
		_, err := svc.CreateLink(ctx, u, "", 0)
		if err != domain.ErrInvalidURL {
			t.Errorf("url %q: want ErrInvalidURL, got %v", u, err)
		}
	}
}

func TestCreateLink_TTL(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	link, err := svc.CreateLink(ctx, "https://example.com", "", 3600)
	if err != nil {
		t.Fatal(err)
	}
	if link.ExpiresAt.IsZero() {
		t.Error("expected non-zero expiry for TTL link")
	}
	if link.ExpiresAt.Before(time.Now()) {
		t.Error("expiry should be in the future")
	}
}

func TestGetLink(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	created, _ := svc.CreateLink(ctx, "https://example.com", "abc", 0)
	got, err := svc.GetLink(ctx, created.Code)
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	if got.OriginalURL != "https://example.com" {
		t.Errorf("wrong url")
	}
}

func TestGetLink_NotFound(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.GetLink(context.Background(), "nope")
	if err != domain.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestResolve(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	link, _ := svc.CreateLink(ctx, "https://example.com", "res1", 0)
	url, err := svc.Resolve(ctx, link.Code, "127.0.0.1")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if url != "https://example.com" {
		t.Errorf("got %q, want https://example.com", url)
	}
}

func TestResolve_Expired(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	// Create link that expired in the past by calling createLink with past expiry
	link := &domain.Link{
		Code:        "old1",
		OriginalURL: "https://example.com",
		CreatedAt:   time.Now().Add(-2 * time.Hour),
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
	}
	_ = svc.repo.Create(ctx, link)

	_, err := svc.Resolve(ctx, "old1", "127.0.0.1")
	if err != domain.ErrExpired {
		t.Errorf("want ErrExpired, got %v", err)
	}
}

func TestDeleteLink(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	link, _ := svc.CreateLink(ctx, "https://example.com", "del1", 0)
	if err := svc.DeleteLink(ctx, link.Code); err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	_, err := svc.GetLink(ctx, link.Code)
	if err != domain.ErrNotFound {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
}

func TestListLinks(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	for range 5 {
		_, err := svc.CreateLink(ctx, "https://example.com", "", 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	page, err := svc.ListLinks(ctx, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Links) > 3 {
		t.Errorf("page too large: %d", len(page.Links))
	}
}

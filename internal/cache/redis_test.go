package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/chirag127/go-vault/internal/domain"
)

func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return New(rdb, 5*time.Minute), mr
}

func TestGetLink_Miss(t *testing.T) {
	c, _ := newTestClient(t)
	link, err := c.GetLink(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link != nil {
		t.Errorf("expected nil on miss, got %+v", link)
	}
}

func TestSetAndGetLink(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	link := &domain.Link{
		Code:        "abc123",
		OriginalURL: "https://example.com",
		CreatedAt:   time.Now().UTC(),
	}
	if err := c.SetLinkByCode(ctx, link.Code, link); err != nil {
		t.Fatalf("SetLinkByCode: %v", err)
	}

	got, err := c.GetLink(ctx, link.Code)
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	if got == nil {
		t.Fatal("expected cached link, got nil")
	}
	if got.OriginalURL != link.OriginalURL {
		t.Errorf("got %q, want %q", got.OriginalURL, link.OriginalURL)
	}
}

func TestDeleteLink(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	link := &domain.Link{Code: "del1", OriginalURL: "https://example.com", CreatedAt: time.Now().UTC()}
	_ = c.SetLinkByCode(ctx, link.Code, link)

	if err := c.DeleteLink(ctx, link.Code); err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	got, _ := c.GetLink(ctx, link.Code)
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestSetLink_ExpiredLinkSkipped(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	// Link already expired
	link := &domain.Link{
		Code:        "exp1",
		OriginalURL: "https://example.com",
		CreatedAt:   time.Now().Add(-2 * time.Hour).UTC(),
		ExpiresAt:   time.Now().Add(-1 * time.Hour).UTC(),
	}
	// Should return nil error (skip storing) instead of panicking
	if err := c.SetLinkByCode(ctx, link.Code, link); err != nil {
		t.Fatalf("SetLinkByCode with expired link: %v", err)
	}
	// Nothing should be stored (TTL would be negative → skipped)
	got, _ := c.GetLink(ctx, link.Code)
	if got != nil {
		t.Error("expired link should not be stored in cache")
	}
}

func TestCheckRateLimit_AllowsUnderLimit(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	allowed, remaining, err := c.CheckRateLimit(ctx, "1.2.3.4", 10, time.Minute)
	if err != nil {
		t.Fatalf("CheckRateLimit: %v", err)
	}
	if !allowed {
		t.Error("expected allowed on first request")
	}
	if remaining != 9 {
		t.Errorf("expected 9 remaining, got %d", remaining)
	}
}

func TestCheckRateLimit_BlocksOverLimit(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	for i := range 5 {
		allowed, _, err := c.CheckRateLimit(ctx, "9.9.9.9", 5, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if i < 5 && !allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
	// 6th request should be blocked
	allowed, remaining, err := c.CheckRateLimit(ctx, "9.9.9.9", 5, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Error("6th request should be blocked")
	}
	if remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", remaining)
	}
}

func TestCheckRateLimit_FixedWindowExpires(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	// Exhaust the limit
	for range 3 {
		c.CheckRateLimit(ctx, "5.5.5.5", 3, time.Second) //nolint:errcheck
	}
	allowed, _, _ := c.CheckRateLimit(ctx, "5.5.5.5", 3, time.Second)
	if allowed {
		t.Error("4th request should be blocked")
	}

	// Fast-forward miniredis clock past the window
	mr.FastForward(2 * time.Second)

	// Now should be allowed again (new window)
	allowed, _, err := c.CheckRateLimit(ctx, "5.5.5.5", 3, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Error("request after window expiry should be allowed")
	}
}

func TestPing(t *testing.T) {
	c, _ := newTestClient(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

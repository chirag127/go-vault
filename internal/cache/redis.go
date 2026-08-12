// Package cache provides a Redis-backed read-through cache and rate limiter.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/chirag127/go-vault/internal/domain"
)

const (
	linkKeyPrefix      = "link:"
	rateLimitKeyPrefix = "rl:"
)

// Client wraps a Redis client with domain-specific operations.
type Client struct {
	rdb     *redis.Client
	cacheTTL time.Duration
}

// New returns a cache Client connected to Redis.
func New(rdb *redis.Client, cacheTTL time.Duration) *Client {
	return &Client{rdb: rdb, cacheTTL: cacheTTL}
}

// GetLink returns a cached link or (nil, nil) on miss.
func (c *Client) GetLink(ctx context.Context, code string) (*domain.Link, error) {
	data, err := c.rdb.Get(ctx, linkKeyPrefix+code).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil // cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}
	var link domain.Link
	if err := json.Unmarshal(data, &link); err != nil {
		return nil, fmt.Errorf("redis unmarshal: %w", err)
	}
	return &link, nil
}

// DeleteLink removes a link from the cache.
func (c *Client) DeleteLink(ctx context.Context, code string) error {
	return c.rdb.Del(ctx, linkKeyPrefix+code).Err()
}

// SetLinkByCode stores a link in the cache keyed by code.
func (c *Client) SetLinkByCode(ctx context.Context, code string, link *domain.Link) error {
	data, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("redis marshal: %w", err)
	}
	ttl := c.cacheTTL
	if !link.ExpiresAt.IsZero() {
		remaining := time.Until(link.ExpiresAt)
		if remaining < ttl {
			ttl = remaining
		}
	}
	if ttl <= 0 {
		return nil
	}
	return c.rdb.Set(ctx, linkKeyPrefix+code, data, ttl).Err()
}

// CheckRateLimit checks whether ip has exceeded maxRequests in window.
// Returns (allowed, remaining, error).
func (c *Client) CheckRateLimit(ctx context.Context, ip string, maxRequests int, window time.Duration) (bool, int, error) {
	key := rateLimitKeyPrefix + ip
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, maxRequests, fmt.Errorf("redis pipeline: %w", err) // fail open
	}
	count := int(incr.Val())
	remaining := maxRequests - count
	if remaining < 0 {
		remaining = 0
	}
	return count <= maxRequests, remaining, nil
}

// Ping verifies Redis connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

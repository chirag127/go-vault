// Package service implements the URL shortener business logic.
package service

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/chirag127/go-vault/internal/cache"
	"github.com/chirag127/go-vault/internal/codec"
	"github.com/chirag127/go-vault/internal/config"
	"github.com/chirag127/go-vault/internal/domain"
	"github.com/chirag127/go-vault/internal/metrics"
	"github.com/chirag127/go-vault/internal/repository"
)

const maxCodeGenAttempts = 5

// ShortenerService is the core business-logic layer.
type ShortenerService struct {
	repo   repository.LinkRepository
	cache  *cache.Client
	cfg    *config.Config
}

// New returns a ShortenerService wired with its dependencies.
func New(repo repository.LinkRepository, cache *cache.Client, cfg *config.Config) *ShortenerService {
	return &ShortenerService{repo: repo, cache: cache, cfg: cfg}
}

// CreateLink creates a new short link.
func (s *ShortenerService) CreateLink(ctx context.Context, originalURL, customCode string, ttlSeconds int64) (*domain.Link, error) {
	if err := validateURL(originalURL); err != nil {
		return nil, domain.ErrInvalidURL
	}

	code := customCode
	if code == "" {
		var err error
		code, err = s.generateUniqueCode(ctx)
		if err != nil {
			return nil, fmt.Errorf("generate code: %w", err)
		}
	} else {
		if !codec.IsValid(code) {
			return nil, fmt.Errorf("%w: custom code contains invalid characters", domain.ErrInvalidURL)
		}
		exists, err := s.repo.CodeExists(ctx, code)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, domain.ErrCodeConflict
		}
	}

	link := &domain.Link{
		Code:        code,
		OriginalURL: originalURL,
		CreatedAt:   time.Now().UTC(),
	}
	if ttlSeconds > 0 {
		link.ExpiresAt = link.CreatedAt.Add(time.Duration(ttlSeconds) * time.Second)
	}

	if err := s.repo.Create(ctx, link); err != nil {
		return nil, err
	}
	_ = s.cache.SetLinkByCode(ctx, link.Code, link) // best-effort
	metrics.LinksCreated.Inc()
	return link, nil
}

// GetLink returns a link by its short code.
func (s *ShortenerService) GetLink(ctx context.Context, code string) (*domain.Link, error) {
	link, err := s.cachedGet(ctx, code)
	if err != nil {
		return nil, err
	}
	if link.IsExpired() {
		return nil, domain.ErrExpired
	}
	return link, nil
}

// Resolve looks up the original URL, increments click count, enforces rate limit.
func (s *ShortenerService) Resolve(ctx context.Context, code, clientIP string) (string, error) {
	allowed, _, err := s.cache.CheckRateLimit(ctx, clientIP, s.cfg.RateLimitMax, s.cfg.RateLimitWindow)
	if err == nil && !allowed {
		metrics.RateLimitRejections.Inc()
		return "", domain.ErrRateLimited
	}

	link, err := s.cachedGet(ctx, code)
	if err != nil {
		return "", err
	}
	if link.IsExpired() {
		_ = s.cache.DeleteLink(ctx, code) // evict stale entry
		return "", domain.ErrExpired
	}

	// fire-and-forget click increment
	go func() {
		_ = s.repo.IncrementClicks(context.WithoutCancel(ctx), code)
		_ = s.cache.DeleteLink(context.WithoutCancel(ctx), code) // invalidate cached click_count
	}()

	metrics.LinksResolved.Inc()
	return link.OriginalURL, nil
}

// Stats returns click statistics for a link.
func (s *ShortenerService) Stats(ctx context.Context, code string) (*domain.Link, error) {
	// bypass cache for stats — always read from DB
	link, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if link.IsExpired() {
		return nil, domain.ErrExpired
	}
	return link, nil
}

// ListLinks returns a paginated list of links.
func (s *ShortenerService) ListLinks(ctx context.Context, pageSize int, pageToken string) (*domain.ListPage, error) {
	return s.repo.List(ctx, pageSize, pageToken)
}

// DeleteLink removes a link by code.
func (s *ShortenerService) DeleteLink(ctx context.Context, code string) error {
	if err := s.repo.Delete(ctx, code); err != nil {
		return err
	}
	_ = s.cache.DeleteLink(ctx, code)
	return nil
}

// cachedGet is a read-through helper: check cache → fall through to DB → populate cache.
func (s *ShortenerService) cachedGet(ctx context.Context, code string) (*domain.Link, error) {
	cached, err := s.cache.GetLink(ctx, code)
	if err == nil && cached != nil {
		metrics.CacheHits.Inc()
		return cached, nil
	}
	metrics.CacheMisses.Inc()

	link, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	_ = s.cache.SetLinkByCode(ctx, code, link) // best-effort
	return link, nil
}

func (s *ShortenerService) generateUniqueCode(ctx context.Context) (string, error) {
	for range maxCodeGenAttempts {
		code, err := codec.Generate(s.cfg.CodeLength)
		if err != nil {
			return "", err
		}
		exists, err := s.repo.CodeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("could not generate unique code after %d attempts", maxCodeGenAttempts)
}

func validateURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("missing host")
	}
	return nil
}

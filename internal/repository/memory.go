package repository

import (
	"context"
	"sync"

	"github.com/chirag127/go-vault/internal/domain"
)

// memRepo is an in-memory LinkRepository for unit tests.
type memRepo struct {
	mu    sync.RWMutex
	links map[string]*domain.Link
}

// NewMemory returns an in-memory LinkRepository suitable for tests.
func NewMemory() LinkRepository {
	return &memRepo{links: make(map[string]*domain.Link)}
}

func (r *memRepo) Create(_ context.Context, link *domain.Link) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.links[link.Code]; ok {
		return domain.ErrCodeConflict
	}
	cp := *link
	r.links[link.Code] = &cp
	return nil
}

func (r *memRepo) GetByCode(_ context.Context, code string) (*domain.Link, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	l, ok := r.links[code]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *l
	return &cp, nil
}

func (r *memRepo) IncrementClicks(_ context.Context, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.links[code]
	if !ok {
		return domain.ErrNotFound
	}
	l.ClickCount++
	return nil
}

func (r *memRepo) List(_ context.Context, pageSize int, _ string) (*domain.ListPage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if pageSize <= 0 {
		pageSize = 20
	}
	links := make([]*domain.Link, 0, len(r.links))
	for _, l := range r.links {
		cp := *l
		links = append(links, &cp)
		if len(links) >= pageSize {
			break
		}
	}
	return &domain.ListPage{Links: links}, nil
}

func (r *memRepo) Delete(_ context.Context, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.links[code]; !ok {
		return domain.ErrNotFound
	}
	delete(r.links, code)
	return nil
}

func (r *memRepo) CodeExists(_ context.Context, code string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.links[code]
	return ok, nil
}

// Compile-time check: memRepo implements LinkRepository.
var _ LinkRepository = (*memRepo)(nil)

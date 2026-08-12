package repository

import (
	"context"

	"github.com/chirag127/go-vault/internal/domain"
)

// LinkRepository defines persistence operations for links.
type LinkRepository interface {
	Create(ctx context.Context, link *domain.Link) error
	GetByCode(ctx context.Context, code string) (*domain.Link, error)
	IncrementClicks(ctx context.Context, code string) error
	List(ctx context.Context, pageSize int, pageToken string) (*domain.ListPage, error)
	Delete(ctx context.Context, code string) error
	CodeExists(ctx context.Context, code string) (bool, error)
}

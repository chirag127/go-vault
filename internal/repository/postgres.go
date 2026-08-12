package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chirag127/go-vault/internal/domain"
)

// pgRepo is the Postgres implementation of LinkRepository.
type pgRepo struct {
	pool *pgxpool.Pool
}

// NewPostgres returns a LinkRepository backed by Postgres.
func NewPostgres(pool *pgxpool.Pool) LinkRepository {
	return &pgRepo{pool: pool}
}

const createLinkSQL = `
INSERT INTO links (code, original_url, click_count, created_at, expires_at)
VALUES ($1, $2, 0, $3, $4)
`

func (r *pgRepo) Create(ctx context.Context, link *domain.Link) error {
	var expiresAt *time.Time
	if !link.ExpiresAt.IsZero() {
		expiresAt = &link.ExpiresAt
	}
	_, err := r.pool.Exec(ctx, createLinkSQL,
		link.Code, link.OriginalURL, link.CreatedAt, expiresAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCodeConflict
		}
		return fmt.Errorf("pg create: %w", err)
	}
	return nil
}

const getLinkSQL = `
SELECT code, original_url, click_count, created_at, expires_at
FROM links
WHERE code = $1
`

func (r *pgRepo) GetByCode(ctx context.Context, code string) (*domain.Link, error) {
	row := r.pool.QueryRow(ctx, getLinkSQL, code)
	link, err := scanLink(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return link, err
}

const incrClickSQL = `
UPDATE links SET click_count = click_count + 1 WHERE code = $1
`

func (r *pgRepo) IncrementClicks(ctx context.Context, code string) error {
	ct, err := r.pool.Exec(ctx, incrClickSQL, code)
	if err != nil {
		return fmt.Errorf("pg incr clicks: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const listLinksSQL = `
SELECT code, original_url, click_count, created_at, expires_at
FROM links
WHERE ($1 = '' OR code > $1)
ORDER BY code
LIMIT $2
`

func (r *pgRepo) List(ctx context.Context, pageSize int, pageToken string) (*domain.ListPage, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	// fetch one extra to determine if there's a next page
	rows, err := r.pool.Query(ctx, listLinksSQL, pageToken, pageSize+1)
	if err != nil {
		return nil, fmt.Errorf("pg list: %w", err)
	}
	defer rows.Close()

	var links []*domain.Link
	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pg list rows: %w", err)
	}

	var nextToken string
	if len(links) > pageSize {
		nextToken = links[pageSize-1].Code
		links = links[:pageSize]
	}
	return &domain.ListPage{Links: links, NextPageToken: nextToken}, nil
}

const deleteLinkSQL = `DELETE FROM links WHERE code = $1`

func (r *pgRepo) Delete(ctx context.Context, code string) error {
	ct, err := r.pool.Exec(ctx, deleteLinkSQL, code)
	if err != nil {
		return fmt.Errorf("pg delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

const codeExistsSQL = `SELECT EXISTS(SELECT 1 FROM links WHERE code = $1)`

func (r *pgRepo) CodeExists(ctx context.Context, code string) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, codeExistsSQL, code).Scan(&exists); err != nil {
		return false, fmt.Errorf("pg code exists: %w", err)
	}
	return exists, nil
}

// scanLink scans a Link from a pgx row.
type pgxRow interface {
	Scan(dest ...any) error
}

func scanLink(row pgxRow) (*domain.Link, error) {
	var (
		link      domain.Link
		expiresAt *time.Time
	)
	if err := row.Scan(
		&link.Code,
		&link.OriginalURL,
		&link.ClickCount,
		&link.CreatedAt,
		&expiresAt,
	); err != nil {
		return nil, err
	}
	if expiresAt != nil {
		link.ExpiresAt = *expiresAt
	}
	return &link, nil
}

// isUniqueViolation checks whether an error is a Postgres unique constraint violation.
func isUniqueViolation(err error) bool {
	// pgx wraps pgconn.PgError; check the SQLSTATE code 23505
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

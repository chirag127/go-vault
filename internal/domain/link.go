// Package domain defines the core business types for the URL shortener.
package domain

import "time"

// Link is the core entity.
type Link struct {
	Code        string
	OriginalURL string
	ClickCount  int64
	CreatedAt   time.Time
	ExpiresAt   time.Time // zero means no expiry
}

// IsExpired returns true if the link has an expiry set and it has passed.
func (l *Link) IsExpired() bool {
	return !l.ExpiresAt.IsZero() && time.Now().After(l.ExpiresAt)
}

// Click records a single redirect event.
type Click struct {
	LinkCode  string
	ClientIP  string
	Timestamp time.Time
}

// ListPage is a paginated response from the repository.
type ListPage struct {
	Links         []*Link
	NextPageToken string
}

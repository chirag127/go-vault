package domain

import "errors"

var (
	ErrNotFound     = errors.New("link not found")
	ErrExpired      = errors.New("link has expired")
	ErrCodeConflict = errors.New("short code already exists")
	ErrInvalidURL   = errors.New("invalid URL")
	ErrRateLimited  = errors.New("rate limit exceeded")
)

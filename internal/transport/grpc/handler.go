// Package grpctransport implements the gRPC ShortenerService handler.
package grpctransport

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/metadata"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	shortenerv1 "github.com/chirag127/go-vault/api/gen/shortener/v1"
	"github.com/chirag127/go-vault/internal/domain"
	"github.com/chirag127/go-vault/internal/service"
)

// Handler is the gRPC server for ShortenerService.
type Handler struct {
	shortenerv1.UnimplementedShortenerServiceServer
	svc *service.ShortenerService
	log *slog.Logger
}

// New returns a new gRPC Handler.
func New(svc *service.ShortenerService, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) CreateLink(ctx context.Context, req *shortenerv1.CreateLinkRequest) (*shortenerv1.CreateLinkResponse, error) {
	link, err := h.svc.CreateLink(ctx, req.OriginalUrl, req.CustomCode, req.TtlSeconds)
	if err != nil {
		return nil, domainToGRPC(err)
	}
	return &shortenerv1.CreateLinkResponse{Link: linkToProto(link)}, nil
}

func (h *Handler) GetLink(ctx context.Context, req *shortenerv1.GetLinkRequest) (*shortenerv1.GetLinkResponse, error) {
	link, err := h.svc.GetLink(ctx, req.Code)
	if err != nil {
		return nil, domainToGRPC(err)
	}
	return &shortenerv1.GetLinkResponse{Link: linkToProto(link)}, nil
}

func (h *Handler) Resolve(ctx context.Context, req *shortenerv1.ResolveRequest) (*shortenerv1.ResolveResponse, error) {
	ip := clientIP(ctx, req.ClientIp)
	originalURL, err := h.svc.Resolve(ctx, req.Code, ip)
	if err != nil {
		return nil, domainToGRPC(err)
	}
	return &shortenerv1.ResolveResponse{OriginalUrl: originalURL}, nil
}

func (h *Handler) Stats(ctx context.Context, req *shortenerv1.StatsRequest) (*shortenerv1.StatsResponse, error) {
	link, err := h.svc.Stats(ctx, req.Code)
	if err != nil {
		return nil, domainToGRPC(err)
	}
	resp := &shortenerv1.StatsResponse{
		Code:       link.Code,
		ClickCount: link.ClickCount,
	}
	if !link.CreatedAt.IsZero() {
		resp.LastAccessed = timestamppb.New(link.CreatedAt)
	}
	return resp, nil
}

func (h *Handler) ListLinks(ctx context.Context, req *shortenerv1.ListLinksRequest) (*shortenerv1.ListLinksResponse, error) {
	page, err := h.svc.ListLinks(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, domainToGRPC(err)
	}
	resp := &shortenerv1.ListLinksResponse{NextPageToken: page.NextPageToken}
	for _, l := range page.Links {
		resp.Links = append(resp.Links, linkToProto(l))
	}
	return resp, nil
}

func (h *Handler) DeleteLink(ctx context.Context, req *shortenerv1.DeleteLinkRequest) (*shortenerv1.DeleteLinkResponse, error) {
	if err := h.svc.DeleteLink(ctx, req.Code); err != nil {
		return nil, domainToGRPC(err)
	}
	return &shortenerv1.DeleteLinkResponse{Deleted: true}, nil
}

// linkToProto converts a domain.Link to the protobuf Link message.
func linkToProto(l *domain.Link) *shortenerv1.Link {
	pb := &shortenerv1.Link{
		Code:        l.Code,
		OriginalUrl: l.OriginalURL,
		ClickCount:  l.ClickCount,
	}
	if !l.CreatedAt.IsZero() {
		pb.CreatedAt = timestamppb.New(l.CreatedAt)
	}
	if !l.ExpiresAt.IsZero() {
		pb.ExpiresAt = timestamppb.New(l.ExpiresAt)
	}
	return pb
}

// domainToGRPC maps domain errors to gRPC status errors.
func domainToGRPC(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrExpired):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrCodeConflict):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidURL):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrRateLimited):
		return status.Error(codes.ResourceExhausted, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

// clientIP extracts the client IP from gRPC metadata or falls back to the provided value.
func clientIP(ctx context.Context, fallback string) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-forwarded-for"); len(vals) > 0 {
			return vals[0]
		}
		if vals := md.Get("x-real-ip"); len(vals) > 0 {
			return vals[0]
		}
	}
	return fallback
}

package grpctransport

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/chirag127/go-vault/internal/metrics"
)

// UnaryLoggingInterceptor logs each gRPC unary call.
func UnaryLoggingInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		log.LogAttrs(ctx, slog.LevelInfo, "grpc",
			slog.String("method", info.FullMethod),
			slog.String("code", code.String()),
			slog.Duration("latency", time.Since(start)),
		)
		return resp, err
	}
}

// UnaryMetricsInterceptor records Prometheus metrics for each gRPC call.
func UnaryMetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		metrics.GRPCRequestDuration.
			WithLabelValues(info.FullMethod, code.String()).
			Observe(time.Since(start).Seconds())
		return resp, err
	}
}

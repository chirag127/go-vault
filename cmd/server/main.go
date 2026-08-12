// Command server runs the go-vault URL shortener microservice.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	shortenerv1 "github.com/chirag127/go-vault/api/gen/shortener/v1"
	"github.com/chirag127/go-vault/internal/cache"
	"github.com/chirag127/go-vault/internal/config"
	"github.com/chirag127/go-vault/internal/repository"
	"github.com/chirag127/go-vault/internal/service"
	grpctransport "github.com/chirag127/go-vault/internal/transport/grpc"
	httptransport "github.com/chirag127/go-vault/internal/transport/http"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logLevel := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		logLevel = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(log)

	// Postgres
	pgPool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("pgxpool.New: %w", err)
	}
	defer pgPool.Close()

	if err := pgPool.Ping(context.Background()); err != nil {
		return fmt.Errorf("postgres ping: %w", err)
	}
	log.Info("postgres connected")

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer rdb.Close()

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	log.Info("redis connected")

	cacheClient := cache.New(rdb, cfg.CacheTTL)
	repo := repository.NewPostgres(pgPool)
	svc := service.New(repo, cacheClient, cfg)

	// gRPC server
	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpctransport.UnaryLoggingInterceptor(log),
			grpctransport.UnaryMetricsInterceptor(),
		),
	)
	shortenerv1.RegisterShortenerServiceServer(grpcSrv, grpctransport.New(svc, log))

	grpcLis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}

	// HTTP server
	httpSrv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httptransport.New(svc, log).Handler(),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)

	go func() {
		log.Info("grpc listening", slog.String("addr", cfg.GRPCAddr))
		if err := grpcSrv.Serve(grpcLis); err != nil {
			errCh <- fmt.Errorf("grpc serve: %w", err)
		}
	}()

	go func() {
		log.Info("http listening", slog.String("addr", cfg.HTTPAddr))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("http serve: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	// Graceful shutdown
	shutCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	grpcSrv.GracefulStop()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Error("http shutdown", slog.Any("err", err))
	}

	log.Info("shutdown complete")
	return nil
}

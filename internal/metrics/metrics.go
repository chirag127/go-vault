// Package metrics exposes Prometheus metrics for the shortener service.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// GRPCRequestDuration tracks gRPC handler latency.
	GRPCRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "govault",
		Subsystem: "grpc",
		Name:      "request_duration_seconds",
		Help:      "gRPC request latency.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "status"})

	// HTTPRequestDuration tracks HTTP handler latency.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "govault",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request latency.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path", "status"})

	// CacheHits counts Redis cache hits.
	CacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "govault",
		Subsystem: "cache",
		Name:      "hits_total",
		Help:      "Redis cache hits.",
	})

	// CacheMisses counts Redis cache misses.
	CacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "govault",
		Subsystem: "cache",
		Name:      "misses_total",
		Help:      "Redis cache misses.",
	})

	// LinksCreated counts successfully created short links.
	LinksCreated = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "govault",
		Subsystem: "links",
		Name:      "created_total",
		Help:      "Short links created.",
	})

	// LinksResolved counts successful redirects.
	LinksResolved = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "govault",
		Subsystem: "links",
		Name:      "resolved_total",
		Help:      "Short links resolved (redirected).",
	})

	// RateLimitRejections counts rate-limited requests.
	RateLimitRejections = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "govault",
		Subsystem: "ratelimit",
		Name:      "rejections_total",
		Help:      "Requests rejected by rate limiter.",
	})
)

package buildcache

import (
	"context"
	"sync/atomic"
)

// Metrics optionally measures inspection work. It does not count configuration,
// cache or evidence reads, directory entries, or operating-system read syscalls.
// A zero value is ready to use. Do not copy Metrics after its first use.
// Use a new instance to exclude fixture setup or evidence priming from a sample.
type Metrics struct {
	SourceBytes     atomic.Int64
	SourceHashes    atomic.Int64
	ToolBytes       atomic.Int64
	ToolHashes      atomic.Int64
	SourceFileInfos atomic.Int64
}

type metricsContextKey struct{}

// WithMetrics attaches optional counters to this context and its descendants.
// Sharing a Metrics instance across concurrent contexts is safe.
func WithMetrics(ctx context.Context, metrics *Metrics) context.Context {
	return context.WithValue(ctx, metricsContextKey{}, metrics)
}

func contextMetrics(ctx context.Context) *Metrics {
	metrics, _ := ctx.Value(metricsContextKey{}).(*Metrics)
	return metrics
}

// RecordSourceHash records one attempted source content hash and the actual
// bytes copied into its hash, including partial bytes when copying fails.
func RecordSourceHash(ctx context.Context, bytes int64) {
	if metrics := contextMetrics(ctx); metrics != nil {
		metrics.SourceHashes.Add(1)
		metrics.SourceBytes.Add(bytes)
	}
}

// RecordToolHash records one attempted utility content hash and actual copied
// bytes. It must be called at the content-copy site, not from an inspection hint.
func RecordToolHash(ctx context.Context, bytes int64) {
	if metrics := contextMetrics(ctx); metrics != nil {
		metrics.ToolHashes.Add(1)
		metrics.ToolBytes.Add(bytes)
	}
}

// RecordSourceInfo counts a successful source-file metadata lookup performed by
// MetadataDigest. It excludes directory and utility lookups.
func RecordSourceInfo(ctx context.Context) {
	if metrics := contextMetrics(ctx); metrics != nil {
		metrics.SourceFileInfos.Add(1)
	}
}

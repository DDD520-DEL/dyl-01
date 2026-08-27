// Package config holds the tunable settings of the TSDB engine.
package config

// Config is the engine configuration. All durations are expressed in
// milliseconds for consistency with point timestamps.
type Config struct {
	// ShardInterval is the fixed time range (milliseconds) covered by one
	// shard. Points are routed to shards by timestamp / ShardInterval.
	ShardInterval int64

	// Retention is how long (milliseconds) data is kept before a shard is
	// eligible for cleanup.
	Retention int64

	// MaxMemPoints is the maximum number of points a memtable can hold before
	// it must be flushed to storage.
	MaxMemPoints int

	// DownsampleInterval is the target bucket width (milliseconds) used when
	// aggregating raw points into downsampled points. Must be a multiple of
	// ShardInterval and greater than ShardInterval.
	DownsampleInterval int64

	// DownsampleEnabled toggles automatic downsampling of sealed shards.
	DownsampleEnabled bool

	// WALDir is the directory used for write-ahead log segment files.
	WALDir string

	// DataDir is the directory used for compressed storage blocks.
	DataDir string
}

// Default returns a sensible default configuration for a single-node engine.
func Default() Config {
	return Config{
		ShardInterval:      3_600_000, // one hour
		Retention:          86_400_000, // 24 hours
		MaxMemPoints:       50_000,
		DownsampleInterval: 21_600_000, // six hours
		DownsampleEnabled:  true,
		WALDir:             "wal",
		DataDir:            "data",
	}
}

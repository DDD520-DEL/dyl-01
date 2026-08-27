package verifycase

import (
	"testing"

	"github.com/dyl-01/tsdb/internal/config"
	"github.com/dyl-01/tsdb/internal/model"
	"github.com/dyl-01/tsdb/internal/shard"
)

// TestCompactRemovesStaleBlocks verifies that compaction merges blocks without
// leaving stale block files behind: after a reopen, the same point must be
// readable exactly once.
func TestCompactRemovesStaleBlocks(t *testing.T) {
	cfg := config.Default()
	cfg.MaxMemPoints = 4
	dir := t.TempDir()

	base := int64(1_700_000_000_000)
	shardID := base / cfg.ShardInterval
	start := shardID * cfg.ShardInterval
	end := start + cfg.ShardInterval
	s, err := shard.New(shardID, start, end, cfg, dir)
	if err != nil {
		t.Fatalf("new shard: %v", err)
	}
	for i := 0; i < 12; i++ {
		if err := s.Write(model.Point{SeriesID: 1, Timestamp: base + int64(i), Value: float64(i)}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	if err := s.Compact(); err != nil {
		t.Fatalf("compact: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Reopen to force loading blocks from disk.
	s2, err := shard.New(shardID, start, end, cfg, dir)
	if err != nil {
		t.Fatalf("reopen shard: %v", err)
	}
	defer s2.Close()

	points, err := s2.ScanSeries(1, base, base+100)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(points) != 12 {
		t.Fatalf("scan returned %d points after compact+reopen, want 12 (stale blocks leaked)", len(points))
	}
	seen := make(map[int64]bool)
	for _, p := range points {
		if seen[p.Timestamp] {
			t.Fatalf("duplicate timestamp %d after compact", p.Timestamp)
		}
		seen[p.Timestamp] = true
	}
}

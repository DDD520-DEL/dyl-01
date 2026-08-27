package verifycase

import (
	"testing"

	"github.com/dyl-01/tsdb/internal/config"
	"github.com/dyl-01/tsdb/internal/model"
	"github.com/dyl-01/tsdb/internal/shard"
)

// TestScanHasNoDuplicateAfterFlush verifies that after a flush the same point
// is not returned twice (once from the flushed block and once from a stale
// memtable).
func TestScanHasNoDuplicateAfterFlush(t *testing.T) {
	cfg := config.Default()
	cfg.MaxMemPoints = 5
	dir := t.TempDir()

	base := int64(1_700_000_000_000)
	shardID := base / cfg.ShardInterval
	start := shardID * cfg.ShardInterval
	end := start + cfg.ShardInterval
	s, err := shard.New(shardID, start, end, cfg, dir)
	if err != nil {
		t.Fatalf("new shard: %v", err)
	}
	defer s.Close()

	for i := 0; i < 10; i++ {
		if err := s.Write(model.Point{SeriesID: 1, Timestamp: base + int64(i), Value: float64(i)}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	if err := s.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	points, err := s.ScanSeries(1, base, base+100)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(points) != 10 {
		t.Fatalf("scan returned %d points after flush, want 10 (duplicates leaked)", len(points))
	}
	seen := make(map[int64]bool)
	for _, p := range points {
		if seen[p.Timestamp] {
			t.Fatalf("duplicate timestamp %d after flush", p.Timestamp)
		}
		seen[p.Timestamp] = true
	}
}

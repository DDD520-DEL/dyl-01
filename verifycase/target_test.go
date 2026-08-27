package verifycase

import (
	"testing"
	"time"

	"github.com/dyl-01/tsdb/internal/clock"
	"github.com/dyl-01/tsdb/internal/config"
	"github.com/dyl-01/tsdb/internal/store"
)

// TestQueryAcrossShardBoundary verifies that a range query spanning two shards
// returns every point on both sides of the shard boundary, exactly once.
func TestQueryAcrossShardBoundary(t *testing.T) {
	cfg := config.Default()
	cfg.ShardInterval = 1000
	cfg.MaxMemPoints = 100

	dir := t.TempDir()
	s, err := store.New(cfg, clock.NewMock(time.Date(2023, 11, 14, 0, 0, 0, 0, time.UTC)), dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	base := int64(1_700_000_000_000)
	// Write points straddling the shard boundary at base+999 and base+1000.
	values := map[int64]float64{
		base + 998: 1,
		base + 999: 2, // last point of shard 0
		base + 1000: 3, // first point of shard 1
		base + 1001: 4,
	}
	for ts, v := range values {
		if err := s.Write("m", nil, ts, v); err != nil {
			t.Fatalf("write %d: %v", ts, err)
		}
	}

	points, err := s.Query("m", nil, base+998, base+1001)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(points) != 4 {
		t.Fatalf("query returned %d points, want 4 (boundary point lost)", len(points))
	}
	for _, p := range points {
		want, ok := values[p.Timestamp]
		if !ok || p.Value != want {
			t.Fatalf("unexpected point ts=%d value=%f", p.Timestamp, p.Value)
		}
	}
}

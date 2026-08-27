package verifycase

import (
	"testing"
	"time"

	"github.com/dyl-01/tsdb/internal/clock"
	"github.com/dyl-01/tsdb/internal/config"
	"github.com/dyl-01/tsdb/internal/store"
)

// TestRetentionKeepsActiveShards verifies that retention removes only shards
// whose entire range has aged out, leaving active shards queryable.
func TestRetentionKeepsActiveShards(t *testing.T) {
	cfg := config.Default()
	cfg.ShardInterval = 1000
	cfg.Retention = 5000

	now := time.Date(2023, 11, 14, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	s, err := store.New(cfg, clock.NewMock(now), dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	base := now.UnixMilli()
	if err := s.Write("m", nil, base, 1); err != nil {
		t.Fatalf("write active: %v", err)
	}
	// A far-past point lands in a fully expired shard.
	if err := s.Write("m", nil, base-100_000, 2); err != nil {
		t.Fatalf("write expired: %v", err)
	}

	if _, err := s.EnforceRetention(); err != nil {
		t.Fatalf("enforce retention: %v", err)
	}

	points, err := s.Query("m", nil, base, base+1)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(points) != 1 || points[0].Value != 1 {
		t.Fatalf("active shard lost after retention: got %d points", len(points))
	}
}

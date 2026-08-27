package verifycase

import (
	"math"
	"testing"
	"time"

	"github.com/dyl-01/tsdb/internal/clock"
	"github.com/dyl-01/tsdb/internal/config"
	"github.com/dyl-01/tsdb/internal/store"
)

// TestWriteBatchAllOrNothing verifies that a batch containing one invalid
// point writes nothing at all, including the otherwise-valid points.
func TestWriteBatchAllOrNothing(t *testing.T) {
	cfg := config.Default()
	cfg.ShardInterval = 1000
	dir := t.TempDir()
	s, err := store.New(cfg, clock.NewMock(time.Date(2023, 11, 14, 0, 0, 0, 0, time.UTC)), dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	base := int64(1_700_000_000_000)
	batch := []store.BatchPoint{
		{Name: "a", Timestamp: base, Value: 1},
		{Name: "b", Timestamp: base + 1, Value: math.NaN()}, // invalid
		{Name: "c", Timestamp: base + 2, Value: 3},
	}
	if err := s.WriteBatch(batch); err == nil {
		t.Fatalf("expected error for NaN value in batch")
	}

	for _, name := range []string{"a", "c"} {
		points, err := s.Query(name, nil, base-10, base+10)
		if err != nil {
			t.Fatalf("query %s: %v", name, err)
		}
		if len(points) != 0 {
			t.Fatalf("batch partially committed: series %s has %d points", name, len(points))
		}
	}
}

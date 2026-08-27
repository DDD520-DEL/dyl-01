package store

import (
	"testing"
	"time"

	"github.com/dyl-01/tsdb/internal/clock"
	"github.com/dyl-01/tsdb/internal/config"
	"github.com/dyl-01/tsdb/internal/model"
)

// countByTs counts how many points share each (SeriesID, Timestamp) key.
// Any value > 1 is a duplicate point that should not exist.
func countByTs(pts model.PointList) map[model.Point]int {
	m := make(map[model.Point]int, len(pts))
	for _, p := range pts {
		m[p]++
	}
	return m
}

// TestCompactThenReopenNoDuplicates reproduces the reported bug: after
// compaction, reopening the shard must not resurrect duplicate points. Before
// the fix, two defects combined to duplicate every point on reopen:
//   - loadBlocks read each block file twice (a duplicated loop);
//   - Compact skipped deleting the first old block file (i == 0 continue), so
//     one stale old block survived alongside the new merged block and was
//     loaded together with it on reopen.
//
// The expected invariant: the point set observed right after compaction is
// byte-for-byte identical to the set observed after closing and reopening the
// store (same count, same values, zero duplicates).
//
// The test uses QueryAll, which scans block contents directly without going
// through the (non-persisted) series index, so a fresh reopen can still read
// the flushed data and expose any duplicates.
func TestCompactThenReopenNoDuplicates(t *testing.T) {
	dir := t.TempDir()

	cfg := config.Default()
	cfg.ShardInterval = 1_000 // 1s per shard, so ts/interval maps cleanly
	cfg.MaxMemPoints = 2      // flush after every 2 writes -> multiple small blocks
	cfg.DownsampleEnabled = false
	// Compact many small blocks into one big block per series.
	cfg.Retention = 1_000_000

	clk := clock.NewMock(time.UnixMilli(0))

	s, err := New(cfg, clk, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	const seriesName = "cpu"

	// Write several batches so each flush produces a separate small block per
	// series. With MaxMemPoints=2, writes 1..2 flush block A, 3..4 flush B.
	for i, ts := range []int64{10, 20, 30, 40} {
		if err := s.Write(seriesName, map[string]string{"host": "a"}, ts, float64(i)); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	// All writes are within the first shard (ts in [0,1000)).
	beforePts, err := s.QueryAll(0, 1000)
	if err != nil {
		t.Fatalf("QueryAll before compact: %v", err)
	}
	if len(beforePts) != 4 {
		t.Fatalf("expected 4 points before compact, got %d", len(beforePts))
	}

	// Compact the shard: merges the small blocks into one block per series and
	// must delete all the old small block files.
	if err := s.CompactShard(10); err != nil {
		t.Fatalf("CompactShard: %v", err)
	}

	afterPts, err := s.QueryAll(0, 1000)
	if err != nil {
		t.Fatalf("QueryAll after compact: %v", err)
	}
	if len(afterPts) != 4 {
		t.Fatalf("expected 4 points after compact, got %d (compaction changed the result)", len(afterPts))
	}
	for k, n := range countByTs(afterPts) {
		if n > 1 {
			t.Fatalf("duplicate point after compact (in-memory): %v seen %d times", k, n)
		}
	}

	// Close and reopen from the same data directory. This re-runs loadBlocks,
	// which is where the duplicate-load bug and the leftover-old-block bug
	// surfaced.
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	s2, err := New(cfg, clk, dir)
	if err != nil {
		t.Fatalf("reopen New: %v", err)
	}
	defer s2.Close()

	reopenedPts, err := s2.QueryAll(0, 1000)
	if err != nil {
		t.Fatalf("QueryAll after reopen: %v", err)
	}
	if len(reopenedPts) != 4 {
		t.Fatalf("expected 4 points after reopen, got %d (duplicate load or leftover old block)", len(reopenedPts))
	}
	for k, n := range countByTs(reopenedPts) {
		if n > 1 {
			t.Fatalf("duplicate point after reopen: %v seen %d times (old block not cleaned)", k, n)
		}
	}

	// The compacted shard must have collapsed to exactly one block per series
	// (one series here), proving the old small blocks were removed from disk.
	infos := s2.ShardInfos()
	if len(infos) != 1 {
		t.Fatalf("expected 1 shard, got %d", len(infos))
	}
	if infos[0].Blocks != 1 {
		t.Fatalf("expected 1 block after compact+reopen, got %d (old blocks not deleted)", infos[0].Blocks)
	}
}

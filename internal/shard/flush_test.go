package shard

import (
	"testing"

	"github.com/dyl-01/tsdb/internal/config"
	"github.com/dyl-01/tsdb/internal/model"
)

// newTestShard builds an in-memory shard rooted under t.TempDir().
func newTestShard(t *testing.T, maxPoints int) *Shard {
	t.Helper()
	cfg := config.Config{ShardInterval: 1000, Retention: 1_000_000, MaxMemPoints: maxPoints}
	dir := t.TempDir()
	sh, err := New(0, 0, cfg.ShardInterval, cfg, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = sh.Close() })
	return sh
}

// TestFlushDoesNotDuplicatePoints reproduces the reported bug: after the
// memtable is flushed into a compressed block, a subsequent query must return
// each point exactly once, and the memtable must be drained.
func TestFlushDoesNotDuplicatePoints(t *testing.T) {
	const seriesID uint64 = 1
	sh := newTestShard(t, 4)

	// Fill the memtable up to its flush threshold with one point per timestamp.
	for i := 0; i < 4; i++ {
		if err := sh.Write(model.Point{SeriesID: seriesID, Timestamp: int64(i), Value: float64(i)}); err != nil {
			t.Fatalf("Write(%d): %v", i, err)
		}
	}

	// Writing the threshold-th point triggered an internal flush: the points
	// must now live in a block and the memtable must be empty.
	if sh.mem.Len() != 0 {
		t.Fatalf("memtable not drained after flush: len=%d", sh.mem.Len())
	}
	if sh.BlockCount() == 0 {
		t.Fatalf("no blocks written after flush")
	}

	// Scan the whole shard range. Each timestamp must appear exactly once.
	got, err := sh.Scan(0, 3)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	seen := make(map[int64]int)
	for _, p := range got {
		seen[p.Timestamp]++
	}
	for ts := int64(0); ts < 4; ts++ {
		if seen[ts] != 1 {
			t.Fatalf("timestamp %d returned %d times, want 1", ts, seen[ts])
		}
	}
}

// TestExplicitFlushDoesNotDuplicatePoints covers Flush() called explicitly
// (the sealed/open path) rather than via the write-triggered flush.
func TestExplicitFlushDoesNotDuplicatePoints(t *testing.T) {
	const seriesID uint64 = 7
	sh := newTestShard(t, 64) // threshold high enough that writes never auto-flush

	for i := 0; i < 5; i++ {
		if err := sh.Write(model.Point{SeriesID: seriesID, Timestamp: int64(i), Value: float64(i)}); err != nil {
			t.Fatalf("Write(%d): %v", i, err)
		}
	}
	if err := sh.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if sh.mem.Len() != 0 {
		t.Fatalf("memtable not drained after explicit Flush: len=%d", sh.mem.Len())
	}

	got, err := sh.Scan(0, 4)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	seen := make(map[int64]int)
	for _, p := range got {
		seen[p.Timestamp]++
	}
	if len(got) != 5 {
		t.Fatalf("got %d points, want 5", len(got))
	}
	for ts := int64(0); ts < 5; ts++ {
		if seen[ts] != 1 {
			t.Fatalf("timestamp %d returned %d times, want 1", ts, seen[ts])
		}
	}
}

// TestFlushThenWriteDoesNotDuplicateNewPoints ensures that after a flush,
// fresh writes land only in the memtable and are not shadowed by stale
// flushed copies returned again by the next query.
func TestFlushThenWriteDoesNotDuplicateNewPoints(t *testing.T) {
	const seriesID uint64 = 1
	sh := newTestShard(t, 2)

	// ts 0,1 -> flush
	for _, ts := range []int64{0, 1} {
		if err := sh.Write(model.Point{SeriesID: seriesID, Timestamp: ts, Value: 1}); err != nil {
			t.Fatalf("Write(%d): %v", ts, err)
		}
	}
	// ts 2,3 -> flush again (second block)
	for _, ts := range []int64{2, 3} {
		if err := sh.Write(model.Point{SeriesID: seriesID, Timestamp: ts, Value: 1}); err != nil {
			t.Fatalf("Write(%d): %v", ts, err)
		}
	}
	// One more point that stays live in the memtable.
	if err := sh.Write(model.Point{SeriesID: seriesID, Timestamp: 4, Value: 1}); err != nil {
		t.Fatalf("Write(4): %v", err)
	}

	got, err := sh.Scan(0, 4)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	seen := make(map[int64]int)
	for _, p := range got {
		seen[p.Timestamp]++
	}
	for ts := int64(0); ts <= 4; ts++ {
		if seen[ts] != 1 {
			t.Fatalf("timestamp %d returned %d times, want 1", ts, seen[ts])
		}
	}
}

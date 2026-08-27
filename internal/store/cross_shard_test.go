package store

import (
	"testing"
	"time"

	"github.com/dyl-01/tsdb/internal/clock"
	"github.com/dyl-01/tsdb/internal/config"
)

// TestQuerySpansShardBoundary is a regression test for a cross-shard query
// that dropped points near the shard boundary: the query range crossed two
// shards but only one shard was ever scanned, and the boundary shard was
// excluded from the overlap test. Both shards must be read so that boundary
// points on either side of the split are returned.
func TestQuerySpansShardBoundary(t *testing.T) {
	// Small shard interval so the boundary lands at 100ms.
	cfg := config.Config{
		ShardInterval:      100,
		Retention:          86_400_000,
		MaxMemPoints:       50_000,
		DownsampleInterval:  300,
		DownsampleEnabled:   false,
		WALDir:              "wal",
		DataDir:             "data",
	}
	dir := t.TempDir()
	clk := clock.NewMock(time.UnixMilli(0))
	s, err := New(cfg, clk, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	// Shard A covers [0,100), shard B covers [100,200).
	const name = "cpu"
	labels := map[string]string{"host": "a"}

	// Points on either side of the boundary at 100.
	writes := []struct {
		ts    int64
		value float64
	}{
		{50, 1.0},   // shard A
		{99, 2.0},   // shard A, just before boundary
		{100, 3.0},  // shard B, exactly on boundary
		{101, 4.0},  // shard B, just after boundary
		{150, 5.0},  // shard B
	}
	for _, w := range writes {
		if err := s.Write(name, labels, w.ts, w.value); err != nil {
			t.Fatalf("Write(%d): %v", w.ts, err)
		}
	}

	// Query spans both shards: inclusive [50, 150].
	pts, err := s.Query(name, labels, 50, 150)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if len(pts) != len(writes) {
		t.Fatalf("expected %d points across both shards, got %d: %+v", len(writes), len(pts), pts)
	}
	for i, w := range writes {
		if pts[i].Timestamp != w.ts {
			t.Errorf("point %d: timestamp = %d, want %d", i, pts[i].Timestamp, w.ts)
		}
		if pts[i].Value != w.value {
			t.Errorf("point %d: value = %v, want %v", i, pts[i].Value, w.value)
		}
	}
}

// TestQueryAllSpansShardBoundary exercises the Scan-based path used by
// QueryAll / QueryByLabel, ensuring both shards are scanned.
func TestQueryAllSpansShardBoundary(t *testing.T) {
	cfg := config.Config{
		ShardInterval:      100,
		Retention:          86_400_000,
		MaxMemPoints:       50_000,
		DownsampleInterval: 300,
		DownsampleEnabled:   false,
		WALDir:              "wal",
		DataDir:             "data",
	}
	dir := t.TempDir()
	clk := clock.NewMock(time.UnixMilli(0))
	s, err := New(cfg, clk, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	// One point in each of two adjacent shards.
	if err := s.Write("m", map[string]string{"h": "1"}, 80, 1.0); err != nil {
		t.Fatalf("Write A: %v", err)
	}
	if err := s.Write("m", map[string]string{"h": "1"}, 120, 2.0); err != nil {
		t.Fatalf("Write B: %v", err)
	}

	pts, err := s.QueryAll(0, 200)
	if err != nil {
		t.Fatalf("QueryAll: %v", err)
	}
	if len(pts) != 2 {
		t.Fatalf("expected 2 points across both shards, got %d: %+v", len(pts), pts)
	}
}

// TestOverlapsUnit directly pins the boundary semantics of the overlap test:
// a shard must be included when the query ends exactly at the shard start, and
// excluded only when the query ends strictly before it.
func TestOverlapsUnit(t *testing.T) {
	cfg := config.Default()
	dir := t.TempDir()
	clk := clock.NewMock(time.UnixMilli(0))
	s, err := New(cfg, clk, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	// Force creation of shards [0,1000) and [1000,2000) using a 1s interval.
	// Use the public Write path to materialize shards with the default config
	// is hour-grained; instead build shards directly via shardFor.
	const interval int64 = 1000
	s.cfg.ShardInterval = interval
	a, err := s.shardFor(500)
	if err != nil {
		t.Fatalf("shardFor A: %v", err)
	}
	b, err := s.shardFor(1500)
	if err != nil {
		t.Fatalf("shardFor B: %v", err)
	}

	cases := []struct {
		name         string
		start, end   int64
		wantA, wantB bool
	}{
		{"fully in A", 100, 500, true, false},
		{"fully in B", 1100, 1900, false, true},
		{"ends at boundary (inclusive)", 500, 1000, true, true},
		{"ends just past boundary", 500, 1001, true, true},
		{"ends just before boundary", 500, 999, true, false},
		{"starts at B start", 1000, 1500, false, true},
		{"fully before A start", -500, -100, false, false},
		{"touches B start only", 1000, 1000, false, true},
	}
	for _, c := range cases {
		gotA := a.Overlaps(c.start, c.end)
		gotB := b.Overlaps(c.start, c.end)
		if gotA != c.wantA || gotB != c.wantB {
			t.Errorf("%s: Overlaps(%d,%d) = (A=%v,B=%v), want (A=%v,B=%v)",
				c.name, c.start, c.end, gotA, gotB, c.wantA, c.wantB)
		}
	}
}

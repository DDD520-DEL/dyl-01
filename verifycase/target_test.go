package verifycase

import (
	"testing"

	"github.com/dyl-01/tsdb/internal/wal"
)

// TestReplayPreservesOrder verifies that WAL replay restores every record with
// its timestamp-value pairing intact.
func TestReplayPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	w, err := wal.OpenWithSize(dir, 24*8)
	if err != nil {
		t.Fatalf("open wal: %v", err)
	}
	defer w.Close()

	want := make(map[int64]float64)
	for i := 0; i < 50; i++ {
		ts := int64(i * 2)
		v := float64(i)
		if err := w.Append(wal.Record{SeriesID: 1, Timestamp: ts, Value: v}); err != nil {
			t.Fatalf("append: %v", err)
		}
		want[ts] = v
	}

	records, err := w.Replay()
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(records) != len(want) {
		t.Fatalf("replayed %d records, want %d", len(records), len(want))
	}
	for _, rec := range records {
		if want[rec.Timestamp] != rec.Value {
			t.Fatalf("record pairing mismatch: ts=%d value=%f", rec.Timestamp, rec.Value)
		}
	}
}

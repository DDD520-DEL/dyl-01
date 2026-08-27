package verifycase

import (
	"testing"

	"github.com/dyl-01/tsdb/internal/wal"
)

// TestWALRotateKeepsAllRecords verifies that rotating WAL segments does not
// lose or reorder any record: every appended record must be replayed exactly
// once, in order, with the correct value.
func TestWALRotateKeepsAllRecords(t *testing.T) {
	dir := t.TempDir()
	// A tiny segment size forces many rotations across the writes.
	w, err := wal.OpenWithSize(dir, 24*10)
	if err != nil {
		t.Fatalf("open wal: %v", err)
	}
	defer w.Close()

	const total = 100
	for i := 0; i < total; i++ {
		rec := wal.Record{SeriesID: uint64(i % 5), Timestamp: int64(i), Value: float64(i)}
		if err := w.Append(rec); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	records, err := w.Replay()
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(records) != total {
		t.Fatalf("replayed %d records, want %d", len(records), total)
	}
	for i, rec := range records {
		if rec.Timestamp != int64(i) || rec.Value != float64(i) {
			t.Fatalf("record %d mismatch: got %+v", i, rec)
		}
	}
}

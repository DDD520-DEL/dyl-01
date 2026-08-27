package verifycase

import (
	"os"
	"testing"

	"github.com/dyl-01/tsdb/internal/model"
	"github.com/dyl-01/tsdb/internal/storage"
)

// TestReadBlockRejectsCorruption verifies that a block whose bytes have been
// tampered with is rejected on read instead of silently decoded.
func TestReadBlockRejectsCorruption(t *testing.T) {
	dir := t.TempDir()
	points := model.PointList{{SeriesID: 1, Timestamp: 100, Value: 1.5}}
	path, err := storage.WriteBlock(dir, storage.Block{SeriesID: 1, Start: 100, End: 100, Points: points})
	if err != nil {
		t.Fatalf("write block: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw: %v", err)
	}
	data[len(data)-1] ^= 0xFF
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("rewrite corrupted: %v", err)
	}

	if _, err := storage.ReadBlock(path); err == nil {
		t.Fatalf("ReadBlock accepted a corrupted block")
	}
}

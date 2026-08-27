package verifycase

import (
	"testing"

	"github.com/dyl-01/tsdb/internal/downsample"
	"github.com/dyl-01/tsdb/internal/model"
)

// TestDownsampleBucketBoundary verifies that points exactly on a bucket edge
// land in the correct bucket, and each bucket aggregates only its own points.
func TestDownsampleBucketBoundary(t *testing.T) {
	const interval int64 = 100
	d := downsample.New(interval)

	points := model.PointList{
		{SeriesID: 1, Timestamp: 100, Value: 1}, // bucket 100
		{SeriesID: 1, Timestamp: 199, Value: 2}, // bucket 100
		{SeriesID: 1, Timestamp: 200, Value: 3}, // bucket 200
		{SeriesID: 1, Timestamp: 299, Value: 5}, // bucket 200
	}

	got := d.Downsample(points)
	if len(got) != 2 {
		t.Fatalf("got %d buckets, want 2", len(got))
	}
	if got[0].Timestamp != 100 || got[0].Value != 1.5 {
		t.Fatalf("bucket 1 wrong: ts=%d value=%f", got[0].Timestamp, got[0].Value)
	}
	if got[1].Timestamp != 200 || got[1].Value != 4 {
		t.Fatalf("bucket 2 wrong: ts=%d value=%f", got[1].Timestamp, got[1].Value)
	}
}

package downsample

import (
	"testing"

	"github.com/dyl-01/tsdb/internal/model"
)

func pointsAt(seriesID uint64, vals map[int64]float64) model.PointList {
	out := make(model.PointList, 0, len(vals))
	for ts, v := range vals {
		out = append(out, model.Point{SeriesID: seriesID, Timestamp: ts, Value: v})
	}
	return out
}

// Downsample must floor timestamps to the bucket start; a point landing
// exactly on a boundary belongs to that bucket, not the next one.
func TestDownsampleFloorsBucketBoundary(t *testing.T) {
	// Interval 100ms. Points at 0,100,200,300 each fall on a boundary and
	// must each start its own bucket.
	d := New(100)
	in := pointsAt(1, map[int64]float64{
		0:   1,
		100: 2,
		200: 3,
		300: 4,
	})
	out := d.DownsampleWith(in, AggAvg)

	if len(out) != 4 {
		t.Fatalf("expected 4 buckets, got %d", len(out))
	}
	want := []struct {
		ts    int64
		value float64
	}{{0, 1}, {100, 2}, {200, 3}, {300, 4}}
	for i, w := range want {
		if out[i].Timestamp != w.ts {
			t.Errorf("bucket %d: timestamp=%d want %d", i, out[i].Timestamp, w.ts)
		}
		if out[i].Value != w.value {
			t.Errorf("bucket %d: value=%v want %v", i, out[i].Value, w.value)
		}
	}
}

// Points inside a window must aggregate into the floored bucket with the
// correct mean, and the bucket timestamp must be the bucket start.
func TestDownsampleAveragesWithinBucket(t *testing.T) {
	d := New(100)
	// Bucket [100,200): points at 100,120,150,199 -> mean of 10,20,30,40 = 25.
	// Bucket [200,300): point at 250 -> value 99.
	in := pointsAt(1, map[int64]float64{
		100: 10,
		120: 20,
		150: 30,
		199: 40,
		250: 99,
	})
	out := d.DownsampleWith(in, AggAvg)

	if len(out) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(out))
	}
	if out[0].Timestamp != 100 {
		t.Errorf("first bucket timestamp=%d want 100", out[0].Timestamp)
	}
	if out[0].Value != 25 {
		t.Errorf("first bucket value=%v want 25", out[0].Value)
	}
	if out[1].Timestamp != 200 {
		t.Errorf("second bucket timestamp=%d want 200", out[1].Timestamp)
	}
	if out[1].Value != 99 {
		t.Errorf("second bucket value=%v want 99", out[1].Value)
	}
}

// The default Downsample aggregation must be avg, matching its documented
// "emits one point with the mean value" contract.
func TestDownsampleDefaultsToAvg(t *testing.T) {
	d := New(100)
	in := pointsAt(1, map[int64]float64{10: 2, 20: 4})
	out := d.Downsample(in)

	if len(out) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(out))
	}
	if out[0].Timestamp != 0 {
		t.Errorf("bucket timestamp=%d want 0", out[0].Timestamp)
	}
	if out[0].Value != 3 {
		t.Errorf("default aggregation value=%v want 3 (avg of 2,4)", out[0].Value)
	}
}

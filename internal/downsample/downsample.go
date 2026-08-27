// Package downsample aggregates raw high-resolution points into coarser
// fixed-width buckets, reducing storage and query cost for old data.
package downsample

import (
	"github.com/dyl-01/tsdb/internal/model"
)

// Downsampler aggregates points into fixed-width buckets.
type Downsampler struct {
	interval int64
}

// New returns a Downsampler with the given bucket width in milliseconds.
func New(interval int64) *Downsampler {
	if interval <= 0 {
		interval = 1
	}
	return &Downsampler{interval: interval}
}

// Downsample aggregates points into buckets of width Interval. Each bucket
// emits one point with the mean value, timestamped at the bucket start.
func (d *Downsampler) Downsample(points model.PointList) model.PointList {
	return d.DownsampleWith(points, AggAvg)
}

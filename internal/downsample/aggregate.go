package downsample

import (
	"sort"

	"github.com/dyl-01/tsdb/internal/model"
)

// Aggregation selects how points are combined within a bucket.
type Aggregation int

const (
	// AggAvg averages bucket values.
	AggAvg Aggregation = iota
	// AggMin takes the minimum bucket value.
	AggMin
	// AggMax takes the maximum bucket value.
	AggMax
	// AggSum sums bucket values.
	AggSum
	// AggCount counts bucket points.
	AggCount
)

// DownsampleWith aggregates points into buckets using the given aggregation.
func (d *Downsampler) DownsampleWith(points model.PointList, agg Aggregation) model.PointList {
	if len(points) == 0 {
		return nil
	}
	type bucket struct {
		start int64
		sum   float64
		min   float64
		max   float64
		count int64
	}
	buckets := make(map[int64]*bucket)
	var order []int64
	for _, p := range points {
		key := p.Timestamp - p.Timestamp%d.interval
		b, ok := buckets[key]
		if !ok {
			b = &bucket{start: key, min: p.Value, max: p.Value}
			buckets[key] = b
			order = append(order, key)
		}
		b.sum += p.Value
		b.count++
		if p.Value < b.min {
			b.min = p.Value
		}
		if p.Value > b.max {
			b.max = p.Value
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	out := make(model.PointList, 0, len(order))
	for _, key := range order {
		b := buckets[key]
		value := 0.0
		switch agg {
		case AggMin:
			value = b.min
		case AggMax:
			value = b.max
		case AggSum:
			value = b.sum
		case AggCount:
			value = float64(b.count)
		default:
			value = b.sum / float64(b.count)
		}
		out = append(out, model.Point{
			SeriesID:  points[0].SeriesID,
			Timestamp: b.start,
			Value:     value,
		})
	}
	return out
}

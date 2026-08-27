package query

import (
	"sort"

	"github.com/dyl-01/tsdb/internal/model"
)

// Clip returns the points whose timestamp falls in [start, end]. The input
// need not be sorted; the output is sorted by timestamp ascending.
func Clip(points model.PointList, start, end int64) model.PointList {
	var out model.PointList
	for _, p := range points {
		if p.Timestamp >= start && p.Timestamp <= end {
			out = append(out, p)
		}
	}
	Sort(out)
	return out
}

// Sort sorts points by timestamp ascending, in place.
func Sort(points model.PointList) {
	sort.Stable(model.PointList(points))
}

// Limit truncates points to at most n elements, preserving ascending order.
func Limit(points model.PointList, n int) model.PointList {
	if n >= len(points) {
		return points
	}
	if n <= 0 {
		return nil
	}
	return points[:n]
}

// Package query merges and deduplicates scan results coming from multiple
// shards so callers see one continuous, ordered series.
package query

import (
	"sort"

	"github.com/dyl-01/tsdb/internal/model"
)

// Merge combines several point lists into one list sorted by timestamp.
func Merge(lists ...model.PointList) model.PointList {
	var out model.PointList
	for _, l := range lists {
		out = append(out, l...)
	}
	sort.Stable(model.PointList(out))
	return out
}

// Dedupe removes duplicate points that share the same (SeriesID, Timestamp),
// keeping the most recent value seen in the input. This protects against the
// brief overlap where a point exists in both a flushed block and the live
// memtable during a flush.
func Dedupe(points model.PointList) model.PointList {
	if len(points) < 2 {
		return points
	}
	seen := make(map[pointKey]bool, len(points))
	out := make(model.PointList, 0, len(points))
	// Walk from last to first so the first occurrence of a key wins.
	for i := len(points) - 1; i >= 0; i-- {
		p := points[i]
		key := pointKey{SeriesID: p.SeriesID, Timestamp: p.Timestamp}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	// Reverse back to ascending order.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

type pointKey struct {
	SeriesID  uint64
	Timestamp int64
}

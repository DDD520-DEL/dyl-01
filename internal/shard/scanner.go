package shard

import (
	"sort"

	"github.com/dyl-01/tsdb/internal/model"
)

// Scan returns all points in [start, end] across both the live memtable and
// the flushed storage blocks, merged and sorted by timestamp.
func (s *Shard) Scan(start, end int64) (model.PointList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var merged model.PointList

	// Flushed blocks.
	for _, b := range s.blocks {
		for _, p := range b.Points {
			if p.Timestamp >= start && p.Timestamp <= end {
				merged = append(merged, p)
			}
		}
	}

	// Live memtable.
	for _, p := range s.mem.Points() {
		if p.Timestamp >= start && p.Timestamp <= end {
			merged = append(merged, p)
		}
	}

	sort.Stable(model.PointList(merged))
	return merged, nil
}

// ScanSeries returns only the points of one series in [start, end].
func (s *Shard) ScanSeries(seriesID uint64, start, end int64) (model.PointList, error) {
	all, err := s.Scan(start, end)
	if err != nil {
		return nil, err
	}
	out := make(model.PointList, 0, len(all))
	for _, p := range all {
		if p.SeriesID == seriesID {
			out = append(out, p)
		}
	}
	return out, nil
}

// BlockCount returns the number of flushed storage blocks.
func (s *Shard) BlockCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.blocks)
}

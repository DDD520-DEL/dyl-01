package store

import (
	"github.com/dyl-01/tsdb/internal/model"
	"github.com/dyl-01/tsdb/internal/query"
)

// QueryByLabel returns the samples of every series carrying the label
// "name=value" in [start, end], merged across shards.
func (s *Store) QueryByLabel(name, value string, start, end int64) (model.PointList, error) {
	ids := s.idx.MatchLabel(name, value)
	if len(ids) == 0 {
		return nil, nil
	}
	want := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	var lists []model.PointList
	for _, sh := range s.overlappingShards(start, end) {
		pts, err := sh.Scan(start, end)
		if err != nil {
			return nil, err
		}
		var filtered model.PointList
		for _, p := range pts {
			if want[p.SeriesID] {
				filtered = append(filtered, p)
			}
		}
		lists = append(lists, filtered)
	}
	return query.Merge(lists...), nil
}

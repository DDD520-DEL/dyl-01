package store

import "github.com/dyl-01/tsdb/internal/model"

// ListSeries returns all registered series reconstructed from the index.
func (s *Store) ListSeries() []model.Series {
	ids := s.idx.All()
	out := make([]model.Series, 0, len(ids))
	for _, id := range ids {
		key, ok := s.idx.Key(id)
		if !ok {
			continue
		}
		out = append(out, model.ParseSeriesKey(key))
	}
	return out
}

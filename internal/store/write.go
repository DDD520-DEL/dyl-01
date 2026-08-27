package store

import (
	"github.com/dyl-01/tsdb/internal/ingest"
	"github.com/dyl-01/tsdb/internal/model"
)

// Write ingests one sample for the named series. It validates the input,
// resolves the series to an ID, and routes the point to the owning shard.
func (s *Store) Write(name string, labels map[string]string, ts int64, value float64) error {
	if err := ingest.ValidateName(name); err != nil {
		return err
	}
	if err := ingest.ValidateLabels(labels); err != nil {
		return err
	}
	if err := ingest.ValidatePoint(ts, value); err != nil {
		return err
	}
	series := model.Series{Name: name, Labels: labels}
	seriesID := s.idx.GetOrCreate(series)
	sh, err := s.shardFor(ts)
	if err != nil {
		return err
	}
	return sh.Write(model.Point{SeriesID: seriesID, Timestamp: ts, Value: value})
}

// FlushShard persists the memtable of the shard covering ts.
func (s *Store) FlushShard(ts int64) error {
	sh, err := s.shardFor(ts)
	if err != nil {
		return err
	}
	return sh.Flush()
}

// SealShard stops writes on the shard covering ts (used when a time window
// closes and the shard should become immutable).
func (s *Store) SealShard(ts int64) error {
	sh, err := s.shardFor(ts)
	if err != nil {
		return err
	}
	return sh.Seal()
}

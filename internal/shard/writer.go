package shard

import (
	"sort"

	"github.com/dyl-01/tsdb/internal/model"
	"github.com/dyl-01/tsdb/internal/storage"
	"github.com/dyl-01/tsdb/internal/wal"
)

// Write durably appends a point to the shard. It writes the WAL record first,
// then inserts into the memtable. The write is rejected when the shard is not
// open or when the point lies outside the shard range.
func (s *Shard) Write(p model.Point) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateOpen {
		return ErrNotWritable
	}
	if !s.Covers(p.Timestamp) {
		return ErrNotWritable
	}
	if err := s.log.Append(wal.Record{SeriesID: p.SeriesID, Timestamp: p.Timestamp, Value: p.Value}); err != nil {
		return err
	}
	s.mem.Insert(p)
	if s.mem.NeedsFlush() {
		return s.flushLocked()
	}
	return nil
}

// Seal marks the shard as no longer writable.
func (s *Shard) Seal() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateOpen {
		return ErrNotWritable
	}
	s.state = StateSealed
	return nil
}

// Flush persists the memtable to storage blocks and advances the state to
// flushed. It is safe to call on a sealed or open shard.
func (s *Shard) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

// flushLocked drains the memtable and writes one storage block per series.
// The caller must hold s.mu.
func (s *Shard) flushLocked() error {
	if s.mem.Len() == 0 {
		return nil
	}
	_, _, hasData := s.mem.TimeRange()
	if !hasData {
		return nil
	}
	drained := s.mem.Drain()
	bySeries := groupBySeries(drained)
	blockDir := s.dir + "/blocks"
	for seriesID, points := range bySeries {
		start := points[0].Timestamp
		end := points[len(points)-1].Timestamp
		b := storage.Block{SeriesID: seriesID, Start: start, End: end, Points: points}
		if _, err := storage.WriteBlock(blockDir, b); err != nil {
			return err
		}
		s.blocks = append(s.blocks, b)
	}
	// The flushed data is now durable in blocks. A sealed shard no longer
	// needs its WAL records, so drop them to keep the log small.
	if s.state == StateSealed {
		if err := s.log.Truncate(); err != nil {
			return err
		}
		s.state = StateFlushed
	}
	return nil
}

// groupBySeries splits points into per-series groups, keeping each group
// sorted by timestamp.
func groupBySeries(points model.PointList) map[uint64]model.PointList {
	groups := make(map[uint64]model.PointList)
	for _, p := range points {
		groups[p.SeriesID] = append(groups[p.SeriesID], p)
	}
	for id, pts := range groups {
		sort.Stable(model.PointList(pts))
		groups[id] = pts
	}
	return groups
}

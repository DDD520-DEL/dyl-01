package shard

import (
	"os"
	"sort"

	"github.com/dyl-01/tsdb/internal/model"
	"github.com/dyl-01/tsdb/internal/storage"
)

// Compact merges all flushed blocks of the shard into one block per series and
// advances the state to compacted. It flushes any live memtable first so no
// point is lost during the merge.
func (s *Shard) Compact() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mem.Len() > 0 {
		if err := s.flushLocked(); err != nil {
			return err
		}
	}
	if len(s.blocks) <= 1 {
		s.state = StateCompacted
		return nil
	}

	bySeries := make(map[uint64]model.PointList)
	for _, b := range s.blocks {
		bySeries[b.SeriesID] = append(bySeries[b.SeriesID], b.Points...)
	}

	blockDir := s.dir + "/blocks"

	// Remove every previously flushed block file before writing the merged
	// blocks. The old blocks are superseded by the merge below; leaving any
	// on disk would make a reopen load both the stale old block and the new
	// merged block, resurrecting every point as a duplicate. Remove all of
	// them (not all-but-one) and surface the error instead of swallowing it,
	// so a failed delete cannot leave a half-cleaned directory behind.
	oldPaths, err := storage.ListBlocks(blockDir)
	if err == nil {
		for _, p := range oldPaths {
			if rmErr := os.Remove(p); rmErr != nil && !os.IsNotExist(rmErr) {
				return rmErr
			}
		}
	}

	newBlocks := make([]storage.Block, 0, len(bySeries))
	for seriesID, points := range bySeries {
		sort.Stable(model.PointList(points))
		b := storage.Block{
			SeriesID: seriesID,
			Start:    points[0].Timestamp,
			End:      points[len(points)-1].Timestamp,
			Points:   points,
		}
		if _, err := storage.WriteBlock(blockDir, b); err != nil {
			return err
		}
		newBlocks = append(newBlocks, b)
	}
	s.blocks = newBlocks
	s.state = StateCompacted
	return nil
}

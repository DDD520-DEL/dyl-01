package store

import "sort"

// ShardInfo is a snapshot of one shard's identity and state for diagnostics.
type ShardInfo struct {
	ID     int64
	Start  int64
	End    int64
	State  string
	Points int
	Blocks int
}

// ShardInfos returns a snapshot of every open shard, sorted by ID.
func (s *Store) ShardInfos() []ShardInfo {
	s.mu.RLock()
	ids := make([]int64, 0, len(s.shards))
	for id := range s.shards {
		ids = append(ids, id)
	}
	s.mu.RUnlock()

	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	infos := make([]ShardInfo, 0, len(ids))
	for _, id := range ids {
		s.mu.RLock()
		sh := s.shards[id]
		s.mu.RUnlock()
		infos = append(infos, ShardInfo{
			ID:     sh.ID(),
			Start:  sh.Start(),
			End:    sh.End(),
			State:  sh.State().String(),
			Points: sh.PointCount(),
			Blocks: sh.BlockCount(),
		})
	}
	return infos
}

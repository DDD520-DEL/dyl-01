package store

// Stats is a snapshot of engine usage counters.
type Stats struct {
	Shards int
	Series int
	Points int
}

// Stats returns engine counters: open shard count, registered series count,
// and total point count across all shards (memtable plus flushed blocks).
func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	points := 0
	for _, sh := range s.shards {
		points += sh.PointCount()
	}
	return Stats{Shards: len(s.shards), Series: s.idx.Len(), Points: points}
}

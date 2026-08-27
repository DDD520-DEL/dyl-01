package shard

// PointCount returns the total number of points held by the shard, across both
// the live memtable and the flushed blocks.
func (s *Shard) PointCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := s.mem.Len()
	for _, b := range s.blocks {
		total += len(b.Points)
	}
	return total
}

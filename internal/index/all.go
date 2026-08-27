package index

import "sort"

// All returns the sorted list of every registered series ID.
func (s *SeriesIndex) All() []uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]uint64, 0, len(s.byID))
	for id := range s.byID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

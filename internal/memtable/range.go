package memtable

// TimeRange returns the minimum and maximum buffered timestamps. The second
// return value reports whether the memtable is non-empty.
func (m *Memtable) TimeRange() (min, max int64, ok bool) {
	if len(m.points) == 0 {
		return 0, 0, false
	}
	min, max = m.points[0].Timestamp, m.points[0].Timestamp
	for _, p := range m.points[1:] {
		if p.Timestamp < min {
			min = p.Timestamp
		}
		if p.Timestamp > max {
			max = p.Timestamp
		}
	}
	return min, max, true
}

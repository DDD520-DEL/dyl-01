package model

// Point is a single time-series sample. Timestamp is Unix milliseconds since
// the epoch; Value is the measured sample value.
type Point struct {
	SeriesID  uint64
	Timestamp int64
	Value     float64
}

// PointList is an ordered slice of points sorted by timestamp in ascending
// order. The ordering invariant is maintained by the shard writer.
type PointList []Point

// Len implements sort.Interface.
func (p PointList) Len() int { return len(p) }

// Less implements sort.Interface (ascending timestamp).
func (p PointList) Less(i, j int) bool { return p[i].Timestamp < p[j].Timestamp }

// Swap implements sort.Interface.
func (p PointList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

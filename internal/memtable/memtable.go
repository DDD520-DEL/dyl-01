// Package memtable holds the most recently written points in memory, ordered
// by timestamp, until they are flushed into a compressed storage block.
package memtable

import (
	"sort"

	"github.com/dyl-01/tsdb/internal/model"
)

// Memtable is an ordered in-memory buffer of points.
type Memtable struct {
	points    model.PointList
	maxPoints int
}

// New returns an empty Memtable that flushes at maxPoints points.
func New(maxPoints int) *Memtable {
	if maxPoints <= 0 {
		maxPoints = 1
	}
	return &Memtable{points: make(model.PointList, 0, maxPoints), maxPoints: maxPoints}
}

// Insert appends a point. The caller must maintain timestamp ordering by
// inserting non-decreasing timestamps per series; a final sort is applied on
// flush to tolerate small out-of-order arrivals.
func (m *Memtable) Insert(p model.Point) {
	m.points = append(m.points, p)
}

// Len returns the number of buffered points.
func (m *Memtable) Len() int { return len(m.points) }

// NeedsFlush reports whether the memtable has reached its flush threshold.
func (m *Memtable) NeedsFlush() bool { return len(m.points) >= m.maxPoints }

// Points returns the buffered points sorted by timestamp ascending.
func (m *Memtable) Points() model.PointList {
	sorted := make(model.PointList, len(m.points))
	copy(sorted, m.points)
	sort.Stable(sorted)
	return sorted
}

// Drain returns the sorted points for the caller to persist.
func (m *Memtable) Drain() model.PointList {
	sorted := m.Points()
	return sorted
}

// Scan returns all buffered points whose timestamp falls in [start, end].
func (m *Memtable) Scan(start, end int64) model.PointList {
	var out model.PointList
	for _, p := range m.points {
		if p.Timestamp >= start && p.Timestamp <= end {
			out = append(out, p)
		}
	}
	sort.Stable(out)
	return out
}

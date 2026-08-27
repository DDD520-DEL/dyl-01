// Package index provides the series-name index: a bidirectional mapping
// between a series key (name + labels) and a compact numeric ID used in
// storage and WAL records, plus a label index for label-based lookups.
package index

import (
	"sort"
	"sync"

	"github.com/dyl-01/tsdb/internal/model"
)

// SeriesIndex is a thread-safe bidirectional series registry.
type SeriesIndex struct {
	mu      sync.RWMutex
	byKey   map[string]uint64
	byID    map[uint64]string
	byLabel map[string]map[uint64]bool
	nextID  uint64
}

// New returns an empty SeriesIndex. IDs are assigned starting from 1; 0 is
// reserved to mean "unknown series".
func New() *SeriesIndex {
	return &SeriesIndex{
		byKey:   make(map[string]uint64),
		byID:    make(map[uint64]string),
		byLabel: make(map[string]map[uint64]bool),
		nextID:  1,
	}
}

// GetOrCreate returns the ID for the given series, creating a new ID when the
// series has not been seen before. It also maintains the label index.
func (s *SeriesIndex) GetOrCreate(series model.Series) uint64 {
	key := series.SeriesKey()
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.byKey[key]; ok {
		return id
	}
	id := s.nextID
	s.nextID++
	s.byKey[key] = id
	s.byID[id] = key
	for name, value := range series.Labels {
		label := name + "=" + value
		if s.byLabel[label] == nil {
			s.byLabel[label] = make(map[uint64]bool)
		}
		s.byLabel[label][id] = true
	}
	return id
}

// Resolve returns the ID for key without creating a new one. The second
// return value reports whether the key exists.
func (s *SeriesIndex) Resolve(key string) (uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byKey[key]
	return id, ok
}

// Key returns the original series key for an ID. The second return value
// reports whether the ID exists.
func (s *SeriesIndex) Key(id uint64) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, ok := s.byID[id]
	return key, ok
}

// Len returns the number of registered series.
func (s *SeriesIndex) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byKey)
}

// MatchLabel returns the sorted IDs of every series carrying the label
// "name=value".
func (s *SeriesIndex) MatchLabel(name, value string) []uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set := s.byLabel[name+"="+value]
	ids := make([]uint64, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

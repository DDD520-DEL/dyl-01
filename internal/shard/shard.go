// Package shard manages one fixed time range of the TSDB: it owns the
// write-ahead log, the in-memory buffer, and the flushed storage blocks for
// that range. A shard moves through a lifecycle state machine as data ages.
package shard

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dyl-01/tsdb/internal/config"
	"github.com/dyl-01/tsdb/internal/memtable"
	"github.com/dyl-01/tsdb/internal/model"
	"github.com/dyl-01/tsdb/internal/storage"
	"github.com/dyl-01/tsdb/internal/wal"
)

// State is the lifecycle state of a shard.
type State int

const (
	// StateOpen accepts writes (WAL + memtable).
	StateOpen State = iota
	// StateSealed no longer accepts writes but is still readable.
	StateSealed
	// StateFlushed means the memtable has been persisted to storage blocks.
	StateFlushed
	// StateCompacted means blocks have been merged into a final form.
	StateCompacted
)

// String returns the human-readable state name.
func (s State) String() string {
	switch s {
	case StateOpen:
		return "open"
	case StateSealed:
		return "sealed"
	case StateFlushed:
		return "flushed"
	case StateCompacted:
		return "compacted"
	default:
		return "unknown"
	}
}

// ErrNotWritable is returned when a write targets a non-open shard.
var ErrNotWritable = fmt.Errorf("shard: not writable")

// Shard owns the storage for one fixed time range.
type Shard struct {
	id    int64
	start int64
	end   int64

	mu    sync.RWMutex
	state State

	log *wal.WAL
	mem *memtable.Memtable

	blocks []storage.Block
	cfg    config.Config
	dir    string
}

// New opens (or creates) the shard covering [start, end). The shard directory
// lives under rootDir/shard-<id>.
func New(id, start, end int64, cfg config.Config, rootDir string) (*Shard, error) {
	dir := filepath.Join(rootDir, fmt.Sprintf("shard-%d", id))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	log, err := wal.Open(filepath.Join(dir, "wal"))
	if err != nil {
		return nil, err
	}
	s := &Shard{
		id:    id,
		start: start,
		end:   end,
		state: StateOpen,
		log:   log,
		mem:   memtable.New(cfg.MaxMemPoints),
		cfg:   cfg,
		dir:   dir,
	}
	if err := s.loadBlocks(); err != nil {
		log.Close()
		return nil, err
	}
	if err := s.recoverFromWAL(); err != nil {
		log.Close()
		return nil, err
	}
	return s, nil
}

// recoverFromWAL replays the write-ahead log into the memtable so unflushed
// writes survive a crash.
func (s *Shard) recoverFromWAL() error {
	records, err := s.log.Replay()
	if err != nil {
		return err
	}
	for _, rec := range records {
		s.mem.Insert(model.Point{SeriesID: rec.SeriesID, Timestamp: rec.Timestamp, Value: rec.Value})
	}
	return nil
}

// loadBlocks reads any previously flushed block files back into memory.
func (s *Shard) loadBlocks() error {
	blockDir := filepath.Join(s.dir, "blocks")
	paths, err := storage.ListBlocks(blockDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, p := range paths {
		b, err := storage.ReadBlock(p)
		if err != nil {
			return err
		}
		s.blocks = append(s.blocks, b)
	}
	return nil
}

// ID returns the shard number.
func (s *Shard) ID() int64 { return s.id }

// Start returns the inclusive start timestamp of the shard range.
func (s *Shard) Start() int64 { return s.start }

// End returns the exclusive end timestamp of the shard range.
func (s *Shard) End() int64 { return s.end }

// State returns the current lifecycle state.
func (s *Shard) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Covers reports whether the timestamp falls in this shard's range.
func (s *Shard) Covers(ts int64) bool {
	return ts >= s.start && ts < s.end
}

// Overlaps reports whether [start, end] intersects this shard's range.
func (s *Shard) Overlaps(start, end int64) bool {
	return start < s.end && end >= s.end
}

// Close releases the WAL and blocks. The shard must not be used afterwards.
func (s *Shard) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.log == nil {
		return nil
	}
	err := s.log.Close()
	s.log = nil
	return err
}

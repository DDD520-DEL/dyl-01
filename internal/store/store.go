// Package store is the engine core: it owns the series index, the shard set,
// and routes writes and reads through the full ingestion and query pipelines.
package store

import (
	"fmt"
	"os"
	"sync"

	"github.com/dyl-01/tsdb/internal/clock"
	"github.com/dyl-01/tsdb/internal/config"
	"github.com/dyl-01/tsdb/internal/index"
	"github.com/dyl-01/tsdb/internal/shard"
)

// Store is the top-level TSDB engine.
type Store struct {
	cfg     config.Config
	clock   clock.Clock
	idx     *index.SeriesIndex
	rootDir string

	mu     sync.RWMutex
	shards map[int64]*shard.Shard
}

// New creates (or reopens) the engine rooted at rootDir.
func New(cfg config.Config, clk clock.Clock, rootDir string) (*Store, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{
		cfg:     cfg,
		clock:   clk,
		idx:     index.New(),
		rootDir: rootDir,
		shards:  make(map[int64]*shard.Shard),
	}
	if err := s.reopenShards(); err != nil {
		return nil, err
	}
	return s, nil
}

// reopenShards loads any existing shard directories from disk.
func (s *Store) reopenShards() error {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var id int64
		if _, err := fmt.Sscanf(entry.Name(), "shard-%d", &id); err != nil {
			continue
		}
		start := id * s.cfg.ShardInterval
		end := start + s.cfg.ShardInterval
		sh, err := shard.New(id, start, end, s.cfg, s.rootDir)
		if err != nil {
			return err
		}
		s.shards[id] = sh
	}
	return nil
}

// shardFor returns the shard that covers ts, creating it on demand.
func (s *Store) shardFor(ts int64) (*shard.Shard, error) {
	id := ts / s.cfg.ShardInterval
	s.mu.RLock()
	sh, ok := s.shards[id]
	s.mu.RUnlock()
	if ok {
		return sh, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if sh, ok = s.shards[id]; ok {
		return sh, nil
	}
	start := id * s.cfg.ShardInterval
	end := start + s.cfg.ShardInterval
	sh, err := shard.New(id, start, end, s.cfg, s.rootDir)
	if err != nil {
		return nil, err
	}
	s.shards[id] = sh
	return sh, nil
}

// overlappingShards returns all shards whose range intersects [start, end].
func (s *Store) overlappingShards(start, end int64) []*shard.Shard {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*shard.Shard
	for _, sh := range s.shards {
		if sh.Overlaps(start, end) {
			out = append(out, sh)
		}
	}
	return out
}

// DataDir returns the engine root directory.
func (s *Store) DataDir() string { return s.rootDir }

// Close closes every shard.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var first error
	for _, sh := range s.shards {
		if err := sh.Close(); err != nil && first == nil {
			first = err
		}
	}
	s.shards = make(map[int64]*shard.Shard)
	return first
}

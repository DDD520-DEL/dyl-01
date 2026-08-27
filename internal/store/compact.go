package store

// CompactShard merges the blocks of the shard covering ts and marks it
// compacted. Compaction reduces the number of block files a query must scan.
func (s *Store) CompactShard(ts int64) error {
	sh, err := s.shardFor(ts)
	if err != nil {
		return err
	}
	return sh.Compact()
}

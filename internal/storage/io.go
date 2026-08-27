package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteBlock writes a block to a file under dir. The file name is derived from
// the block identity so repeated writes are idempotent. It returns the
// absolute file path.
func WriteBlock(dir string, b Block) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := encodeBlock(b)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("block-%d-%d-%d.blk", b.SeriesID, b.Start, b.End)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ReadBlock reads and decodes a block file written by WriteBlock.
func ReadBlock(path string) (Block, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Block{}, err
	}
	return decodeBlock(data)
}

// ListBlocks returns the paths of all block files under dir, sorted by name.
func ListBlocks(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.blk"))
	if err != nil {
		return nil, err
	}
	sortPaths(matches)
	return matches, nil
}

// Package wal implements a write-ahead log for the TSDB engine. Writes are
// appended to rotating segment files before they become visible in a
// memtable, so a crash can be recovered by replaying the log.
package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Record is a single durable write of one point.
type Record struct {
	SeriesID  uint64
	Timestamp int64
	Value     float64
}

// ErrCorruptSegment is returned when a WAL segment cannot be decoded.
var ErrCorruptSegment = errors.New("wal: corrupt segment")

// headerBytes is the fixed binary size of one encoded record:
// seriesID(8) + timestamp(8) + value(8) = 24 bytes.
const headerBytes = 24

// DefaultSegmentSize is the default maximum size of a WAL segment file before
// it is rotated.
const DefaultSegmentSize int64 = 1 << 20 // 1 MiB

// encodeRecord writes r into buf, which must have at least headerBytes bytes.
func encodeRecord(buf []byte, r Record) {
	binary.LittleEndian.PutUint64(buf[0:8], r.SeriesID)
	binary.LittleEndian.PutUint64(buf[8:16], uint64(r.Timestamp))
	binary.LittleEndian.PutUint64(buf[16:24], bitsOf(r.Value))
}

// decodeRecord reads one record from buf, which must have at least
// headerBytes bytes.
func decodeRecord(buf []byte) Record {
	return Record{
		SeriesID:  binary.LittleEndian.Uint64(buf[0:8]),
		Timestamp: int64(binary.LittleEndian.Uint64(buf[8:16])),
		Value:     fromBits(binary.LittleEndian.Uint64(buf[16:24])),
	}
}

// WAL is a write-ahead log backed by a directory of rotating segment files.
type WAL struct {
	dir         string
	segmentSize int64
	file        *os.File
	segment     int
	offset      int64
}

// Open opens (or creates) the WAL in dir with the default segment size.
func Open(dir string) (*WAL, error) {
	return OpenWithSize(dir, DefaultSegmentSize)
}

// OpenWithSize opens (or creates) the WAL in dir, rotating segments at
// segmentSize bytes.
func OpenWithSize(dir string, segmentSize int64) (*WAL, error) {
	if segmentSize < headerBytes {
		segmentSize = headerBytes
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	w := &WAL{dir: dir, segmentSize: segmentSize}
	segment, path := w.latestSegment()
	w.segment = segment
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	w.file = file
	w.offset = info.Size()
	return w, nil
}

// Append durably writes a record to the log, rotating the segment when the
// current one reaches the size limit. It is safe for concurrent use only
// through the shard writer, which serializes appends.
func (w *WAL) Append(r Record) error {
	if w.offset >= w.segmentSize {
		if err := w.rotate(); err != nil {
			return err
		}
	}
	buf := make([]byte, headerBytes)
	encodeRecord(buf, r)
	n, err := w.file.Write(buf)
	if err != nil {
		return err
	}
	if n != headerBytes {
		return fmt.Errorf("wal: short write %d/%d", n, headerBytes)
	}
	w.offset += int64(n)
	return w.file.Sync()
}

// Close closes the underlying segment file.
func (w *WAL) Close() error {
	if w.file == nil {
		return nil
	}
	return w.file.Close()
}

// Truncate removes all segment files and reopens the log at segment zero. It
// is called after a flush: the flushed data is durable in storage blocks, so
// the log no longer needs those records.
func (w *WAL) Truncate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	matches, _ := filepath.Glob(filepath.Join(w.dir, "segment-*.log"))
	for _, m := range matches {
		if err := os.Remove(m); err != nil {
			return err
		}
	}
	w.segment = 0
	file, err := os.OpenFile(segmentPath(w.dir, 0), os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	w.file = file
	w.offset = 0
	return nil
}

// Replay reads all records from all segment files in order.
func (w *WAL) Replay() ([]Record, error) {
	if err := w.file.Sync(); err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(w.dir, "segment-*.log"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var records []Record
	for _, p := range paths {
		recs, err := replayFile(p)
		if err != nil {
			return nil, err
		}
		records = append(records, recs...)
	}
	return records, nil
}

func replayFile(path string) ([]Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data)%headerBytes != 0 {
		return nil, fmt.Errorf("%w: %s has partial record", ErrCorruptSegment, path)
	}
	records := make([]Record, 0, len(data)/headerBytes)
	for i := 0; i < len(data); i += headerBytes {
		records = append(records, decodeRecord(data[i:i+headerBytes]))
	}
	return records, nil
}

// rotate closes the current segment and opens the next numbered segment.
func (w *WAL) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	w.segment++
	path := segmentPath(w.dir, w.segment)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	w.file = file
	w.offset = 0
	return nil
}

// latestSegment returns the highest numbered existing segment and its path.
// If no segment exists it returns segment 0.
func (w *WAL) latestSegment() (int, string) {
	matches, _ := filepath.Glob(filepath.Join(w.dir, "segment-*.log"))
	max := 0
	for _, m := range matches {
		base := filepath.Base(m)
		digits := strings.TrimSuffix(strings.TrimPrefix(base, "segment-"), ".log")
		if n, err := strconv.Atoi(digits); err == nil && n > max {
			max = n
		}
	}
	return max, segmentPath(w.dir, max)
}

// segmentPath formats a segment file path for the given segment number.
func segmentPath(dir string, segment int) string {
	return filepath.Join(dir, fmt.Sprintf("segment-%06d.log", segment))
}

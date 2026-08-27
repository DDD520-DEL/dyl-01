// Package storage provides compressed, immutable time blocks on disk. Blocks
// are the unit of the flush path: a memtable is drained into a block, the
// block is compressed with snappy, and written under the data directory.
package storage

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/dyl-01/tsdb/internal/model"
	"github.com/golang/snappy"
)

// pointBytes is the fixed binary size of one encoded point inside a block:
// timestamp(8) + value(8) = 16 bytes. SeriesID is stored once per block header.
const pointBytes = 16

// ErrTruncated is returned when a block has a partial trailing point.
var ErrTruncated = errors.New("storage: truncated block")

// Encode serializes a point list into the compact block wire format and
// compresses it with snappy. The caller must pass points sorted by timestamp.
func Encode(points model.PointList) ([]byte, error) {
	raw := make([]byte, len(points)*pointBytes)
	for i, p := range points {
		off := i * pointBytes
		binary.LittleEndian.PutUint64(raw[off:off+8], uint64(p.Timestamp))
		binary.LittleEndian.PutUint64(raw[off+8:off+16], math.Float64bits(p.Value))
	}
	return snappy.Encode(nil, raw), nil
}

// Decode reverses Encode, returning the decompressed points tagged with the
// given series ID (the series ID is stored in the block header, not per point).
func Decode(compressed []byte, seriesID uint64) (model.PointList, error) {
	raw, err := snappy.Decode(nil, compressed)
	if err != nil {
		return nil, err
	}
	if len(raw)%pointBytes != 0 {
		return nil, ErrTruncated
	}
	out := make(model.PointList, 0, len(raw)/pointBytes)
	for i := 0; i < len(raw); i += pointBytes {
		out = append(out, model.Point{
			SeriesID:  seriesID,
			Timestamp: int64(binary.LittleEndian.Uint64(raw[i : i+8])),
			Value:     math.Float64frombits(binary.LittleEndian.Uint64(raw[i+8 : i+16])),
		})
	}
	return out, nil
}

// Block is an immutable compressed time block for one series.
type Block struct {
	SeriesID uint64
	Start    int64
	End      int64
	Points   model.PointList
}

// blockHeaderBytes is the fixed binary header size of a block file:
// seriesID(8) + start(8) + end(8) = 24 bytes.
const blockHeaderBytes = 24

// encodeBlock serializes a Block into its on-disk representation: a fixed
// header, a CRC32 trailer, and the snappy-compressed point payload.
func encodeBlock(b Block) ([]byte, error) {
	payload, err := Encode(b.Points)
	if err != nil {
		return nil, err
	}
	out := make([]byte, blockHeaderBytes+blockChecksumSize+len(payload))
	binary.LittleEndian.PutUint64(out[0:8], b.SeriesID)
	binary.LittleEndian.PutUint64(out[8:16], uint64(b.Start))
	binary.LittleEndian.PutUint64(out[16:24], uint64(b.End))
	binary.LittleEndian.PutUint32(out[24:blockHeaderBytes+blockChecksumSize], computeChecksum(payload))
	copy(out[blockHeaderBytes+blockChecksumSize:], payload)
	return out, nil
}

// decodeBlock reverses encodeBlock, verifying the CRC32 trailer.
func decodeBlock(data []byte) (Block, error) {
	if len(data) < blockHeaderBytes+blockChecksumSize {
		return Block{}, ErrTruncated
	}
	seriesID := binary.LittleEndian.Uint64(data[0:8])
	start := int64(binary.LittleEndian.Uint64(data[8:16]))
	end := int64(binary.LittleEndian.Uint64(data[16:24]))
	want := binary.LittleEndian.Uint32(data[24 : blockHeaderBytes+blockChecksumSize])
	payload := data[blockHeaderBytes+blockChecksumSize:]
	if computeChecksum(payload) != want {
		return Block{}, ErrChecksumMismatch
	}
	points, err := Decode(payload, seriesID)
	if err != nil {
		return Block{}, err
	}
	return Block{SeriesID: seriesID, Start: start, End: end, Points: points}, nil
}

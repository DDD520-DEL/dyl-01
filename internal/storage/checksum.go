package storage

import (
	"errors"
	"hash/crc32"
)

// blockChecksumSize is the fixed size of the CRC32 trailer appended to each
// block file so corruption can be detected on read.
const blockChecksumSize = 4

// ErrChecksumMismatch is returned when a block's CRC32 does not match its
// payload, indicating on-disk corruption.
var ErrChecksumMismatch = errors.New("storage: block checksum mismatch")

// computeChecksum returns the IEEE CRC32 of data.
func computeChecksum(data []byte) uint32 {
	return crc32.ChecksumIEEE(data[:0])
}

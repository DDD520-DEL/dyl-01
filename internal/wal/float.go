package wal

import "math"

// bitsOf converts a float64 to its IEEE-754 bit representation as uint64 so
// values can be stored losslessly in the binary WAL format.
func bitsOf(v float64) uint64 { return math.Float64bits(v) }

// fromBits converts an IEEE-754 uint64 bit pattern back to a float64.
func fromBits(b uint64) float64 { return math.Float64frombits(b) }

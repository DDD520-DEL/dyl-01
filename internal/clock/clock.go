// Package clock provides a small time abstraction so the TSDB engine can be
// driven by a real wall clock in production and a deterministic mock in tests.
package clock

import "time"

// Clock is the minimal time source used by the engine.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// NowUnixMilli returns the current time as Unix milliseconds.
	NowUnixMilli() int64
}

// Real is the production clock backed by time.Now.
type Real struct{}

// NewReal returns a Real clock.
func NewReal() Real { return Real{} }

// Now returns time.Now.
func (Real) Now() time.Time { return time.Now() }

// NowUnixMilli returns the current Unix milliseconds.
func (r Real) NowUnixMilli() int64 { return r.Now().UnixMilli() }

// Mock is a deterministic clock for tests and replay scenarios.
type Mock struct {
	// T is the fixed time returned by the mock.
	T time.Time
}

// NewMock returns a Mock clock pinned to the given time.
func NewMock(t time.Time) *Mock { return &Mock{T: t} }

// Now returns the pinned time.
func (m *Mock) Now() time.Time { return m.T }

// NowUnixMilli returns the pinned time as Unix milliseconds.
func (m *Mock) NowUnixMilli() int64 { return m.Now().UnixMilli() }

// Advance moves the pinned time forward by the given duration.
func (m *Mock) Advance(d time.Duration) { m.T = m.T.Add(d) }

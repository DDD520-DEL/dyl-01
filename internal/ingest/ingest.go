// Package ingest validates incoming write requests before they enter the
// engine. It rejects malformed series names and non-finite values early so
// downstream storage never sees invalid data.
package ingest

import (
	"errors"
	"math"
	"strings"
)

// ErrEmptyName is returned for a blank or whitespace-only series name.
var ErrEmptyName = errors.New("ingest: empty series name")

// ErrInvalidValue is returned for NaN or infinite sample values.
var ErrInvalidValue = errors.New("ingest: value is NaN or infinite")

// ErrInvalidTimestamp is returned for non-positive timestamps.
var ErrInvalidTimestamp = errors.New("ingest: timestamp must be positive")

// ErrInvalidLabels is returned when a label set contains an empty label name.
var ErrInvalidLabels = errors.New("ingest: empty label name")

// ValidateName reports whether a series name is acceptable.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrEmptyName
	}
	return nil
}

// ValidateLabels reports whether a label set is acceptable.
func ValidateLabels(labels map[string]string) error {
	for name := range labels {
		if strings.TrimSpace(name) == "" {
			return ErrInvalidLabels
		}
	}
	return nil
}

// ValidatePoint reports whether a (timestamp, value) sample is acceptable.
func ValidatePoint(ts int64, value float64) error {
	if ts <= 0 {
		return ErrInvalidTimestamp
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return ErrInvalidValue
	}
	return nil
}

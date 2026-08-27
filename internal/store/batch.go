package store

import "github.com/dyl-01/tsdb/internal/ingest"

// BatchPoint is one sample in a batch write.
type BatchPoint struct {
	Name      string
	Labels    map[string]string
	Timestamp int64
	Value     float64
}

// WriteBatch ingests multiple samples. Every sample is validated before any
// sample is written, so a batch with one bad point writes nothing.
func (s *Store) WriteBatch(points []BatchPoint) error {
	for _, p := range points {
		if err := ingest.ValidateName(p.Name); err != nil {
			return err
		}
		if err := ingest.ValidatePoint(p.Timestamp, p.Value); err != nil {
			return err
		}
	}
	for _, p := range points {
		if err := s.Write(p.Name, p.Labels, p.Timestamp, p.Value); err != nil {
			return err
		}
	}
	return nil
}

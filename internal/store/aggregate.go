package store

import (
	"errors"

	"github.com/dyl-01/tsdb/internal/downsample"
	"github.com/dyl-01/tsdb/internal/model"
)

// ErrUnknownAggregation is returned for an unsupported aggregation name.
var ErrUnknownAggregation = errors.New("store: unknown aggregation")

// AggregationByName maps an aggregation name to its enum value.
func AggregationByName(name string) (downsample.Aggregation, error) {
	switch name {
	case "avg":
		return downsample.AggAvg, nil
	case "min":
		return downsample.AggMin, nil
	case "max":
		return downsample.AggMax, nil
	case "sum":
		return downsample.AggSum, nil
	case "count":
		return downsample.AggCount, nil
	default:
		return downsample.AggAvg, ErrUnknownAggregation
	}
}

// DownsampleWithAgg aggregates the raw points of the shard covering ts using
// the named aggregation ("avg", "min", "max", "sum", "count").
func (s *Store) DownsampleWithAgg(ts int64, agg string) (model.PointList, error) {
	a, err := AggregationByName(agg)
	if err != nil {
		return nil, err
	}
	sh, err := s.shardFor(ts)
	if err != nil {
		return nil, err
	}
	raw, err := sh.Scan(sh.Start(), sh.End())
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	d := downsample.New(s.cfg.DownsampleInterval)
	return d.DownsampleWith(raw, a), nil
}

package store

import (
	"sort"

	"github.com/dyl-01/tsdb/internal/downsample"
	"github.com/dyl-01/tsdb/internal/model"
	"github.com/dyl-01/tsdb/internal/query"
	"github.com/dyl-01/tsdb/internal/retention"
)

// Query returns the samples of the named series in [start, end], merged across
// all overlapping shards and deduplicated.
func (s *Store) Query(name string, labels map[string]string, start, end int64) (model.PointList, error) {
	series := model.Series{Name: name, Labels: labels}
	seriesID, ok := s.idx.Resolve(series.SeriesKey())
	if !ok {
		return nil, nil
	}
	var lists []model.PointList
	for _, sh := range s.overlappingShards(start, end) {
		pts, err := sh.ScanSeries(seriesID, start, end)
		if err != nil {
			return nil, err
		}
		lists = append(lists, pts)
	}
	merged := query.Merge(lists...)
	clipped := query.Clip(merged, start, end)
	return query.Dedupe(clipped), nil
}

// QueryAll returns all series' samples in [start, end], merged and sorted.
func (s *Store) QueryAll(start, end int64) (model.PointList, error) {
	var lists []model.PointList
	for _, sh := range s.overlappingShards(start, end) {
		pts, err := sh.Scan(start, end)
		if err != nil {
			return nil, err
		}
		lists = append(lists, pts)
	}
	return query.Merge(lists...), nil
}

// DownsampleSeries aggregates only one series across the shard covering ts.
func (s *Store) DownsampleSeries(name string, labels map[string]string, ts int64) (model.PointList, error) {
	series := model.Series{Name: name, Labels: labels}
	seriesID, ok := s.idx.Resolve(series.SeriesKey())
	if !ok {
		return nil, nil
	}
	sh, err := s.shardFor(ts)
	if err != nil {
		return nil, err
	}
	raw, err := sh.ScanSeries(seriesID, sh.Start(), sh.End())
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	d := downsample.New(s.cfg.DownsampleInterval)
	return d.Downsample(raw), nil
}

// EnforceRetention closes and removes shards whose range has aged past the
// retention window. It returns the number of removed shards.
func (s *Store) EnforceRetention() (int, error) {
	now := s.clock.NowUnixMilli()
	policy := retention.NewPolicy(s.cfg.Retention)

	s.mu.Lock()
	expired := make([]int64, 0)
	for id, sh := range s.shards {
		if policy.Expired(sh.End(), now) {
			expired = append(expired, id)
		}
	}
	s.mu.Unlock()

	sort.Slice(expired, func(i, j int) bool { return expired[i] < expired[j] })
	removed := 0
	for _, id := range expired {
		s.mu.Lock()
		sh, ok := s.shards[id]
		if ok {
			delete(s.shards, id)
		}
		s.mu.Unlock()
		if !ok {
			continue
		}
		_ = sh.Close()
		removed++
	}
	return removed, nil
}

package healthz

// Trend summarizes a window of recent snapshots so operators can spot slow
// growth or sustained degradation without paging through every snapshot.
type Trend struct {
	Samples         int   `json:"samples"`
	MinPoints       int   `json:"min_points"`
	MaxPoints       int   `json:"max_points"`
	AvgPoints       int   `json:"avg_points"`
	MinSeries       int   `json:"min_series"`
	MaxSeries       int   `json:"max_series"`
	AvgSeries       int   `json:"avg_series"`
	DegradedSamples int   `json:"degraded_samples"`
}

// Trend returns the trend summary over up to n recent snapshots.
func (p *Prober) Trend(n int) Trend {
	p.mu.Lock()
	reports := make([]Report, 0, n)
	for i := len(p.history) - 1; i >= 0 && len(reports) < n; i-- {
		reports = append(reports, p.history[i])
	}
	p.mu.Unlock()
	out := Trend{Samples: len(reports)}
	if len(reports) == 0 {
		return out
	}
	out.MinPoints = reports[0].Points
	out.MaxPoints = reports[0].Points
	out.MinSeries = reports[0].Series
	out.MaxSeries = reports[0].Series
	pointsTotal := 0
	seriesTotal := 0
	for _, report := range reports {
		if report.Points < out.MinPoints {
			out.MinPoints = report.Points
		}
		if report.Points > out.MaxPoints {
			out.MaxPoints = report.Points
		}
		if report.Series < out.MinSeries {
			out.MinSeries = report.Series
		}
		if report.Series > out.MaxSeries {
			out.MaxSeries = report.Series
		}
		pointsTotal += report.Points
		seriesTotal += report.Series
		if report.Status == "degraded" {
			out.DegradedSamples++
		}
	}
	out.AvgPoints = pointsTotal / len(reports)
	out.AvgSeries = seriesTotal / len(reports)
	return out
}

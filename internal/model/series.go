// Package model defines the core data types shared across the TSDB engine:
// series identity and the time-series data point.
package model

// Series identifies a unique time series by its name and the label set.
// Two series are equal only when both the name and every label match.
type Series struct {
	Name   string
	Labels map[string]string
}

// SeriesKey returns a canonical string key for the series. It is used by the
// index to deduplicate series and resolve names to numeric IDs.
func (s Series) SeriesKey() string {
	if s.Name == "" {
		return ""
	}
	key := s.Name
	if len(s.Labels) > 0 {
		key += "\x00"
		// Labels are ordered by key to make the key deterministic regardless
		// of map iteration order.
		keys := make([]string, 0, len(s.Labels))
		for k := range s.Labels {
			keys = append(keys, k)
		}
		sortStrings(keys)
		for i, k := range keys {
			if i > 0 {
				key += ","
			}
			key += k + "=" + s.Labels[k]
		}
	}
	return key
}

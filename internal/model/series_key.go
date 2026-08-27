package model

import "strings"

// ParseSeriesKey reverses SeriesKey, reconstructing the series from its
// canonical string form. Labels that fail to parse are skipped.
func ParseSeriesKey(key string) Series {
	if key == "" {
		return Series{}
	}
	idx := strings.IndexByte(key, 0)
	if idx < 0 {
		return Series{Name: key}
	}
	s := Series{Name: key[:idx]}
	rest := key[idx+1:]
	if rest == "" {
		return s
	}
	labels := make(map[string]string)
	for _, pair := range strings.Split(rest, ",") {
		eq := strings.IndexByte(pair, '=')
		if eq <= 0 {
			continue
		}
		labels[pair[:eq]] = pair[eq+1:]
	}
	s.Labels = labels
	return s
}

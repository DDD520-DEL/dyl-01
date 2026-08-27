package model

import "sort"

// sortStrings sorts a slice of strings in place. It is extracted so the series
// key builder stays deterministic and testable.
func sortStrings(values []string) {
	sort.Strings(values)
}

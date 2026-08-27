package storage

import "sort"

// sortPaths sorts a slice of file paths in place.
func sortPaths(paths []string) {
	sort.Strings(paths)
}

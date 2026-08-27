package index

import (
	"testing"

	"github.com/dyl-01/tsdb/internal/model"
)

func TestGetOrCreateMaintainsLabelIndex(t *testing.T) {
	idx := New()
	idx.GetOrCreate(model.Series{
		Name:   "cpu",
		Labels: map[string]string{"host": "server1", "region": "us"},
	})
	idx.GetOrCreate(model.Series{
		Name:   "cpu",
		Labels: map[string]string{"host": "server2", "region": "us"},
	})

	cases := []struct {
		name, value string
		want        int
	}{
		{"host", "server1", 1},
		{"host", "server2", 1},
		{"region", "us", 2},
		{"host", "server3", 0},
		{"missing", "x", 0},
	}
	for _, c := range cases {
		ids := idx.MatchLabel(c.name, c.value)
		if len(ids) != c.want {
			t.Errorf("MatchLabel(%q,%q) = %v ids, want %d", c.name, c.value, ids, c.want)
		}
	}
}

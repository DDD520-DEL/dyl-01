package verifycase

import (
	"testing"

	"github.com/dyl-01/tsdb/internal/index"
	"github.com/dyl-01/tsdb/internal/model"
)

// TestLabelIndexMatchesSeries verifies that the label index resolves every
// series carrying a given label, and stays in sync with the series registry.
func TestLabelIndexMatchesSeries(t *testing.T) {
	idx := index.New()

	idx.GetOrCreate(model.Series{Name: "cpu.usage", Labels: map[string]string{"host": "a"}})
	idx.GetOrCreate(model.Series{Name: "cpu.usage", Labels: map[string]string{"host": "b"}})
	idx.GetOrCreate(model.Series{Name: "mem.usage", Labels: map[string]string{"host": "a"}})

	hostA := idx.MatchLabel("host", "a")
	if len(hostA) != 2 {
		t.Fatalf("label host=a matched %d series, want 2", len(hostA))
	}
	hostB := idx.MatchLabel("host", "b")
	if len(hostB) != 1 {
		t.Fatalf("label host=b matched %d series, want 1", len(hostB))
	}

	// A repeated GetOrCreate must not create a duplicate ID.
	cpuA := model.Series{Name: "cpu.usage", Labels: map[string]string{"host": "a"}}
	id1 := idx.GetOrCreate(cpuA)
	id2 := idx.GetOrCreate(cpuA)
	if id1 != id2 {
		t.Fatalf("repeated GetOrCreate returned %d then %d", id1, id2)
	}
	if idx.Len() != 3 {
		t.Fatalf("series count = %d, want 3", idx.Len())
	}
}

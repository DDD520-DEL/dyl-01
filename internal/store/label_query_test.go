package store

import (
	"testing"
	"time"

	"github.com/dyl-01/tsdb/internal/clock"
	"github.com/dyl-01/tsdb/internal/config"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	cfg := config.Default()
	cfg.WALDir = t.TempDir()
	cfg.DataDir = t.TempDir()
	// Downsample is irrelevant here and adds config constraints; disable it.
	cfg.DownsampleEnabled = false
	s, err := New(cfg, clock.NewMock(time.UnixMilli(0)), t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestQueryByLabelFindsWrittenSeries(t *testing.T) {
	s := newTestStore(t)

	if err := s.Write("cpu", map[string]string{"host": "server1", "region": "us"}, 1_000, 1.0); err != nil {
		t.Fatalf("Write server1: %v", err)
	}
	if err := s.Write("cpu", map[string]string{"host": "server2", "region": "us"}, 2_000, 2.0); err != nil {
		t.Fatalf("Write server2: %v", err)
	}
	if err := s.Write("mem", map[string]string{"host": "server1"}, 3_000, 3.0); err != nil {
		t.Fatalf("Write mem: %v", err)
	}

	cases := []struct {
		name, value string
		wantCount  int
	}{
		{"host", "server1", 2},
		{"host", "server2", 1},
		{"region", "us", 2},
		{"host", "server3", 0},
	}
	for _, c := range cases {
		pts, err := s.QueryByLabel(c.name, c.value, 0, 10_000)
		if err != nil {
			t.Fatalf("QueryByLabel(%q,%q): %v", c.name, c.value, err)
		}
		if len(pts) != c.wantCount {
			t.Errorf("QueryByLabel(%q,%q) = %d points, want %d", c.name, c.value, len(pts), c.wantCount)
		}
	}
}

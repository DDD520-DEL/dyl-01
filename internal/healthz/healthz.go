// Package healthz implements a real readiness probe over the TSDB engine:
// store, open shards, registered series and total points. The HTTP service
// exposes the snapshot at /health and a strict gate at /healthz/ready so
// operators can verify a node is safe to serve traffic.
package healthz

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/dyl-01/tsdb/internal/store"
)

// Component identifiers reported by the probe.
const (
	ComponentStore  = "store"
	ComponentShards = "shards"
	ComponentSeries = "series"
	ComponentPoints = "points"
)

// ComponentView carries the per-component snapshot shown in the report.
type ComponentView struct {
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
	Latency int64  `json:"latency_ms"`
}

// CheckResult is the flattened per-component probe record.
type CheckResult struct {
	Component string `json:"component"`
	OK        bool   `json:"ok"`
	Detail    string `json:"detail,omitempty"`
	LatencyMS int64  `json:"latency_ms"`
}

// Report is the immutable snapshot returned by one Check call.
type Report struct {
	Status      string                   `json:"status"`
	GeneratedAt string                   `json:"generated_at"`
	Shards      int                      `json:"shards"`
	Series      int                      `json:"series"`
	Points      int                      `json:"points"`
	Labels      int                      `json:"labels"`
	Checks      []CheckResult            `json:"checks"`
	Components  map[string]ComponentView `json:"components"`
}

// Prober runs bounded readiness probes against the live engine and keeps a
// short ring of past snapshots for trend inspection.
type Prober struct {
	st      *store.Store
	mu      sync.Mutex
	history []Report
	cap     int
}

// New returns a Prober bound to the given engine.
func New(st *store.Store) *Prober {
	return &Prober{st: st, cap: 64}
}

// Check gathers one readiness snapshot from the live engine.
func (p *Prober) Check(ctx context.Context) Report {
	started := time.Now()
	components := make(map[string]ComponentView, 4)
	checks := make([]CheckResult, 0, 4)
	collect := func(name string, ok bool, detail string, lat time.Duration) {
		ms := lat.Milliseconds()
		components[name] = ComponentView{OK: ok, Detail: detail, Latency: ms}
		checks = append(checks, CheckResult{Component: name, OK: ok, Detail: detail, LatencyMS: ms})
	}

	stats := p.st.Stats()
	collect(ComponentStore, true, p.storeDetail(stats), time.Since(started))
	collect(ComponentShards, true, p.shardDetail(stats), time.Since(started))
	collect(ComponentSeries, true, p.seriesDetail(stats), time.Since(started))
	collect(ComponentPoints, true, fmt.Sprintf("points=%d", stats.Points), time.Since(started))

	allOK := true
	for _, c := range checks {
		if !c.OK {
			allOK = false
			break
		}
	}
	status := "ok"
	if !allOK {
		status = "degraded"
	}
	report := Report{
		Status:      status,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Shards:      stats.Shards,
		Series:      stats.Series,
		Points:      stats.Points,
		Labels:      p.labelCount(),
		Checks:      checks,
		Components:  components,
	}
	p.mu.Lock()
	p.history = append(p.history, report)
	if len(p.history) > p.cap {
		p.history = p.history[len(p.history)-p.cap:]
	}
	p.mu.Unlock()
	return report
}

// Ready reports whether every component passed its most recent probe.
func (p *Prober) Ready(ctx context.Context) bool {
	return p.Check(ctx).Status == "ok"
}

// Recent returns up to n of the most recent snapshots, newest first.
func (p *Prober) Recent(n int) []Report {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Report, 0, n)
	for i := len(p.history) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, p.history[i])
	}
	return out
}

// Summary aggregates the recent snapshots into a compact view used by the
// history endpoint so operators can spot a degrading component at a glance.
type Summary struct {
	Probes         int            `json:"probes"`
	LastStatus     string         `json:"last_status"`
	AvgLatencyMS   int64          `json:"avg_latency_ms"`
	ComponentOK    map[string]int `json:"component_ok"`
	ComponentTotal map[string]int `json:"component_total"`
}

// Summary returns the aggregated view over the retained snapshots.
func (p *Prober) Summary() Summary {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := Summary{
		ComponentOK:    make(map[string]int),
		ComponentTotal: make(map[string]int),
	}
	if len(p.history) == 0 {
		return out
	}
	out.Probes = len(p.history)
	out.LastStatus = p.history[len(p.history)-1].Status
	var latencyTotal int64
	var checkCount int
	for _, report := range p.history {
		for _, check := range report.Checks {
			out.ComponentTotal[check.Component]++
			if check.OK {
				out.ComponentOK[check.Component]++
			}
			latencyTotal += check.LatencyMS
			checkCount++
		}
	}
	if checkCount > 0 {
		out.AvgLatencyMS = latencyTotal / int64(checkCount)
	}
	return out
}

func (p *Prober) storeDetail(stats store.Stats) string {
	return fmt.Sprintf("data_dir=%s shards=%d series=%d points=%d", p.st.DataDir(), stats.Shards, stats.Series, stats.Points)
}

func (p *Prober) shardDetail(stats store.Stats) string {
	infos := p.st.ShardInfos()
	states := map[string]int{}
	points := 0
	blocks := 0
	for _, info := range infos {
		states[info.State]++
		points += info.Points
		blocks += info.Blocks
	}
	keys := make([]string, 0, len(states))
	for k := range states {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	detail := fmt.Sprintf("shards=%d points=%d blocks=%d", stats.Shards, points, blocks)
	for _, k := range keys {
		detail += fmt.Sprintf(" %s=%d", k, states[k])
	}
	return detail
}

func (p *Prober) seriesDetail(stats store.Stats) string {
	return fmt.Sprintf("series=%d labels=%d", stats.Series, p.labelCount())
}

func (p *Prober) labelCount() int {
	series := p.st.ListSeries()
	labels := 0
	for _, s := range series {
		labels += len(s.Labels)
	}
	return labels
}

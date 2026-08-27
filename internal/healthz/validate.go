package healthz

import (
	"fmt"
	"strings"
	"time"
)

// Validate checks a snapshot for internal consistency and returns a list of
// problems operators should treat as alarms. An empty result means the
// snapshot is structurally sound.
func Validate(report Report) []string {
	var problems []string
	if report.Status != "ok" && report.Status != "degraded" {
		problems = append(problems, fmt.Sprintf("unexpected status %q", report.Status))
	}
	if _, err := time.Parse(time.RFC3339, report.GeneratedAt); err != nil {
		problems = append(problems, fmt.Sprintf("generated_at is not RFC3339: %q", report.GeneratedAt))
	}
	if report.Shards < 0 {
		problems = append(problems, "negative shard count")
	}
	if report.Series < 0 {
		problems = append(problems, "negative series count")
	}
	if report.Points < 0 {
		problems = append(problems, "negative point count")
	}
	if len(report.Checks) == 0 {
		problems = append(problems, "snapshot has no component checks")
	}
	seen := map[string]bool{}
	for _, check := range report.Checks {
		if seen[check.Component] {
			problems = append(problems, fmt.Sprintf("duplicate component %q", check.Component))
		}
		seen[check.Component] = true
		if check.LatencyMS < 0 {
			problems = append(problems, fmt.Sprintf("component %q has negative latency", check.Component))
		}
	}
	okCount := 0
	for _, check := range report.Checks {
		if check.OK {
			okCount++
		}
	}
	if report.Status == "ok" && okCount != len(report.Checks) {
		problems = append(problems, "status ok but not every component passed")
	}
	if report.Status == "degraded" && okCount == len(report.Checks) && len(report.Checks) > 0 {
		problems = append(problems, "status degraded but every component passed")
	}
	return problems
}

// Describe renders a compact one-line summary of a snapshot for log lines
// and alert hooks.
func Describe(report Report) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("status=%s shards=%d series=%d points=%d", report.Status, report.Shards, report.Series, report.Points))
	for _, check := range report.Checks {
		state := "ok"
		if !check.OK {
			state = "degraded"
		}
		builder.WriteString(fmt.Sprintf(" %s=%s(%dms)", check.Component, state, check.LatencyMS))
	}
	return builder.String()
}

// AlarmSeverity maps a validation result to a severity label so operators
// can route alerts without parsing problem strings.
func AlarmSeverity(problems []string) string {
	switch len(problems) {
	case 0:
		return "none"
	case 1, 2:
		return "warning"
	default:
		return "critical"
	}
}

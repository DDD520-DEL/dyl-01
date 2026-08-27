// Package retention decides when shards have aged past the configured
// retention window and should be removed.
package retention

// Policy is the data retention policy.
type Policy struct {
	// Retention is how long data is kept, in milliseconds.
	Retention int64
}

// NewPolicy returns a Policy with the given retention window in milliseconds.
func NewPolicy(retention int64) Policy {
	return Policy{Retention: retention}
}

// Expired reports whether a shard time range ending at shardEnd is fully older
// than the retention window relative to now.
func (p Policy) Expired(shardEnd, now int64) bool {
	if p.Retention <= 0 {
		return false
	}
	return shardEnd >= now-p.Retention
}

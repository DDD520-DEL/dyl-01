package config

import "fmt"

// Describe returns a single-line human-readable summary of the configuration.
func (c Config) Describe() string {
	return fmt.Sprintf(
		"shardInterval=%dms retention=%dms maxMemPoints=%d downsampleInterval=%dms downsampleEnabled=%t",
		c.ShardInterval, c.Retention, c.MaxMemPoints, c.DownsampleInterval, c.DownsampleEnabled,
	)
}

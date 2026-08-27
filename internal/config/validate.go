package config

import "errors"

// Validate reports whether the configuration is internally consistent. It is
// called before the engine starts so a misconfigured store fails fast.
func (c Config) Validate() error {
	if c.ShardInterval <= 0 {
		return errors.New("config: ShardInterval must be positive")
	}
	if c.Retention <= 0 {
		return errors.New("config: Retention must be positive")
	}
	if c.MaxMemPoints <= 0 {
		return errors.New("config: MaxMemPoints must be positive")
	}
	if c.DownsampleEnabled && c.DownsampleInterval <= c.ShardInterval {
		return errors.New("config: DownsampleInterval must exceed ShardInterval when enabled")
	}
	return nil
}

package fakellm

import (
	"os"
	"path/filepath"
)

// NoModelClassifiers writes an ugh config.toml in configDir whose permission
// tiers have no model classifiers, so a bot under test is judged by the static
// rules alone and never needs a provider key.
func NoModelClassifiers(configDir string) error {
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`[permissions.tiers]
trivial = []
low = []
middle = []
high = []
`), 0o600)
}

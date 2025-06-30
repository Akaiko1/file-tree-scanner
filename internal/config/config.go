package config

// Config defines configuration parameters for directory scanning behavior and UI settings.
type Config struct {
	MaxDepth      int
	ShowHidden    bool
	SortDirs      bool
	ShowSize      bool
	ConcurrentOps int
}

// DefaultConfig returns a configuration with sensible defaults: max depth 15, hidden files disabled, directory sorting enabled.
func DefaultConfig() *Config {
	return &Config{
		MaxDepth:      15, // Reasonable depth limit to prevent hangs
		ShowHidden:    false,
		SortDirs:      true,
		ShowSize:      false,
		ConcurrentOps: 5, // Reduced for stability
	}
}
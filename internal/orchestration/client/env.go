package client

// BuildEnvVars creates common environment variables for agent processes.
// Returns a slice of environment variables in "KEY=VALUE" format.
// These are added to the process environment via SpawnBuilder.WithEnv().
func BuildEnvVars(cfg Config) []string {
	var env []string
	if cfg.BeadsDir != "" {
		env = append(env, "BEADS_DIR="+cfg.BeadsDir)
	}
	return env
}

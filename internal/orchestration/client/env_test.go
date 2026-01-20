package client

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildEnvVars_WithBeadsDir(t *testing.T) {
	cfg := Config{
		BeadsDir: "/path/to/project",
	}

	env := BuildEnvVars(cfg)

	require.Equal(t, []string{"BEADS_DIR=/path/to/project"}, env)
}

func TestBuildEnvVars_EmptyBeadsDir(t *testing.T) {
	cfg := Config{
		BeadsDir: "",
	}

	env := BuildEnvVars(cfg)

	require.Empty(t, env)
}

func TestBuildEnvVars_WithWorkDirOnly(t *testing.T) {
	cfg := Config{
		WorkDir: "/some/work/dir",
	}

	env := BuildEnvVars(cfg)

	require.Empty(t, env, "WorkDir should not affect BuildEnvVars")
}

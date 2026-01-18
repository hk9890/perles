package client

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Test-only options for ExecutableFinder.
// These are not exported and live in the test file to avoid polluting the public API.

// withStatFunc sets a custom stat function for testing.
func withStatFunc(fn func(string) (os.FileInfo, error)) FinderOption {
	return func(f *ExecutableFinder) {
		f.statFn = fn
	}
}

// withLookPathFunc sets a custom LookPath function for testing.
func withLookPathFunc(fn func(string) (string, error)) FinderOption {
	return func(f *ExecutableFinder) {
		f.lookPathFn = fn
	}
}

// withUserHomeFunc sets a custom UserHomeDir function for testing.
func withUserHomeFunc(fn func() (string, error)) FinderOption {
	return func(f *ExecutableFinder) {
		f.userHomeFn = fn
	}
}

// withGOOS sets a custom GOOS value for testing cross-platform behavior.
func withGOOS(goos string) FinderOption {
	return func(f *ExecutableFinder) {
		f.goos = goos
	}
}

// mockFileInfo implements os.FileInfo for testing.
type mockFileInfo struct {
	name  string
	isDir bool
	mode  os.FileMode
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return nil }

func TestNewExecutableFinder(t *testing.T) {
	t.Run("creates finder with defaults", func(t *testing.T) {
		f := NewExecutableFinder("claude")
		require.Equal(t, "claude", f.execName)
		require.Empty(t, f.knownPaths)
		require.Empty(t, f.envOverride)
		require.NotNil(t, f.statFn)
		require.NotNil(t, f.lookPathFn)
		require.NotNil(t, f.userHomeFn)
	})

	t.Run("applies WithKnownPaths option", func(t *testing.T) {
		paths := []string{"~/.claude/{name}", "/usr/local/bin/{name}"}
		f := NewExecutableFinder("claude", WithKnownPaths(paths...))
		require.Equal(t, paths, f.knownPaths)
	})

	t.Run("applies WithEnvOverride option", func(t *testing.T) {
		f := NewExecutableFinder("claude", WithEnvOverride("CLAUDE_PATH"))
		require.Equal(t, "CLAUDE_PATH", f.envOverride)
	})

	t.Run("applies multiple options", func(t *testing.T) {
		f := NewExecutableFinder("gemini",
			WithKnownPaths("~/.npm/bin/{name}"),
			WithEnvOverride("GEMINI_PATH"),
		)
		require.Equal(t, "gemini", f.execName)
		require.Equal(t, []string{"~/.npm/bin/{name}"}, f.knownPaths)
		require.Equal(t, "GEMINI_PATH", f.envOverride)
	})
}

func TestExecutableFinder_expandPath(t *testing.T) {
	t.Run("expands {name} on Unix", func(t *testing.T) {
		f := NewExecutableFinder("claude",
			withGOOS("darwin"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
		)
		path, err := f.expandPath("~/.local/bin/{name}")
		require.NoError(t, err)
		require.Equal(t, filepath.Clean("/home/test/.local/bin/claude"), path)
	})

	t.Run("expands {name} with .exe on Windows", func(t *testing.T) {
		f := NewExecutableFinder("claude",
			withGOOS("windows"),
			withUserHomeFunc(func() (string, error) { return "C:\\Users\\test", nil }),
		)
		path, err := f.expandPath("~\\.claude\\{name}")
		require.NoError(t, err)
		require.Equal(t, filepath.Clean("C:\\Users\\test\\.claude\\claude.exe"), path)
	})

	t.Run("expands tilde to home directory", func(t *testing.T) {
		f := NewExecutableFinder("myapp",
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/alice", nil }),
		)
		path, err := f.expandPath("~/.config/myapp/{name}")
		require.NoError(t, err)
		require.Equal(t, filepath.Clean("/home/alice/.config/myapp/myapp"), path)
	})

	t.Run("returns error when home directory lookup fails", func(t *testing.T) {
		f := NewExecutableFinder("myapp",
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "", errors.New("no home") }),
		)
		_, err := f.expandPath("~/.local/bin/{name}")
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot expand ~")
	})

	t.Run("expands $HOME on Unix", func(t *testing.T) {
		t.Setenv("HOME", "/custom/home")
		f := NewExecutableFinder("myapp",
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
		)
		path, err := f.expandPath("$HOME/bin/{name}")
		require.NoError(t, err)
		require.Equal(t, filepath.Clean("/custom/home/bin/myapp"), path)
	})

	t.Run("expands ${VAR} syntax", func(t *testing.T) {
		t.Setenv("MY_PATH", "/opt/tools")
		f := NewExecutableFinder("myapp",
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
		)
		path, err := f.expandPath("${MY_PATH}/bin/{name}")
		require.NoError(t, err)
		require.Equal(t, filepath.Clean("/opt/tools/bin/myapp"), path)
	})

	t.Run("expands %USERPROFILE% on Windows", func(t *testing.T) {
		t.Setenv("USERPROFILE", "C:\\Users\\bob")
		f := NewExecutableFinder("myapp",
			withGOOS("windows"),
			withUserHomeFunc(func() (string, error) { return "C:\\Users\\test", nil }),
		)
		path, err := f.expandPath("%USERPROFILE%\\bin\\{name}")
		require.NoError(t, err)
		require.Equal(t, filepath.Clean("C:\\Users\\bob\\bin\\myapp.exe"), path)
	})

	t.Run("expands %LOCALAPPDATA% on Windows", func(t *testing.T) {
		t.Setenv("LOCALAPPDATA", "C:\\Users\\bob\\AppData\\Local")
		f := NewExecutableFinder("myapp",
			withGOOS("windows"),
			withUserHomeFunc(func() (string, error) { return "C:\\Users\\test", nil }),
		)
		path, err := f.expandPath("%LOCALAPPDATA%\\Programs\\{name}")
		require.NoError(t, err)
		require.Equal(t, filepath.Clean("C:\\Users\\bob\\AppData\\Local\\Programs\\myapp.exe"), path)
	})

	t.Run("keeps %VAR% if not set on Windows", func(t *testing.T) {
		// Ensure the variable is not set
		t.Setenv("NOTSET_VAR", "")
		os.Unsetenv("NOTSET_VAR")

		f := NewExecutableFinder("myapp",
			withGOOS("windows"),
			withUserHomeFunc(func() (string, error) { return "C:\\Users\\test", nil }),
		)
		path, err := f.expandPath("C:\\%NOTSET_VAR%\\{name}")
		require.NoError(t, err)
		// The %NOTSET_VAR% remains as-is since the var is not set
		require.Contains(t, path, "%NOTSET_VAR%")
	})

	t.Run("handles path without templates", func(t *testing.T) {
		f := NewExecutableFinder("myapp",
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
		)
		path, err := f.expandPath("/usr/local/bin/myapp")
		require.NoError(t, err)
		require.Equal(t, filepath.Clean("/usr/local/bin/myapp"), path)
	})

	t.Run("does not expand tilde in middle of path", func(t *testing.T) {
		f := NewExecutableFinder("myapp",
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
		)
		path, err := f.expandPath("/some/path/~/{name}")
		require.NoError(t, err)
		// Tilde in middle should remain unchanged
		require.Equal(t, filepath.Clean("/some/path/~/myapp"), path)
	})
}

func TestExecutableFinder_isExecutable(t *testing.T) {
	t.Run("Unix: returns true for executable file", func(t *testing.T) {
		f := NewExecutableFinder("myapp", withGOOS("linux"))
		info := mockFileInfo{name: "myapp", mode: 0755}
		require.True(t, f.isExecutable(info))
	})

	t.Run("Unix: returns true for group executable", func(t *testing.T) {
		f := NewExecutableFinder("myapp", withGOOS("darwin"))
		info := mockFileInfo{name: "myapp", mode: 0750}
		require.True(t, f.isExecutable(info))
	})

	t.Run("Unix: returns true for other executable", func(t *testing.T) {
		f := NewExecutableFinder("myapp", withGOOS("linux"))
		info := mockFileInfo{name: "myapp", mode: 0705}
		require.True(t, f.isExecutable(info))
	})

	t.Run("Unix: returns false for non-executable file", func(t *testing.T) {
		f := NewExecutableFinder("myapp", withGOOS("linux"))
		info := mockFileInfo{name: "myapp", mode: 0644}
		require.False(t, f.isExecutable(info))
	})

	t.Run("Windows: returns true for .exe file", func(t *testing.T) {
		f := NewExecutableFinder("myapp", withGOOS("windows"))
		info := mockFileInfo{name: "myapp.exe", mode: 0644} // mode doesn't matter on Windows
		require.True(t, f.isExecutable(info))
	})

	t.Run("Windows: returns true for .EXE file (case insensitive)", func(t *testing.T) {
		f := NewExecutableFinder("myapp", withGOOS("windows"))
		info := mockFileInfo{name: "MYAPP.EXE", mode: 0644}
		require.True(t, f.isExecutable(info))
	})

	t.Run("Windows: returns false for non-.exe file", func(t *testing.T) {
		f := NewExecutableFinder("myapp", withGOOS("windows"))
		info := mockFileInfo{name: "myapp", mode: 0755}
		require.False(t, f.isExecutable(info))
	})
}

func TestExecutableFinder_Find_EnvOverride(t *testing.T) {
	t.Run("finds executable via env override first", func(t *testing.T) {
		t.Setenv("CLAUDE_PATH", "/custom/claude")

		f := NewExecutableFinder("claude",
			WithEnvOverride("CLAUDE_PATH"),
			WithKnownPaths("~/.claude/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				if path == "/custom/claude" {
					return mockFileInfo{name: "claude", mode: 0755}, nil
				}
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not in PATH")
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, "/custom/claude", path)
	})

	t.Run("skips env override if file not executable", func(t *testing.T) {
		envPath := filepath.FromSlash("/custom/claude")
		t.Setenv("CLAUDE_PATH", envPath)

		expectedPath := filepath.FromSlash("/home/test/.claude/local/claude")
		f := NewExecutableFinder("claude",
			WithEnvOverride("CLAUDE_PATH"),
			WithKnownPaths("~/.claude/local/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return filepath.FromSlash("/home/test"), nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				if path == envPath {
					return mockFileInfo{name: "claude", mode: 0644}, nil // Not executable
				}
				if path == expectedPath {
					return mockFileInfo{name: "claude", mode: 0755}, nil
				}
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not in PATH")
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, expectedPath, path)
	})

	t.Run("skips env override if not set", func(t *testing.T) {
		// Ensure env var is not set
		os.Unsetenv("NONEXISTENT_VAR")

		expectedPath := filepath.FromSlash("/home/test/.claude/claude")
		f := NewExecutableFinder("claude",
			WithEnvOverride("NONEXISTENT_VAR"),
			WithKnownPaths("~/.claude/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return filepath.FromSlash("/home/test"), nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				if path == expectedPath {
					return mockFileInfo{name: "claude", mode: 0755}, nil
				}
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not in PATH")
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, expectedPath, path)
	})
}

func TestExecutableFinder_Find_KnownPaths(t *testing.T) {
	t.Run("finds in first known path", func(t *testing.T) {
		firstPath := filepath.FromSlash("/home/test/.claude/local/claude")
		secondPath := filepath.FromSlash("/home/test/.claude/claude")
		f := NewExecutableFinder("claude",
			WithKnownPaths("~/.claude/local/{name}", "~/.claude/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return filepath.FromSlash("/home/test"), nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				if path == firstPath {
					return mockFileInfo{name: "claude", mode: 0755}, nil
				}
				if path == secondPath {
					return mockFileInfo{name: "claude", mode: 0755}, nil
				}
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not in PATH")
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, firstPath, path)
	})

	t.Run("finds in second known path when first doesn't exist", func(t *testing.T) {
		expectedPath := filepath.FromSlash("/home/test/.claude/claude")
		f := NewExecutableFinder("claude",
			WithKnownPaths("~/.claude/local/{name}", "~/.claude/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return filepath.FromSlash("/home/test"), nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				if path == expectedPath {
					return mockFileInfo{name: "claude", mode: 0755}, nil
				}
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not in PATH")
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, expectedPath, path)
	})

	t.Run("respects priority order", func(t *testing.T) {
		checkedOrder := []string{}
		firstPath := filepath.FromSlash("/first/myapp")
		secondPath := filepath.FromSlash("/second/myapp")
		thirdPath := filepath.FromSlash("/third/myapp")

		f := NewExecutableFinder("myapp",
			WithKnownPaths("/first/{name}", "/second/{name}", "/third/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return filepath.FromSlash("/home/test"), nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				checkedOrder = append(checkedOrder, path)
				if path == thirdPath {
					return mockFileInfo{name: "myapp", mode: 0755}, nil
				}
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not in PATH")
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, thirdPath, path)
		require.Equal(t, []string{firstPath, secondPath, thirdPath}, checkedOrder)
	})
}

func TestExecutableFinder_Find_FallbackToPATH(t *testing.T) {
	t.Run("falls back to PATH when no known paths match", func(t *testing.T) {
		f := NewExecutableFinder("claude",
			WithKnownPaths("~/.claude/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				if name == "claude" {
					return "/usr/bin/claude", nil
				}
				return "", errors.New("not found")
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, "/usr/bin/claude", path)
	})

	t.Run("falls back to PATH when no known paths configured", func(t *testing.T) {
		f := NewExecutableFinder("amp",
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				if name == "amp" {
					return "/usr/local/bin/amp", nil
				}
				return "", errors.New("not found")
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, "/usr/local/bin/amp", path)
	})

	t.Run("uses .exe suffix for PATH lookup on Windows", func(t *testing.T) {
		var lookedUpName string
		f := NewExecutableFinder("claude",
			withGOOS("windows"),
			withUserHomeFunc(func() (string, error) { return "C:\\Users\\test", nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				lookedUpName = name
				return "C:\\Program Files\\claude\\claude.exe", nil
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, "C:\\Program Files\\claude\\claude.exe", path)
		require.Equal(t, "claude.exe", lookedUpName)
	})
}

func TestExecutableFinder_Find_DirectoryRejected(t *testing.T) {
	t.Run("rejects directory with same name", func(t *testing.T) {
		f := NewExecutableFinder("claude",
			WithKnownPaths("~/.claude/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				if path == "/home/test/.claude/claude" {
					return mockFileInfo{name: "claude", isDir: true, mode: 0755}, nil
				}
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "/usr/bin/claude", nil
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		// Should skip the directory and find via PATH
		require.Equal(t, "/usr/bin/claude", path)
	})
}

func TestExecutableFinder_Find_NotExecutable(t *testing.T) {
	t.Run("Unix: rejects file without execute permission", func(t *testing.T) {
		f := NewExecutableFinder("myapp",
			WithKnownPaths("~/.local/bin/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				if path == "/home/test/.local/bin/myapp" {
					return mockFileInfo{name: "myapp", mode: 0644}, nil // Not executable
				}
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "/usr/bin/myapp", nil
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		// Should skip non-executable and find via PATH
		require.Equal(t, "/usr/bin/myapp", path)
	})
}

func TestExecutableFinder_Find_ErrorMessage(t *testing.T) {
	t.Run("error includes all checked paths", func(t *testing.T) {
		f := NewExecutableFinder("notfound",
			WithKnownPaths("~/.local/bin/{name}", "/opt/tools/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return filepath.FromSlash("/home/test"), nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not found")
			}),
		)

		_, err := f.Find()
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrExecutableNotFound))
		require.Contains(t, err.Error(), "notfound")
		// Use platform-agnostic path checks
		require.Contains(t, err.Error(), filepath.FromSlash("/home/test/.local/bin/notfound"))
		require.Contains(t, err.Error(), filepath.FromSlash("/opt/tools/notfound"))
		require.Contains(t, err.Error(), "PATH")
	})

	t.Run("error includes env override path when checked", func(t *testing.T) {
		t.Setenv("MY_PATH", "/invalid/path")

		f := NewExecutableFinder("notfound",
			WithEnvOverride("MY_PATH"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not found")
			}),
		)

		_, err := f.Find()
		require.Error(t, err)
		require.Contains(t, err.Error(), "/invalid/path")
		require.Contains(t, err.Error(), "$MY_PATH")
	})

	t.Run("error is ErrExecutableNotFound", func(t *testing.T) {
		f := NewExecutableFinder("notfound",
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "/home/test", nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not found")
			}),
		)

		_, err := f.Find()
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrExecutableNotFound))
	})
}

func TestExecutableFinder_Find_CrossPlatform(t *testing.T) {
	t.Run("Windows: adds .exe suffix to {name}", func(t *testing.T) {
		var statPath string
		f := NewExecutableFinder("claude",
			WithKnownPaths("C:\\Program Files\\Claude\\{name}"),
			withGOOS("windows"),
			withUserHomeFunc(func() (string, error) { return "C:\\Users\\test", nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				statPath = path
				return mockFileInfo{name: "claude.exe", mode: 0644}, nil
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not found")
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, filepath.Clean("C:\\Program Files\\Claude\\claude.exe"), path)
		require.Contains(t, statPath, "claude.exe")
	})

	t.Run("Darwin: no .exe suffix", func(t *testing.T) {
		var statPath string
		expectedPath := filepath.FromSlash("/Applications/Claude.app/Contents/MacOS/claude")
		f := NewExecutableFinder("claude",
			WithKnownPaths("/Applications/Claude.app/Contents/MacOS/{name}"),
			withGOOS("darwin"),
			withUserHomeFunc(func() (string, error) { return filepath.FromSlash("/Users/test"), nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				statPath = path
				return mockFileInfo{name: "claude", mode: 0755}, nil
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not found")
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, expectedPath, path)
		require.Equal(t, expectedPath, statPath)
	})

	t.Run("Linux: no .exe suffix", func(t *testing.T) {
		expectedPath := filepath.FromSlash("/home/user/.npm/bin/gemini")
		f := NewExecutableFinder("gemini",
			WithKnownPaths("~/.npm/bin/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return filepath.FromSlash("/home/user"), nil }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				if path == expectedPath {
					return mockFileInfo{name: "gemini", mode: 0755}, nil
				}
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not found")
			}),
		)

		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, expectedPath, path)
	})
}

func TestExecutableFinder_Find_SkipsInvalidTemplates(t *testing.T) {
	t.Run("skips path when home directory lookup fails", func(t *testing.T) {
		expectedPath := filepath.FromSlash("/usr/local/bin/claude")
		f := NewExecutableFinder("claude",
			WithKnownPaths("~/.claude/{name}", "/usr/local/bin/{name}"),
			withGOOS("linux"),
			withUserHomeFunc(func() (string, error) { return "", errors.New("no home") }),
			withStatFunc(func(path string) (os.FileInfo, error) {
				if path == expectedPath {
					return mockFileInfo{name: "claude", mode: 0755}, nil
				}
				return nil, os.ErrNotExist
			}),
			withLookPathFunc(func(name string) (string, error) {
				return "", errors.New("not found")
			}),
		)

		// Should skip the ~ path and find in /usr/local/bin
		path, err := f.Find()
		require.NoError(t, err)
		require.Equal(t, expectedPath, path)
	})
}

func TestExecutableFinder_platformExecName(t *testing.T) {
	t.Run("returns name with .exe on Windows", func(t *testing.T) {
		f := NewExecutableFinder("myapp", withGOOS("windows"))
		require.Equal(t, "myapp.exe", f.platformExecName())
	})

	t.Run("returns plain name on Darwin", func(t *testing.T) {
		f := NewExecutableFinder("myapp", withGOOS("darwin"))
		require.Equal(t, "myapp", f.platformExecName())
	})

	t.Run("returns plain name on Linux", func(t *testing.T) {
		f := NewExecutableFinder("myapp", withGOOS("linux"))
		require.Equal(t, "myapp", f.platformExecName())
	})
}

func TestExecutableFinder_isValidExecutable(t *testing.T) {
	t.Run("returns false for stat error", func(t *testing.T) {
		f := NewExecutableFinder("myapp",
			withGOOS("linux"),
			withStatFunc(func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}),
		)
		require.False(t, f.isValidExecutable("/nonexistent/path"))
	})

	t.Run("returns false for directory", func(t *testing.T) {
		f := NewExecutableFinder("myapp",
			withGOOS("linux"),
			withStatFunc(func(path string) (os.FileInfo, error) {
				return mockFileInfo{name: "myapp", isDir: true}, nil
			}),
		)
		require.False(t, f.isValidExecutable("/some/dir"))
	})

	t.Run("returns true for valid executable", func(t *testing.T) {
		f := NewExecutableFinder("myapp",
			withGOOS("linux"),
			withStatFunc(func(path string) (os.FileInfo, error) {
				return mockFileInfo{name: "myapp", mode: 0755}, nil
			}),
		)
		require.True(t, f.isValidExecutable("/usr/bin/myapp"))
	})
}

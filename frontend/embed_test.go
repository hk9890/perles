package frontend

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistFS_ContainsIndexHTML(t *testing.T) {
	fsys := DistFS()

	// The embedded FS has paths prefixed with "dist/"
	content, err := fs.ReadFile(fsys, "dist/index.html")
	require.NoError(t, err, "dist/index.html should exist in embedded FS")
	assert.Contains(t, string(content), "<!DOCTYPE html>", "index.html should contain DOCTYPE")
	assert.Contains(t, string(content), "<div id=\"root\">", "index.html should contain React root div")
}

func TestDistFS_ContainsAssetsDirectory(t *testing.T) {
	fsys := DistFS()

	// List the assets directory
	entries, err := fs.ReadDir(fsys, "dist/assets")
	require.NoError(t, err, "dist/assets directory should exist")
	require.NotEmpty(t, entries, "dist/assets should contain files")

	// Verify we have at least one JS and one CSS file
	var hasJS, hasCSS bool
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > 3 && name[len(name)-3:] == ".js" {
			hasJS = true
		}
		if len(name) > 4 && name[len(name)-4:] == ".css" {
			hasCSS = true
		}
	}

	assert.True(t, hasJS, "dist/assets should contain at least one .js file")
	assert.True(t, hasCSS, "dist/assets should contain at least one .css file")
}

func TestDistFS_CanSub(t *testing.T) {
	fsys := DistFS()

	// Test that we can use fs.Sub to strip the dist/ prefix
	subFS, err := fs.Sub(fsys, "dist")
	require.NoError(t, err, "should be able to create sub FS at dist/")

	// Now paths should work without the dist/ prefix
	content, err := fs.ReadFile(subFS, "index.html")
	require.NoError(t, err, "index.html should be accessible after fs.Sub")
	assert.Contains(t, string(content), "<!DOCTYPE html>")
}

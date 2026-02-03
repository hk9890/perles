package vimtextarea

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeNewlines_NoChange(t *testing.T) {
	input := "line1\nline2\n"
	require.Equal(t, input, normalizeNewlines(input))
}

func TestNormalizeNewlines_CRLF(t *testing.T) {
	input := "line1\r\nline2\r\n"
	expected := "line1\nline2\n"
	require.Equal(t, expected, normalizeNewlines(input))
}

func TestNormalizeNewlines_CR(t *testing.T) {
	input := "line1\rline2\r"
	expected := "line1\nline2\n"
	require.Equal(t, expected, normalizeNewlines(input))
}

func TestNormalizeNewlines_Mixed(t *testing.T) {
	input := "line1\r\nline2\rline3\nline4"
	expected := "line1\nline2\nline3\nline4"
	require.Equal(t, expected, normalizeNewlines(input))
}

package vimtextarea

import "strings"

// normalizeNewlines converts CRLF/CR line endings to LF to prevent carriage
// returns from affecting terminal layout during rendering.
func normalizeNewlines(s string) string {
	if !strings.Contains(s, "\r") {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

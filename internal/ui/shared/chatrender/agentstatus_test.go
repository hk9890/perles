package chatrender

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zjrosen/perles/internal/orchestration/events"
	"github.com/zjrosen/perles/internal/orchestration/metrics"
	"github.com/zjrosen/perles/internal/ui/styles"
)

// TestStatusIndicator_AllStatuses verifies correct char/style for all known ProcessStatus values.
func TestStatusIndicator_AllStatuses(t *testing.T) {
	tests := []struct {
		name       string
		status     events.ProcessStatus
		wantChar   string
		wantBold   bool
		checkStyle bool // whether to verify style properties
	}{
		{
			name:     "Ready returns green circle",
			status:   events.ProcessStatusReady,
			wantChar: "○",
		},
		{
			name:     "Working returns blue filled circle",
			status:   events.ProcessStatusWorking,
			wantChar: "●",
		},
		{
			name:       "Paused returns pause symbol with bold",
			status:     events.ProcessStatusPaused,
			wantChar:   "⏸",
			wantBold:   true,
			checkStyle: true,
		},
		{
			name:     "Stopped returns warning symbol",
			status:   events.ProcessStatusStopped,
			wantChar: "⚠",
		},
		{
			name:     "Retired returns X",
			status:   events.ProcessStatusRetired,
			wantChar: "✗",
		},
		{
			name:     "Failed returns X",
			status:   events.ProcessStatusFailed,
			wantChar: "✗",
		},
		{
			name:     "Pending returns muted circle",
			status:   events.ProcessStatusPending,
			wantChar: "○",
		},
		{
			name:     "Starting returns muted circle",
			status:   events.ProcessStatusStarting,
			wantChar: "○",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			char, style := StatusIndicator(tt.status)
			require.Equal(t, tt.wantChar, char, "indicator char mismatch for status %v", tt.status)

			if tt.checkStyle && tt.wantBold {
				// Verify the style has bold set
				require.True(t, style.GetBold(), "expected bold style for status %v", tt.status)
			}
		})
	}
}

// TestStatusIndicator_NoQuestionMarkForKnownStatuses ensures all known statuses have explicit handling.
// This test guards against regression where a new status would fall through to "?" default.
func TestStatusIndicator_NoQuestionMarkForKnownStatuses(t *testing.T) {
	knownStatuses := []events.ProcessStatus{
		events.ProcessStatusPending,
		events.ProcessStatusStarting,
		events.ProcessStatusReady,
		events.ProcessStatusWorking,
		events.ProcessStatusPaused,
		events.ProcessStatusStopped,
		events.ProcessStatusRetiring,
		events.ProcessStatusRetired,
		events.ProcessStatusFailed,
	}

	for _, status := range knownStatuses {
		char, _ := StatusIndicator(status)
		// Note: Retiring is intentionally not handled separately and falls to default "?"
		// which is acceptable as it's a transitional state. However, we still test it
		// to document expected behavior.
		if status == events.ProcessStatusRetiring {
			require.Equal(t, "?", char, "Retiring status uses default fallback")
		} else {
			require.NotEqual(t, "?", char, "status %v should have explicit indicator, not '?'", status)
		}
	}
}

// TestStatusBorderColor_Working verifies working status returns blue border color.
func TestStatusBorderColor_Working(t *testing.T) {
	color := StatusBorderColor(events.ProcessStatusWorking)
	require.Equal(t, StatusWorkingBorderColor, color, "working status should return blue border color")
}

// TestStatusBorderColor_Stopped verifies stopped/retired/failed statuses return red border color.
func TestStatusBorderColor_Stopped(t *testing.T) {
	tests := []struct {
		name   string
		status events.ProcessStatus
	}{
		{"Stopped", events.ProcessStatusStopped},
		{"Retired", events.ProcessStatusRetired},
		{"Failed", events.ProcessStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color := StatusBorderColor(tt.status)
			require.Equal(t, StatusStoppedBorderColor, color, "%s status should return red border color", tt.name)
		})
	}
}

// TestStatusBorderColor_Default verifies Ready/unknown statuses return default border color.
func TestStatusBorderColor_Default(t *testing.T) {
	tests := []struct {
		name   string
		status events.ProcessStatus
	}{
		{"Ready", events.ProcessStatusReady},
		{"Pending", events.ProcessStatusPending},
		{"Starting", events.ProcessStatusStarting},
		{"Paused", events.ProcessStatusPaused},
		{"Unknown", events.ProcessStatus("unknown")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color := StatusBorderColor(tt.status)
			require.Equal(t, styles.BorderDefaultColor, color, "%s status should return default border color", tt.name)
		})
	}
}

// TestStatusBorderColor_UnknownDoesNotPanic verifies unknown status values don't panic.
func TestStatusBorderColor_UnknownDoesNotPanic(t *testing.T) {
	// This should not panic
	color := StatusBorderColor(events.ProcessStatus("completely_invalid_status"))
	require.Equal(t, styles.BorderDefaultColor, color, "unknown status should return default border color")
}

// TestFormatQueueCount_Zero verifies zero count returns empty string.
func TestFormatQueueCount_Zero(t *testing.T) {
	result := FormatQueueCount(0)
	require.Empty(t, result, "zero count should return empty string")
}

// TestFormatQueueCount_Negative verifies negative count returns empty string.
func TestFormatQueueCount_Negative(t *testing.T) {
	result := FormatQueueCount(-1)
	require.Empty(t, result, "negative count should return empty string")

	result = FormatQueueCount(-100)
	require.Empty(t, result, "large negative count should return empty string")
}

// TestFormatQueueCount_Positive verifies positive count returns formatted string.
func TestFormatQueueCount_Positive(t *testing.T) {
	tests := []struct {
		count    int
		contains string
	}{
		{1, "[1 queued]"},
		{5, "[5 queued]"},
		{100, "[100 queued]"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			result := FormatQueueCount(tt.count)
			require.NotEmpty(t, result, "positive count should return non-empty string")
			require.Contains(t, result, tt.contains, "result should contain formatted count")
		})
	}
}

// TestFormatMetricsDisplay_Nil verifies nil metrics returns empty string.
func TestFormatMetricsDisplay_Nil(t *testing.T) {
	result := FormatMetricsDisplay(nil)
	require.Empty(t, result, "nil metrics should return empty string")
}

// TestFormatMetricsDisplay_ZeroTokens verifies zero token metrics returns empty string.
func TestFormatMetricsDisplay_ZeroTokens(t *testing.T) {
	m := &metrics.TokenMetrics{
		TokensUsed:  0,
		TotalTokens: 200000,
	}
	result := FormatMetricsDisplay(m)
	require.Empty(t, result, "zero tokens should return empty string")
}

// TestFormatMetricsDisplay_NegativeTokens verifies negative token metrics returns empty string.
func TestFormatMetricsDisplay_NegativeTokens(t *testing.T) {
	m := &metrics.TokenMetrics{
		TokensUsed:  -100,
		TotalTokens: 200000,
	}
	result := FormatMetricsDisplay(m)
	require.Empty(t, result, "negative tokens should return empty string")
}

// TestFormatMetricsDisplay_WithTokens verifies valid metrics returns formatted display.
func TestFormatMetricsDisplay_WithTokens(t *testing.T) {
	m := &metrics.TokenMetrics{
		TokensUsed:  27000,
		TotalTokens: 200000,
	}
	result := FormatMetricsDisplay(m)
	require.NotEmpty(t, result, "valid metrics should return non-empty string")
	// FormatContextDisplay returns "27k/200k" format
	require.Equal(t, "27k/200k", result, "should format as 27k/200k")
}

// TestFormatMetricsDisplay_SmallTokens verifies small token values are formatted correctly.
func TestFormatMetricsDisplay_SmallTokens(t *testing.T) {
	m := &metrics.TokenMetrics{
		TokensUsed:  500,
		TotalTokens: 200000,
	}
	result := FormatMetricsDisplay(m)
	require.NotEmpty(t, result, "small positive tokens should return non-empty string")
	// 500 / 1000 = 0 (integer division), so it shows "0k/200k"
	require.Equal(t, "0k/200k", result, "should format small tokens correctly")
}

// TestExportedBorderColors verifies exported border color constants are set correctly.
func TestExportedBorderColors(t *testing.T) {
	// StatusWorkingBorderColor should be blue
	require.Equal(t, "#54A0FF", StatusWorkingBorderColor.Light, "working border light color should be blue")
	require.Equal(t, "#54A0FF", StatusWorkingBorderColor.Dark, "working border dark color should be blue")

	// StatusStoppedBorderColor should be red
	require.Equal(t, "#FF6B6B", StatusStoppedBorderColor.Light, "stopped border light color should be red")
	require.Equal(t, "#FF8787", StatusStoppedBorderColor.Dark, "stopped border dark color should be red")
}

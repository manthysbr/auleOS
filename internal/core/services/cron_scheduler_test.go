package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchesCronField(t *testing.T) {
	tests := []struct {
		pattern string
		value   int
		want    bool
	}{
		{"*", 0, true},
		{"*", 59, true},
		{"0", 0, true},
		{"0", 1, false},
		{"*/5", 0, true},
		{"*/5", 5, true},
		{"*/5", 10, true},
		{"*/5", 3, false},
		{"*/15", 0, true},
		{"*/15", 15, true},
		{"*/15", 30, true},
		{"*/15", 7, false},
		{"1,5,10", 5, true},
		{"1,5,10", 3, false},
		{"1,5,10", 10, true},
		{"30", 30, true},
		{"30", 31, false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+string(rune('0'+tt.value)), func(t *testing.T) {
			got := matchesCronField(tt.pattern, tt.value)
			assert.Equal(t, tt.want, got, "pattern=%q value=%d", tt.pattern, tt.value)
		})
	}
}

func TestNextCronRun(t *testing.T) {
	// "0 9 * * *" = every day at 9:00 AM
	base := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)
	next, err := nextCronRun("0 9 * * *", base)
	require.NoError(t, err)
	assert.Equal(t, 9, next.Hour())
	assert.Equal(t, 0, next.Minute())
	assert.Equal(t, 1, next.Day()) // same day, just 1 hour later

	// From 9:30 should go to next day
	base2 := time.Date(2025, 1, 1, 9, 30, 0, 0, time.UTC)
	next2, err := nextCronRun("0 9 * * *", base2)
	require.NoError(t, err)
	assert.Equal(t, 9, next2.Hour())
	assert.Equal(t, 2, next2.Day()) // next day

	// "*/30 * * * *" = every 30 minutes
	base3 := time.Date(2025, 1, 1, 12, 10, 0, 0, time.UTC)
	next3, err := nextCronRun("*/30 * * * *", base3)
	require.NoError(t, err)
	assert.Equal(t, 30, next3.Minute())

	// Invalid expression
	_, err = nextCronRun("bad expr", base)
	assert.Error(t, err)
}

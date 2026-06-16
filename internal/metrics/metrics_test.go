package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
)

func TestRecordSnapshotAndReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	store := NewStore(path)
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return now }

	require.NoError(t, store.Record(Event{
		At:                  now,
		Command:             "web-search",
		Provider:            "duckduckgo",
		Status:              "success",
		DurationMS:          100,
		ResultCount:         3,
		Quality:             "high",
		FallbackRecommended: true,
	}))

	snap, err := store.Snapshot(RangeAll, BucketAuto)
	require.NoError(t, err)
	assert.Equal(t, int64(1), snap.Commands["web-search"].Total)
	assert.Equal(t, int64(1), snap.Commands["web-search"].Success)
	assert.Equal(t, int64(100), snap.Commands["web-search"].AvgDurationMS)
	assert.Equal(t, int64(3), snap.Commands["web-search"].ResultCount)
	assert.Equal(t, int64(1), snap.Providers["search:duckduckgo"].Success)
	assert.Equal(t, int64(1), snap.ReaderQuality.High)
	assert.Equal(t, int64(1), snap.ReaderQuality.FallbackRecommended)
	assert.Len(t, snap.RecentEvents, 1)

	require.NoError(t, store.Reset())
	snap, err = store.Snapshot(RangeAll, BucketAuto)
	require.NoError(t, err)
	assert.Empty(t, snap.Commands)
	assert.Empty(t, snap.RecentEvents)
}

func TestErrorEventAndPrivacy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	store := NewStore(path)
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return now }

	require.NoError(t, store.Record(Event{
		At:            now,
		Command:       "web-reader",
		Provider:      "builtin-reader",
		Status:        "error",
		ErrorCategory: "network",
		DurationMS:    50,
	}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "https://secret.example.com")
	assert.NotContains(t, string(data), "private search query marker")

	var snap Snapshot
	require.NoError(t, json.Unmarshal(data, &snap))
	assert.Equal(t, int64(1), snap.Commands["web-reader"].Error)
	assert.Equal(t, int64(1), snap.Errors["network"])
	assert.Equal(t, "network", snap.RecentEvents[0].ErrorCategory)
}

func TestRecentEventsRingBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	store := NewStore(path)
	base := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return base.Add(30 * time.Minute) }

	for i := 0; i < 25; i++ {
		require.NoError(t, store.Record(Event{
			At:         base.Add(time.Duration(i) * time.Minute),
			Command:    "web-search",
			Status:     "success",
			DurationMS: int64(i),
		}))
	}

	snap, err := store.Snapshot(RangeAll, BucketAuto)
	require.NoError(t, err)
	require.Len(t, snap.RecentEvents, 20)
	assert.Equal(t, int64(5), snap.RecentEvents[0].DurationMS)
	assert.Equal(t, int64(24), snap.RecentEvents[19].DurationMS)
}

func TestRangeFilterUsesBuckets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	store := NewStore(path)
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return now }

	require.NoError(t, store.Record(Event{
		At:         now.Add(-2 * time.Hour),
		Command:    "web-search",
		Status:     "success",
		DurationMS: 100,
	}))
	require.NoError(t, store.Record(Event{
		At:         now.Add(-30 * time.Minute),
		Command:    "web-search",
		Status:     "error",
		DurationMS: 200,
	}))

	oneHour, err := store.Snapshot(Range1H, BucketAuto)
	require.NoError(t, err)
	assert.Equal(t, int64(1), oneHour.Commands["web-search"].Total)
	assert.Equal(t, int64(1), oneHour.Commands["web-search"].Error)
	assert.False(t, oneHour.Period.FirstSeenAt.IsZero())
	assert.False(t, oneHour.Period.LastSeenAt.IsZero())

	all, err := store.Snapshot(RangeAll, BucketAuto)
	require.NoError(t, err)
	assert.Equal(t, int64(2), all.Commands["web-search"].Total)
}

func TestCorruptRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.json")
	require.NoError(t, os.WriteFile(path, []byte("{not-json"), 0644))
	store := NewStore(path)
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return now }

	snap, err := store.Snapshot(RangeAll, BucketAuto)
	require.NoError(t, err)
	assert.Contains(t, strings.Join(snap.Warnings, ","), "corrupt")
	matches, err := filepath.Glob(path + ".corrupt.*")
	require.NoError(t, err)
	assert.Len(t, matches, 1)
}

func TestDisabledDoesNotWrite(t *testing.T) {
	t.Setenv("WEB_TOOLS_NO_METRICS", "1")
	path := filepath.Join(t.TempDir(), "metrics.json")
	store := NewStore(path)
	require.NoError(t, store.Record(Event{Command: "web-search", Status: "success"}))
	require.NoFileExists(t, path)

	snap, err := store.Snapshot(RangeAll, BucketAuto)
	require.NoError(t, err)
	assert.True(t, snap.Disabled)
}

func TestParseRangeAndBucket(t *testing.T) {
	_, err := ParseRange("3h")
	require.Error(t, err)
	r, err := ParseRange("24h")
	require.NoError(t, err)
	assert.Equal(t, Range24H, r)

	_, err = ParseBucket("minute")
	require.Error(t, err)
	b, err := ParseBucket("day")
	require.NoError(t, err)
	assert.Equal(t, BucketDay, b)
}

func TestErrorCategoryUsesAppErrorCategoryOnly(t *testing.T) {
	err := apperrors.NewInputError(
		"bad input",
		"https://secret.example.com/private?q=hidden",
		[]string{"fix the input"},
	)

	assert.Equal(t, "input", ErrorCategory(err))
}

func TestDefaultPathUsesOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-metrics.json")
	t.Setenv("WEB_TOOLS_METRICS_FILE", path)

	assert.Equal(t, path, DefaultPath())
}

func TestWordCountBucket(t *testing.T) {
	assert.Empty(t, WordCountBucket(0))
	assert.Equal(t, "lt200", WordCountBucket(50))
	assert.Equal(t, "200_999", WordCountBucket(500))
	assert.Equal(t, "gte1000", WordCountBucket(1000))
}

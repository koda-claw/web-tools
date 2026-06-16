package metricscmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/koda-claw/web-tools/internal/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShowMetricsJSONAndReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	store := metrics.NewStore(path)
	require.NoError(t, store.Record(metrics.Event{
		At:         time.Now(),
		Command:    "web-search",
		Status:     "success",
		DurationMS: 10,
	}))

	stdout := captureStdout(t, func() {
		require.NoError(t, showMetrics(path, "all", "auto", true))
	})
	assert.Contains(t, stdout, `"schema_version": 1`)
	assert.Contains(t, stdout, `"web-search"`)

	cmd := Cmd()
	cmd.SetArgs([]string{"reset", "--file", path, "--json"})
	stdout = captureStdout(t, func() {
		require.NoError(t, cmd.Execute())
	})
	assert.Contains(t, stdout, `"ok":true`)

	snap, err := store.Snapshot(metrics.RangeAll, metrics.BucketAuto)
	require.NoError(t, err)
	assert.Empty(t, snap.Commands)
}

func TestShowMetricsInvalidRange(t *testing.T) {
	err := showMetrics(filepath.Join(t.TempDir(), "metrics.json"), "3h", "auto", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid metrics range")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	require.NoError(t, w.Close())
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String()
}

package metrics_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ayush18/networkbooster/core/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistory_SaveAndListSessions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	h, err := metrics.NewHistory(dbPath)
	require.NoError(t, err)
	defer h.Close()

	session := metrics.Session{
		StartTime:     time.Now().Add(-5 * time.Minute),
		EndTime:       time.Now(),
		Mode:          "download",
		Profile:       "medium",
		Connections:   8,
		TotalDownload: 500 * 1024 * 1024,
		TotalUpload:   0,
		PeakDownload:  150.5,
		PeakUpload:    0,
		AvgDownload:   120.3,
		AvgUpload:     0,
	}

	err = h.SaveSession(session)
	require.NoError(t, err)

	sessions, err := h.ListSessions(10)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "download", sessions[0].Mode)
	assert.Equal(t, int64(500*1024*1024), sessions[0].TotalDownload)
}

func TestHistory_TotalStats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	h, err := metrics.NewHistory(dbPath)
	require.NoError(t, err)
	defer h.Close()

	h.SaveSession(metrics.Session{
		StartTime: time.Now(), EndTime: time.Now(),
		TotalDownload: 100, TotalUpload: 50,
	})
	h.SaveSession(metrics.Session{
		StartTime: time.Now(), EndTime: time.Now(),
		TotalDownload: 200, TotalUpload: 75,
	})

	stats, err := h.TotalStats()
	require.NoError(t, err)
	assert.Equal(t, int64(300), stats.TotalDownload)
	assert.Equal(t, int64(125), stats.TotalUpload)
	assert.Equal(t, 2, stats.SessionCount)
}

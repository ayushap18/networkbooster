package metrics_test

import (
	"testing"
	"time"

	"github.com/ayush18/networkbooster/core/metrics"
	"github.com/stretchr/testify/assert"
)

func TestCollector_RecordAndSnapshot(t *testing.T) {
	c := metrics.NewCollector()

	c.RecordBytes(metrics.DirectionDownload, 1000)
	c.RecordBytes(metrics.DirectionDownload, 500)
	c.RecordBytes(metrics.DirectionUpload, 200)

	snap := c.Snapshot()
	assert.Equal(t, int64(1500), snap.TotalDownloadBytes)
	assert.Equal(t, int64(200), snap.TotalUploadBytes)
}

func TestCollector_SpeedCalculation(t *testing.T) {
	c := metrics.NewCollector()

	c.RecordBytes(metrics.DirectionDownload, 1*1024*1024)
	time.Sleep(150 * time.Millisecond)

	snap := c.Snapshot()
	assert.Greater(t, snap.DownloadMbps, float64(0))
}

func TestCollector_ConnectionCount(t *testing.T) {
	c := metrics.NewCollector()

	c.AddConnection("server-1")
	c.AddConnection("server-2")
	assert.Equal(t, 2, c.Snapshot().ActiveConnections)

	c.RemoveConnection("server-1")
	assert.Equal(t, 1, c.Snapshot().ActiveConnections)
}

func TestCollector_Reset(t *testing.T) {
	c := metrics.NewCollector()
	c.RecordBytes(metrics.DirectionDownload, 1000)
	c.AddConnection("s1")

	c.Reset()

	snap := c.Snapshot()
	assert.Equal(t, int64(0), snap.TotalDownloadBytes)
	assert.Equal(t, 0, snap.ActiveConnections)
}

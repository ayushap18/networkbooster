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

func TestCollector_PerServerStats(t *testing.T) {
	c := metrics.NewCollector()

	c.RecordServerBytes("server-1", metrics.DirectionDownload, 1000)
	c.RecordServerBytes("server-1", metrics.DirectionDownload, 500)
	c.RecordServerBytes("server-2", metrics.DirectionDownload, 2000)

	snap := c.Snapshot()
	assert.Equal(t, int64(3500), snap.TotalDownloadBytes)

	servers := snap.ServerStats
	assert.Len(t, servers, 2)
	assert.Equal(t, int64(1500), servers["server-1"].DownloadBytes)
	assert.Equal(t, int64(2000), servers["server-2"].DownloadBytes)
}

func TestCollector_PeakSpeed(t *testing.T) {
	c := metrics.NewCollector()

	c.RecordBytes(metrics.DirectionDownload, 5*1024*1024)
	time.Sleep(150 * time.Millisecond)
	c.Snapshot() // first reading

	c.RecordBytes(metrics.DirectionDownload, 1*1024*1024)
	time.Sleep(150 * time.Millisecond)
	snap2 := c.Snapshot()

	assert.GreaterOrEqual(t, snap2.PeakDownloadMbps, snap2.DownloadMbps)
	assert.Greater(t, snap2.AvgDownloadMbps, float64(0))
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

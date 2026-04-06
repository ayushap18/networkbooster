package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

type Direction int

const (
	DirectionDownload Direction = iota
	DirectionUpload
)

type ServerStat struct {
	DownloadBytes int64
	UploadBytes   int64
}

type Snapshot struct {
	DownloadMbps       float64
	UploadMbps         float64
	TotalDownloadBytes int64
	TotalUploadBytes   int64
	ActiveConnections  int
	Elapsed            time.Duration
	PeakDownloadMbps   float64
	PeakUploadMbps     float64
	AvgDownloadMbps    float64
	AvgUploadMbps      float64
	ServerStats        map[string]ServerStat
}

type serverAccum struct {
	download int64
	upload   int64
}

const smoothingWindowSize = 5

type Collector struct {
	mu sync.RWMutex

	totalDownload atomic.Int64
	totalUpload   atomic.Int64

	windowDownload atomic.Int64
	windowUpload   atomic.Int64
	windowStart    time.Time

	connections map[string]struct{}
	startTime   time.Time

	// Per-server tracking
	serverStats map[string]*serverAccum

	// Peak/avg tracking
	peakDl, peakUl       float64
	speedSamples         int
	totalDlMbps          float64
	totalUlMbps          float64

	// Rolling speed smoothing
	recentDl []float64
	recentUl []float64
}

func NewCollector() *Collector {
	now := time.Now()
	return &Collector{
		connections: make(map[string]struct{}),
		serverStats: make(map[string]*serverAccum),
		startTime:   now,
		windowStart: now,
	}
}

func (c *Collector) RecordBytes(dir Direction, n int64) {
	switch dir {
	case DirectionDownload:
		c.totalDownload.Add(n)
		c.windowDownload.Add(n)
	case DirectionUpload:
		c.totalUpload.Add(n)
		c.windowUpload.Add(n)
	}
}

func (c *Collector) RecordServerBytes(serverID string, dir Direction, n int64) {
	c.RecordBytes(dir, n)

	c.mu.Lock()
	defer c.mu.Unlock()
	acc, ok := c.serverStats[serverID]
	if !ok {
		acc = &serverAccum{}
		c.serverStats[serverID] = acc
	}
	switch dir {
	case DirectionDownload:
		acc.download += n
	case DirectionUpload:
		acc.upload += n
	}
}

func (c *Collector) AddConnection(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connections[id] = struct{}{}
}

func (c *Collector) RemoveConnection(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.connections, id)
}

func (c *Collector) Snapshot() Snapshot {
	c.mu.RLock()
	connCount := len(c.connections)
	c.mu.RUnlock()

	now := time.Now()
	elapsed := now.Sub(c.windowStart).Seconds()
	if elapsed < 0.001 {
		elapsed = 0.001
	}

	dlBytes := c.windowDownload.Swap(0)
	ulBytes := c.windowUpload.Swap(0)

	instantDl := float64(dlBytes) * 8.0 / (elapsed * 1_000_000)
	instantUl := float64(ulBytes) * 8.0 / (elapsed * 1_000_000)

	c.mu.Lock()
	c.windowStart = now

	// Rolling window for smoothed display speed
	c.recentDl = append(c.recentDl, instantDl)
	if len(c.recentDl) > smoothingWindowSize {
		c.recentDl = c.recentDl[len(c.recentDl)-smoothingWindowSize:]
	}
	c.recentUl = append(c.recentUl, instantUl)
	if len(c.recentUl) > smoothingWindowSize {
		c.recentUl = c.recentUl[len(c.recentUl)-smoothingWindowSize:]
	}

	var dlMbps, ulMbps float64
	for _, v := range c.recentDl {
		dlMbps += v
	}
	dlMbps /= float64(len(c.recentDl))
	for _, v := range c.recentUl {
		ulMbps += v
	}
	ulMbps /= float64(len(c.recentUl))

	// Update peak (use instantaneous, not smoothed)
	if instantDl > c.peakDl {
		c.peakDl = instantDl
	}
	if instantUl > c.peakUl {
		c.peakUl = instantUl
	}

	// Update running average
	c.speedSamples++
	c.totalDlMbps += instantDl
	c.totalUlMbps += instantUl

	peakDl := c.peakDl
	peakUl := c.peakUl
	avgDl := c.totalDlMbps / float64(c.speedSamples)
	avgUl := c.totalUlMbps / float64(c.speedSamples)

	// Copy server stats
	serverStats := make(map[string]ServerStat, len(c.serverStats))
	for id, acc := range c.serverStats {
		serverStats[id] = ServerStat{
			DownloadBytes: acc.download,
			UploadBytes:   acc.upload,
		}
	}
	c.mu.Unlock()

	return Snapshot{
		DownloadMbps:       dlMbps,
		UploadMbps:         ulMbps,
		TotalDownloadBytes: c.totalDownload.Load(),
		TotalUploadBytes:   c.totalUpload.Load(),
		ActiveConnections:  connCount,
		Elapsed:            now.Sub(c.startTime),
		PeakDownloadMbps:   peakDl,
		PeakUploadMbps:     peakUl,
		AvgDownloadMbps:    avgDl,
		AvgUploadMbps:      avgUl,
		ServerStats:        serverStats,
	}
}

func (c *Collector) Reset() {
	c.totalDownload.Store(0)
	c.totalUpload.Store(0)
	c.windowDownload.Store(0)
	c.windowUpload.Store(0)

	c.mu.Lock()
	c.connections = make(map[string]struct{})
	c.serverStats = make(map[string]*serverAccum)
	c.peakDl = 0
	c.peakUl = 0
	c.speedSamples = 0
	c.totalDlMbps = 0
	c.totalUlMbps = 0
	c.recentDl = nil
	c.recentUl = nil
	now := time.Now()
	c.startTime = now
	c.windowStart = now
	c.mu.Unlock()
}

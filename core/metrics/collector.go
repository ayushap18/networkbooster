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

type Snapshot struct {
	DownloadMbps       float64
	UploadMbps         float64
	TotalDownloadBytes int64
	TotalUploadBytes   int64
	ActiveConnections  int
	Elapsed            time.Duration
}

type Collector struct {
	mu sync.RWMutex

	totalDownload atomic.Int64
	totalUpload   atomic.Int64

	windowDownload atomic.Int64
	windowUpload   atomic.Int64
	windowStart    time.Time

	connections map[string]struct{}
	startTime   time.Time
}

func NewCollector() *Collector {
	now := time.Now()
	return &Collector{
		connections: make(map[string]struct{}),
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
	c.mu.Lock()
	c.windowStart = now
	c.mu.Unlock()

	dlMbps := float64(dlBytes) * 8.0 / (elapsed * 1_000_000)
	ulMbps := float64(ulBytes) * 8.0 / (elapsed * 1_000_000)

	return Snapshot{
		DownloadMbps:       dlMbps,
		UploadMbps:         ulMbps,
		TotalDownloadBytes: c.totalDownload.Load(),
		TotalUploadBytes:   c.totalUpload.Load(),
		ActiveConnections:  connCount,
		Elapsed:            now.Sub(c.startTime),
	}
}

func (c *Collector) Reset() {
	c.totalDownload.Store(0)
	c.totalUpload.Store(0)
	c.windowDownload.Store(0)
	c.windowUpload.Store(0)

	c.mu.Lock()
	c.connections = make(map[string]struct{})
	now := time.Now()
	c.startTime = now
	c.windowStart = now
	c.mu.Unlock()
}

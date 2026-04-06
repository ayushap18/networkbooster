package engine

import (
	"context"
	"sync"
	"time"

	"github.com/ayush18/networkbooster/core/metrics"
)

// ConnSetter is the interface the scaler uses to adjust connections.
type ConnSetter interface {
	SetConnections(n int)
	ConnectionCount() int
	IsPaused() bool
}

// ScalerOptions configures the adaptive scaler.
type ScalerOptions struct {
	MinConnections int
	MaxConnections int
	Interval       time.Duration
}

// Scaler monitors throughput samples and adjusts connection count accordingly.
type Scaler struct {
	opts       ScalerOptions
	target     ConnSetter
	history    []float64
	stallCount int
	mu         sync.Mutex
}

// NewScaler creates a new Scaler with the given options and target.
// Default values are applied for any zero-value option fields.
func NewScaler(opts ScalerOptions, target ConnSetter) *Scaler {
	if opts.MinConnections <= 0 {
		opts.MinConnections = 2
	}
	if opts.MaxConnections <= 0 {
		opts.MaxConnections = 64
	}
	if opts.Interval <= 0 {
		opts.Interval = 5 * time.Second
	}
	return &Scaler{
		opts:   opts,
		target: target,
	}
}

// RecordSample feeds a throughput sample and evaluates whether to scale up or down.
func (s *Scaler) RecordSample(throughputMbps float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.history = append(s.history, throughputMbps)
	if len(s.history) > 3 {
		s.history = s.history[len(s.history)-3:]
	}

	if len(s.history) < 3 {
		return
	}

	oldest := s.history[0]
	latest := s.history[2]

	var delta float64
	if oldest > 0 {
		delta = (latest - oldest) / oldest
	}

	current := s.target.ConnectionCount()

	if delta >= 0.10 {
		// Strong improvement: add 2 connections
		s.stallCount = 0
		next := current + 2
		if next > s.opts.MaxConnections {
			next = s.opts.MaxConnections
		}
		if next != current {
			s.target.SetConnections(next)
		}
	} else if delta >= 0.05 {
		// Moderate improvement: add 1 connection
		s.stallCount = 0
		next := current + 1
		if next > s.opts.MaxConnections {
			next = s.opts.MaxConnections
		}
		if next != current {
			s.target.SetConnections(next)
		}
	} else if delta < 0.01 {
		// Flat or declining
		s.stallCount++
		if s.stallCount >= 3 {
			next := current - 1
			if next < s.opts.MinConnections {
				next = s.opts.MinConnections
			}
			if next != current {
				s.target.SetConnections(next)
			}
			s.stallCount = 0
		}
	} else {
		// Marginal improvement: hold steady
		s.stallCount = 0
	}
}

// ResetHistory clears the sample history and stall counter.
// Should be called after a safety pause.
func (s *Scaler) ResetHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = nil
	s.stallCount = 0
}

// RunLoop is the main scaling loop. It ticks at opts.Interval, reads a snapshot
// from collector, and feeds the combined throughput to RecordSample.
func (s *Scaler) RunLoop(ctx context.Context, collector *metrics.Collector) {
	ticker := time.NewTicker(s.opts.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.target.IsPaused() {
				s.ResetHistory()
				continue
			}
			snap := collector.Snapshot()
			totalMbps := snap.DownloadMbps + snap.UploadMbps
			s.RecordSample(totalMbps)
		}
	}
}

package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ayush18/networkbooster/core/metrics"
	"github.com/ayush18/networkbooster/core/sources"
)

type Mode int

const (
	ModeDownload Mode = iota
	ModeUpload
	ModeBidirectional
)

type Options struct {
	Connections int
}

type Status struct {
	Running  bool
	Paused   bool
	Mode     Mode
	Snapshot metrics.Snapshot
}

type workerEntry struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type Engine struct {
	registry  *sources.Registry
	opts      Options
	collector *metrics.Collector

	mu         sync.Mutex
	running    bool
	paused     bool
	mode       Mode
	parentCtx  context.Context
	parentStop context.CancelFunc
	workers    []workerEntry
	workerSeq  int
	discovered []sources.DiscoveredServer
}

func New(registry *sources.Registry, opts Options) *Engine {
	if opts.Connections <= 0 {
		opts.Connections = 8
	}
	return &Engine{
		registry:  registry,
		opts:      opts,
		collector: metrics.NewCollector(),
	}
}

func (e *Engine) Start(mode Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return errors.New("engine is already running")
	}

	discovered, err := e.registry.DiscoverAll()
	if err != nil || len(discovered) == 0 {
		if err != nil {
			return fmt.Errorf("discovery failed: %w", err)
		}
		return errors.New("no servers discovered")
	}

	e.collector.Reset()
	e.running = true
	e.paused = false
	e.mode = mode
	e.discovered = discovered
	e.workerSeq = 0
	e.workers = nil

	e.parentCtx, e.parentStop = context.WithCancel(context.Background())

	for i := 0; i < e.opts.Connections; i++ {
		e.addWorkerLocked()
	}
	return nil
}

// addWorkerLocked launches ONE worker with its own child context of parentCtx.
// Must be called with e.mu held.
func (e *Engine) addWorkerLocked() {
	ctx, cancel := context.WithCancel(e.parentCtx)
	done := make(chan struct{})

	i := e.workerSeq
	e.workerSeq++

	ds := e.discovered[i%len(e.discovered)]
	workerID := fmt.Sprintf("worker-%d", i)

	switch e.mode {
	case ModeDownload:
		w := NewWorker(workerID, ds.Source, ds.Server, e.collector)
		go func() {
			defer close(done)
			w.RunDownload(ctx)
		}()
	case ModeUpload:
		w := NewWorker(workerID, ds.Source, ds.Server, e.collector)
		go func() {
			defer close(done)
			w.RunUpload(ctx)
		}()
	case ModeBidirectional:
		totalConns := e.opts.Connections
		if i < (totalConns+1)/2 {
			dlServer := sources.Server{
				ID:      ds.Server.ID,
				Name:    ds.Server.Name,
				URL:     ds.Server.URL + "/download",
				Latency: ds.Server.Latency,
			}
			w := NewWorker(workerID+"-dl", ds.Source, dlServer, e.collector)
			go func() {
				defer close(done)
				w.RunDownload(ctx)
			}()
		} else {
			ulServer := sources.Server{
				ID:      ds.Server.ID,
				Name:    ds.Server.Name,
				URL:     ds.Server.URL + "/upload",
				Latency: ds.Server.Latency,
			}
			w := NewWorker(workerID+"-ul", ds.Source, ulServer, e.collector)
			go func() {
				defer close(done)
				w.RunUpload(ctx)
			}()
		}
	}

	e.workers = append(e.workers, workerEntry{cancel: cancel, done: done})
}

// removeLastWorkerLocked cancels the most recently added worker (LIFO) and waits for it to finish.
// Must be called with e.mu held. Releases and re-acquires the lock while waiting.
func (e *Engine) removeLastWorkerLocked() {
	if len(e.workers) == 0 {
		return
	}
	last := e.workers[len(e.workers)-1]
	e.workers = e.workers[:len(e.workers)-1]
	last.cancel()
	e.mu.Unlock()
	<-last.done
	e.mu.Lock()
}

func (e *Engine) Stop() error {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return errors.New("engine is not running")
	}
	e.parentStop()
	workers := e.workers
	e.workers = nil
	e.mu.Unlock()

	for _, w := range workers {
		<-w.done
	}

	e.mu.Lock()
	e.running = false
	e.paused = false
	e.mu.Unlock()
	return nil
}

// Pause stops all workers but keeps the engine in running state.
// Call Resume() to restart workers.
func (e *Engine) Pause() {
	e.mu.Lock()
	if !e.running || e.paused {
		e.mu.Unlock()
		return
	}
	e.parentStop()
	workers := e.workers
	e.workers = nil
	e.mu.Unlock()

	for _, w := range workers {
		<-w.done
	}

	e.mu.Lock()
	e.paused = true
	e.mu.Unlock()
}

// Resume restarts workers after a Pause.
func (e *Engine) Resume() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running || !e.paused {
		return
	}
	e.paused = false
	e.workerSeq = 0
	e.workers = nil
	e.parentCtx, e.parentStop = context.WithCancel(context.Background())
	for i := 0; i < e.opts.Connections; i++ {
		e.addWorkerLocked()
	}
}

// IsPaused returns whether the engine is paused.
func (e *Engine) IsPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.paused
}

// SetConnections adjusts the worker count dynamically.
// No-op if the engine is paused or stopped.
func (e *Engine) SetConnections(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running || e.paused || n < 0 {
		return
	}

	current := len(e.workers)
	if n > current {
		for i := 0; i < n-current; i++ {
			e.addWorkerLocked()
		}
	} else if n < current {
		for i := 0; i < current-n; i++ {
			e.removeLastWorkerLocked()
		}
	}
	e.opts.Connections = n
}

// ConnectionCount returns the current number of workers.
func (e *Engine) ConnectionCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.workers)
}

func (e *Engine) Status() Status {
	e.mu.Lock()
	running := e.running
	paused := e.paused
	mode := e.mode
	e.mu.Unlock()

	return Status{
		Running:  running,
		Paused:   paused,
		Mode:     mode,
		Snapshot: e.collector.Snapshot(),
	}
}

func (e *Engine) Collector() *metrics.Collector {
	return e.collector
}

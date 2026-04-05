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
	Mode     Mode
	Snapshot metrics.Snapshot
}

type Engine struct {
	registry  *sources.Registry
	opts      Options
	collector *metrics.Collector

	mu      sync.Mutex
	running bool
	mode    Mode
	cancel  context.CancelFunc
	wg      sync.WaitGroup
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
	e.mode = mode

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel

	for i := 0; i < e.opts.Connections; i++ {
		ds := discovered[i%len(discovered)]
		workerID := fmt.Sprintf("worker-%d", i)

		switch mode {
		case ModeDownload:
			w := NewWorker(workerID, ds.Source, ds.Server, e.collector)
			e.wg.Add(1)
			go func() {
				defer e.wg.Done()
				w.RunDownload(ctx)
			}()
		case ModeUpload:
			w := NewWorker(workerID, ds.Source, ds.Server, e.collector)
			e.wg.Add(1)
			go func() {
				defer e.wg.Done()
				w.RunUpload(ctx)
			}()
		case ModeBidirectional:
			if i < (e.opts.Connections+1)/2 {
				dlServer := sources.Server{
					ID:      ds.Server.ID,
					Name:    ds.Server.Name,
					URL:     ds.Server.URL + "/download",
					Latency: ds.Server.Latency,
				}
				w := NewWorker(workerID+"-dl", ds.Source, dlServer, e.collector)
				e.wg.Add(1)
				go func() {
					defer e.wg.Done()
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
				e.wg.Add(1)
				go func() {
					defer e.wg.Done()
					w.RunUpload(ctx)
				}()
			}
		}
	}

	return nil
}

func (e *Engine) Stop() error {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return errors.New("engine is not running")
	}
	e.cancel()
	e.mu.Unlock()

	e.wg.Wait()

	e.mu.Lock()
	e.running = false
	e.mu.Unlock()
	return nil
}

func (e *Engine) Status() Status {
	e.mu.Lock()
	running := e.running
	mode := e.mode
	e.mu.Unlock()

	return Status{
		Running:  running,
		Mode:     mode,
		Snapshot: e.collector.Snapshot(),
	}
}

func (e *Engine) Collector() *metrics.Collector {
	return e.collector
}

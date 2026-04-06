# Phase 5: Adaptive Scaling + Speedtest Source — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Ookla speedtest source (live discovery + bundled fallback) and adaptive connection scaling that dynamically adjusts worker count based on throughput.

**Architecture:** New `SpeedtestSource` in `core/sources/` following existing Source interface. New `Scaler` in `core/engine/` that monitors collector throughput and calls `SetConnections()` on the engine. Engine refactored to support per-worker contexts for dynamic add/remove.

**Tech Stack:** Go stdlib (net/http, encoding/xml, math), existing testutil mock server, testify assertions

---

### Task 1: Speedtest Source — Discovery + Bundled Fallback

**Files:**
- Create: `core/sources/speedtest.go`
- Create: `core/sources/speedtest_test.go`

- [ ] **Step 1: Write failing test for bundled fallback discovery**

In `core/sources/speedtest_test.go`:

```go
package sources_test

import (
	"testing"

	"github.com/ayush18/networkbooster/core/sources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpeedtestSource_Name(t *testing.T) {
	src := sources.NewSpeedtestSource()
	assert.Equal(t, "Speedtest", src.Name())
	assert.Equal(t, sources.SourceTypeSpeedtest, src.Type())
}

func TestSpeedtestSource_BundledFallback(t *testing.T) {
	src := sources.NewSpeedtestSource()
	servers := src.BundledServers()
	require.Greater(t, len(servers), 10, "should have at least 10 bundled servers")
	for _, s := range servers {
		assert.NotEmpty(t, s.ID)
		assert.NotEmpty(t, s.Name)
		assert.NotEmpty(t, s.URL)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./core/sources/ -run TestSpeedtestSource -v`
Expected: FAIL — `NewSpeedtestSource` not defined

- [ ] **Step 3: Implement SpeedtestSource struct with bundled servers**

In `core/sources/speedtest.go`:

```go
package sources

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

var bundledSpeedtestServers = []Server{
	{ID: "st-1", Name: "New York, US", URL: "https://speed.measurementlab.net/ndt_protocol"},
	{ID: "st-2", Name: "Los Angeles, US", URL: "https://la.speedtest.clouvider.net/backend"},
	{ID: "st-3", Name: "London, UK", URL: "https://lon.speedtest.clouvider.net/backend"},
	{ID: "st-4", Name: "Frankfurt, DE", URL: "https://fra.speedtest.clouvider.net/backend"},
	{ID: "st-5", Name: "Amsterdam, NL", URL: "https://ams.speedtest.clouvider.net/backend"},
	{ID: "st-6", Name: "Paris, FR", URL: "https://par.speedtest.clouvider.net/backend"},
	{ID: "st-7", Name: "Tokyo, JP", URL: "https://tyo.speedtest.clouvider.net/backend"},
	{ID: "st-8", Name: "Singapore, SG", URL: "https://sg.speedtest.clouvider.net/backend"},
	{ID: "st-9", Name: "Sydney, AU", URL: "https://syd.speedtest.clouvider.net/backend"},
	{ID: "st-10", Name: "São Paulo, BR", URL: "https://sp.speedtest.clouvider.net/backend"},
	{ID: "st-11", Name: "Mumbai, IN", URL: "https://mum.speedtest.clouvider.net/backend"},
	{ID: "st-12", Name: "Toronto, CA", URL: "https://tor.speedtest.clouvider.net/backend"},
	{ID: "st-13", Name: "Chicago, US", URL: "https://chi.speedtest.clouvider.net/backend"},
	{ID: "st-14", Name: "Dallas, US", URL: "https://dal.speedtest.clouvider.net/backend"},
	{ID: "st-15", Name: "Miami, US", URL: "https://mia.speedtest.clouvider.net/backend"},
	{ID: "st-16", Name: "Warsaw, PL", URL: "https://war.speedtest.clouvider.net/backend"},
	{ID: "st-17", Name: "Dubai, AE", URL: "https://dxb.speedtest.clouvider.net/backend"},
	{ID: "st-18", Name: "Hong Kong, HK", URL: "https://hkg.speedtest.clouvider.net/backend"},
	{ID: "st-19", Name: "Seoul, KR", URL: "https://sel.speedtest.clouvider.net/backend"},
	{ID: "st-20", Name: "Stockholm, SE", URL: "https://sto.speedtest.clouvider.net/backend"},
}

// xmlServerList is the Ookla server list XML format.
type xmlServerList struct {
	XMLName xml.Name    `xml:"settings"`
	Servers []xmlServer `xml:"servers>server"`
}

type xmlServer struct {
	URL     string `xml:"url,attr"`
	Lat     string `xml:"lat,attr"`
	Lon     string `xml:"lon,attr"`
	Name    string `xml:"name,attr"`
	Country string `xml:"country,attr"`
	Sponsor string `xml:"sponsor,attr"`
	ID      string `xml:"id,attr"`
}

const ooklaServerListURL = "https://www.speedtest.net/speedtest-servers-static.php"

type SpeedtestSource struct {
	client       *http.Client
	serverListURL string
}

func NewSpeedtestSource() *SpeedtestSource {
	return &SpeedtestSource{
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 64,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		serverListURL: ooklaServerListURL,
	}
}

func (s *SpeedtestSource) Name() string     { return "Speedtest" }
func (s *SpeedtestSource) Type() SourceType { return SourceTypeSpeedtest }

func (s *SpeedtestSource) BundledServers() []Server {
	out := make([]Server, len(bundledSpeedtestServers))
	copy(out, bundledSpeedtestServers)
	return out
}

func (s *SpeedtestSource) fetchLiveServers() ([]Server, error) {
	resp, err := s.client.Get(s.serverListURL)
	if err != nil {
		return nil, fmt.Errorf("fetch server list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server list returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read server list: %w", err)
	}

	var list xmlServerList
	if err := xml.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse server list: %w", err)
	}

	servers := make([]Server, 0, len(list.Servers))
	for _, xs := range list.Servers {
		if xs.URL == "" || xs.ID == "" {
			continue
		}
		servers = append(servers, Server{
			ID:   "ookla-" + xs.ID,
			Name: fmt.Sprintf("%s (%s, %s)", xs.Sponsor, xs.Name, xs.Country),
			URL:  xs.URL,
		})
	}
	if len(servers) == 0 {
		return nil, errors.New("no servers in list")
	}
	return servers, nil
}

func (s *SpeedtestSource) Discover() ([]Server, error) {
	live, err := s.fetchLiveServers()
	if err == nil && len(live) > 0 {
		return live, nil
	}
	// Fallback to bundled
	return s.BundledServers(), nil
}

func (s *SpeedtestSource) Download(ctx context.Context, server Server) (io.ReadCloser, error) {
	dlURL := server.URL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, errors.New("unexpected status: " + resp.Status)
	}
	return resp.Body, nil
}

func (s *SpeedtestSource) Upload(ctx context.Context, server Server, r io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("upload failed: " + resp.Status)
	}
	return nil
}

func (s *SpeedtestSource) Latency(server Server) (time.Duration, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, server.URL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return time.Since(start), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./core/sources/ -run TestSpeedtestSource -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add core/sources/speedtest.go core/sources/speedtest_test.go
git commit -m "feat: add speedtest source with bundled server fallback"
```

---

### Task 2: Speedtest Source — Live Discovery with Mock XML

**Files:**
- Modify: `core/sources/speedtest_test.go`

- [ ] **Step 1: Write failing test for live XML discovery**

Append to `core/sources/speedtest_test.go`:

```go
import (
	"context"
	"io"
	"net/http/httptest"
	"net/http"
)

func TestSpeedtestSource_LiveDiscovery(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<settings>
  <servers>
    <server url="http://example.com/speedtest" lat="40.7128" lon="-74.0060" name="New York" country="US" sponsor="TestISP" id="9999"/>
    <server url="http://example.com/speedtest2" lat="51.5074" lon="-0.1278" name="London" country="GB" sponsor="TestISP2" id="9998"/>
  </servers>
</settings>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(xmlBody))
	}))
	defer ts.Close()

	src := sources.NewSpeedtestSourceWithURL(ts.URL)
	servers, err := src.Discover()
	require.NoError(t, err)
	assert.Len(t, servers, 2)
	assert.Equal(t, "ookla-9999", servers[0].ID)
	assert.Contains(t, servers[0].Name, "TestISP")
	assert.Equal(t, "http://example.com/speedtest", servers[0].URL)
}

func TestSpeedtestSource_LiveDiscoveryFails_FallsToBundled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	src := sources.NewSpeedtestSourceWithURL(ts.URL)
	servers, err := src.Discover()
	require.NoError(t, err)
	assert.Greater(t, len(servers), 10, "should fall back to bundled servers")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./core/sources/ -run TestSpeedtestSource_Live -v`
Expected: FAIL — `NewSpeedtestSourceWithURL` not defined

- [ ] **Step 3: Add NewSpeedtestSourceWithURL constructor**

Add to `core/sources/speedtest.go`:

```go
func NewSpeedtestSourceWithURL(serverListURL string) *SpeedtestSource {
	src := NewSpeedtestSource()
	src.serverListURL = serverListURL
	return src
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./core/sources/ -run TestSpeedtestSource -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add core/sources/speedtest.go core/sources/speedtest_test.go
git commit -m "feat: add live XML discovery with fallback for speedtest source"
```

---

### Task 3: Speedtest Source — Download + Upload + Latency Tests

**Files:**
- Modify: `core/sources/speedtest_test.go`

- [ ] **Step 1: Write failing tests for download, upload, and latency**

Append to `core/sources/speedtest_test.go`:

```go
import "github.com/ayush18/networkbooster/internal/testutil"

func TestSpeedtestSource_Download(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	src := sources.NewSpeedtestSource()
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL + "/download"}

	rc, err := src.Download(context.Background(), server)
	require.NoError(t, err)
	defer rc.Close()

	n, err := io.Copy(io.Discard, rc)
	require.NoError(t, err)
	assert.Greater(t, n, int64(0))
}

func TestSpeedtestSource_Upload(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	src := sources.NewSpeedtestSource()
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL + "/upload"}

	data := make([]byte, 1024)
	err := src.Upload(context.Background(), server, io.NopCloser(bytes.NewReader(data)))
	require.NoError(t, err)
}

func TestSpeedtestSource_Latency(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	src := sources.NewSpeedtestSource()
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL + "/download"}

	lat, err := src.Latency(server)
	require.NoError(t, err)
	assert.Greater(t, lat, time.Duration(0))
}
```

- [ ] **Step 2: Run tests to verify they pass**

These should pass immediately since Download/Upload/Latency are already implemented.

Run: `go test ./core/sources/ -run "TestSpeedtestSource_(Download|Upload|Latency)" -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add core/sources/speedtest_test.go
git commit -m "test: add download, upload, and latency tests for speedtest source"
```

---

### Task 4: Engine — Per-Worker Context + SetConnections + ConnectionCount

**Files:**
- Modify: `core/engine/engine.go`

- [ ] **Step 1: Write failing tests for SetConnections and ConnectionCount**

Create `core/engine/engine_adaptive_test.go`:

```go
package engine_test

import (
	"testing"
	"time"

	"github.com/ayush18/networkbooster/core/engine"
	"github.com/ayush18/networkbooster/core/sources"
	"github.com/ayush18/networkbooster/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_ConnectionCount(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	reg.Register(sources.NewSelfHostedSource(ts.URL + "/download"))

	eng := engine.New(reg, engine.Options{Connections: 4})
	require.NoError(t, eng.Start(engine.ModeDownload))
	defer eng.Stop()

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 4, eng.ConnectionCount())
}

func TestEngine_SetConnections_ScaleUp(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	reg.Register(sources.NewSelfHostedSource(ts.URL + "/download"))

	eng := engine.New(reg, engine.Options{Connections: 2})
	require.NoError(t, eng.Start(engine.ModeDownload))
	defer eng.Stop()

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 2, eng.ConnectionCount())

	eng.SetConnections(5)
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 5, eng.ConnectionCount())
}

func TestEngine_SetConnections_ScaleDown(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	reg.Register(sources.NewSelfHostedSource(ts.URL + "/download"))

	eng := engine.New(reg, engine.Options{Connections: 6})
	require.NoError(t, eng.Start(engine.ModeDownload))
	defer eng.Stop()

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 6, eng.ConnectionCount())

	eng.SetConnections(3)
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 3, eng.ConnectionCount())
}

func TestEngine_SetConnections_WhilePaused_Noop(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	reg.Register(sources.NewSelfHostedSource(ts.URL + "/download"))

	eng := engine.New(reg, engine.Options{Connections: 4})
	require.NoError(t, eng.Start(engine.ModeDownload))
	defer eng.Stop()

	eng.Pause()
	time.Sleep(100 * time.Millisecond)

	eng.SetConnections(8)
	assert.Equal(t, 0, eng.ConnectionCount())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./core/engine/ -run "TestEngine_(ConnectionCount|SetConnections)" -v`
Expected: FAIL — `ConnectionCount` and `SetConnections` not defined

- [ ] **Step 3: Refactor engine to per-worker contexts**

Replace the worker management in `core/engine/engine.go`. The key changes:

1. Add a `workerEntry` struct holding per-worker cancel func and done channel
2. Track workers in a `[]workerEntry` slice instead of relying on a single cancel/WaitGroup
3. Add `SetConnections(n int)` and `ConnectionCount() int`

Replace the entire `core/engine/engine.go` with:

```go
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
	discovered []sources.DiscoveredServer
	workerSeq  int
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

	e.parentCtx, e.parentStop = context.WithCancel(context.Background())
	e.workers = nil

	for i := 0; i < e.opts.Connections; i++ {
		e.addWorkerLocked()
	}
	return nil
}

// addWorkerLocked launches one worker goroutine. Must be called with e.mu held.
func (e *Engine) addWorkerLocked() {
	ds := e.discovered[e.workerSeq%len(e.discovered)]
	id := fmt.Sprintf("worker-%d", e.workerSeq)
	e.workerSeq++

	ctx, cancel := context.WithCancel(e.parentCtx)
	done := make(chan struct{})

	entry := workerEntry{cancel: cancel, done: done}
	e.workers = append(e.workers, entry)

	switch e.mode {
	case ModeDownload:
		w := NewWorker(id, ds.Source, ds.Server, e.collector)
		go func() {
			defer close(done)
			w.RunDownload(ctx)
		}()
	case ModeUpload:
		w := NewWorker(id, ds.Source, ds.Server, e.collector)
		go func() {
			defer close(done)
			w.RunUpload(ctx)
		}()
	case ModeBidirectional:
		dlCount := len(e.workers)
		if dlCount <= (e.opts.Connections+1)/2 {
			dlServer := sources.Server{
				ID: ds.Server.ID, Name: ds.Server.Name,
				URL: ds.Server.URL + "/download", Latency: ds.Server.Latency,
			}
			w := NewWorker(id+"-dl", ds.Source, dlServer, e.collector)
			go func() {
				defer close(done)
				w.RunDownload(ctx)
			}()
		} else {
			ulServer := sources.Server{
				ID: ds.Server.ID, Name: ds.Server.Name,
				URL: ds.Server.URL + "/upload", Latency: ds.Server.Latency,
			}
			w := NewWorker(id+"-ul", ds.Source, ulServer, e.collector)
			go func() {
				defer close(done)
				w.RunUpload(ctx)
			}()
		}
	}
}

// removeLastWorkerLocked cancels the most recently added worker. Must be called with e.mu held.
func (e *Engine) removeLastWorkerLocked() {
	if len(e.workers) == 0 {
		return
	}
	last := e.workers[len(e.workers)-1]
	e.workers = e.workers[:len(e.workers)-1]
	last.cancel()
	<-last.done
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

func (e *Engine) Resume() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running || !e.paused {
		return
	}
	e.paused = false
	e.parentCtx, e.parentStop = context.WithCancel(context.Background())
	for i := 0; i < e.opts.Connections; i++ {
		e.addWorkerLocked()
	}
}

func (e *Engine) IsPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.paused
}

// SetConnections dynamically adjusts the number of active workers.
// If the engine is paused or not running, this is a no-op.
func (e *Engine) SetConnections(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running || e.paused {
		return
	}
	if n < 1 {
		n = 1
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

// ConnectionCount returns the number of active workers.
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
```

- [ ] **Step 4: Run all engine tests**

Run: `go test ./core/engine/ -v`
Expected: ALL PASS (both new adaptive tests and existing tests)

- [ ] **Step 5: Commit**

```bash
git add core/engine/engine.go core/engine/engine_adaptive_test.go
git commit -m "feat: refactor engine to per-worker contexts, add SetConnections and ConnectionCount"
```

---

### Task 5: Adaptive Scaler

**Files:**
- Create: `core/engine/scaler.go`
- Create: `core/engine/scaler_test.go`

- [ ] **Step 1: Write failing tests for scaler algorithm**

In `core/engine/scaler_test.go`:

```go
package engine_test

import (
	"testing"

	"github.com/ayush18/networkbooster/core/engine"
	"github.com/stretchr/testify/assert"
)

type mockConnSetter struct {
	conns int
}

func (m *mockConnSetter) SetConnections(n int) { m.conns = n }
func (m *mockConnSetter) ConnectionCount() int { return m.conns }
func (m *mockConnSetter) IsPaused() bool       { return false }

func TestScaler_RampUp(t *testing.T) {
	mock := &mockConnSetter{conns: 4}
	s := engine.NewScaler(engine.ScalerOptions{
		MinConnections: 2,
		MaxConnections: 20,
	}, mock)

	// Feed increasing throughput — should scale up
	s.RecordSample(100.0)
	s.RecordSample(108.0)
	s.RecordSample(117.0) // 17% increase over window — should add connections

	assert.Greater(t, mock.conns, 4, "scaler should have increased connections")
}

func TestScaler_Plateau_ThenBackoff(t *testing.T) {
	mock := &mockConnSetter{conns: 10}
	s := engine.NewScaler(engine.ScalerOptions{
		MinConnections: 2,
		MaxConnections: 20,
	}, mock)

	// Feed flat throughput — should eventually back off
	s.RecordSample(100.0)
	s.RecordSample(100.5)
	s.RecordSample(100.2) // flat, stall 1

	s.RecordSample(100.1)
	s.RecordSample(100.3)
	s.RecordSample(100.0) // flat, stall 2

	s.RecordSample(100.2)
	s.RecordSample(100.4)
	s.RecordSample(100.1) // flat, stall 3 — should back off

	assert.Less(t, mock.conns, 10, "scaler should have decreased connections after plateau")
}

func TestScaler_RespectsMax(t *testing.T) {
	mock := &mockConnSetter{conns: 19}
	s := engine.NewScaler(engine.ScalerOptions{
		MinConnections: 2,
		MaxConnections: 20,
	}, mock)

	s.RecordSample(100.0)
	s.RecordSample(110.0)
	s.RecordSample(125.0)

	assert.LessOrEqual(t, mock.conns, 20, "scaler should not exceed max")
}

func TestScaler_RespectsMin(t *testing.T) {
	mock := &mockConnSetter{conns: 3}
	s := engine.NewScaler(engine.ScalerOptions{
		MinConnections: 2,
		MaxConnections: 20,
	}, mock)

	// Feed declining throughput
	for i := 0; i < 12; i++ {
		s.RecordSample(100.0)
	}

	assert.GreaterOrEqual(t, mock.conns, 2, "scaler should not go below min")
}

func TestScaler_ResetHistory(t *testing.T) {
	mock := &mockConnSetter{conns: 8}
	s := engine.NewScaler(engine.ScalerOptions{
		MinConnections: 2,
		MaxConnections: 20,
	}, mock)

	s.RecordSample(100.0)
	s.RecordSample(100.0)
	s.ResetHistory()

	// After reset, needs 3 fresh samples before acting
	s.RecordSample(50.0) // should not change conns — not enough history
	assert.Equal(t, 8, mock.conns)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./core/engine/ -run TestScaler -v`
Expected: FAIL — `NewScaler` not defined

- [ ] **Step 3: Implement the scaler**

In `core/engine/scaler.go`:

```go
package engine

import (
	"context"
	"sync"
	"time"
)

// ConnSetter is the interface the scaler uses to adjust connections.
type ConnSetter interface {
	SetConnections(n int)
	ConnectionCount() int
	IsPaused() bool
}

type ScalerOptions struct {
	MinConnections int
	MaxConnections int
	Interval       time.Duration
}

type Scaler struct {
	opts   ScalerOptions
	target ConnSetter

	mu         sync.Mutex
	history    []float64
	stallCount int
}

func NewScaler(opts ScalerOptions, target ConnSetter) *Scaler {
	if opts.MinConnections < 1 {
		opts.MinConnections = 2
	}
	if opts.MaxConnections < opts.MinConnections {
		opts.MaxConnections = 64
	}
	if opts.Interval == 0 {
		opts.Interval = 5 * time.Second
	}
	return &Scaler{
		opts:   opts,
		target: target,
	}
}

// RecordSample feeds a throughput sample (Mbps) to the scaler and evaluates scaling.
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

	if oldest <= 0 {
		return
	}

	delta := (latest - oldest) / oldest
	current := s.target.ConnectionCount()

	if delta >= 0.05 {
		// Throughput improving — scale up
		s.stallCount = 0
		add := 1
		if delta >= 0.10 {
			add = 2
		}
		newCount := current + add
		if newCount > s.opts.MaxConnections {
			newCount = s.opts.MaxConnections
		}
		if newCount != current {
			s.target.SetConnections(newCount)
		}
	} else if delta < 0.01 {
		// Flat or declining
		s.stallCount++
		if s.stallCount >= 3 {
			newCount := current - 1
			if newCount < s.opts.MinConnections {
				newCount = s.opts.MinConnections
			}
			if newCount != current {
				s.target.SetConnections(newCount)
			}
			s.stallCount = 0
		}
	} else {
		// Marginal improvement — hold steady
		s.stallCount = 0
	}
}

// ResetHistory clears the throughput history and stall counter.
// Called when the engine resumes after a safety pause.
func (s *Scaler) ResetHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = nil
	s.stallCount = 0
}

// RunLoop starts the scaler's evaluation loop. It samples throughput from the
// collector every interval and adjusts connections. Blocks until ctx is cancelled.
func (s *Scaler) RunLoop(ctx context.Context, collector interface{ Snapshot() Snapshot }) {
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
```

Note: The `Snapshot` type in the `RunLoop` signature refers to `metrics.Snapshot`. Since this is in the `engine` package, we need to import metrics. Update the import and signature:

```go
import (
	"context"
	"sync"
	"time"

	"github.com/ayush18/networkbooster/core/metrics"
)

// RunLoop signature uses metrics.Collector directly:
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./core/engine/ -run TestScaler -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add core/engine/scaler.go core/engine/scaler_test.go
git commit -m "feat: add adaptive scaler with gradual ramp-up and backoff"
```

---

### Task 6: Config Changes — Adaptive Settings

**Files:**
- Modify: `config/config.go`
- Modify: `config/config_test.go`

- [ ] **Step 1: Write failing test for adaptive config**

Add to `config/config_test.go`:

```go
func TestConfig_AdaptiveDefaults(t *testing.T) {
	cfg := config.Default()
	assert.False(t, cfg.Adaptive.Enabled)
	assert.Equal(t, 5, cfg.Adaptive.IntervalSecs)
	assert.Equal(t, 2, cfg.Adaptive.MinConnections)
	assert.Equal(t, 0, cfg.Adaptive.MaxConnections)
}

func TestConfig_AdaptiveFromYAML(t *testing.T) {
	yaml := `
adaptive:
  enabled: true
  interval_secs: 10
  min_connections: 4
  max_connections: 32
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(tmpFile, []byte(yaml), 0644)

	cfg, err := config.Load(tmpFile)
	require.NoError(t, err)
	assert.True(t, cfg.Adaptive.Enabled)
	assert.Equal(t, 10, cfg.Adaptive.IntervalSecs)
	assert.Equal(t, 4, cfg.Adaptive.MinConnections)
	assert.Equal(t, 32, cfg.Adaptive.MaxConnections)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./config/ -run TestConfig_Adaptive -v`
Expected: FAIL — `Adaptive` field not defined on Config

- [ ] **Step 3: Add AdaptiveConfig to config**

In `config/config.go`, add the struct and wire it into Config:

```go
type AdaptiveConfig struct {
	Enabled        bool `yaml:"enabled"`
	IntervalSecs   int  `yaml:"interval_secs"`
	MinConnections int  `yaml:"min_connections"`
	MaxConnections int  `yaml:"max_connections"`
}
```

Add to `Config` struct:

```go
Adaptive      AdaptiveConfig  `yaml:"adaptive"`
```

Update `Default()` to include:

```go
Adaptive: AdaptiveConfig{
    Enabled:        false,
    IntervalSecs:   5,
    MinConnections: 2,
    MaxConnections: 0,
},
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./config/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/config_test.go
git commit -m "feat: add adaptive scaling config with defaults"
```

---

### Task 7: CLI Integration — Wire Speedtest + Scaler

**Files:**
- Modify: `cmd/cli/main.go`

- [ ] **Step 1: Register speedtest source in buildEngine**

In `cmd/cli/main.go`, in the `buildEngine` function, add after the CDN source registration:

```go
reg.Register(sources.NewSpeedtestSource())
```

- [ ] **Step 2: Add --adaptive flag**

In the `init()` function, add:

```go
startCmd.Flags().BoolVarP(&adaptiveFlag, "adaptive", "a", false, "Enable adaptive connection scaling")
```

Add the variable declaration:

```go
var adaptiveFlag bool
```

- [ ] **Step 3: Wire scaler into runStart**

In `runStart`, after `eng.Start(mode)` and the safety monitor, add scaler startup:

```go
// Start adaptive scaler if enabled
var scalerCancel context.CancelFunc
if cfg.Adaptive.Enabled || adaptiveFlag {
    scalerCtx, sc := context.WithCancel(context.Background())
    scalerCancel = sc

    maxConns := cfg.Adaptive.MaxConnections
    if maxConns == 0 && cfg.Safety.MaxConnections > 0 {
        maxConns = cfg.Safety.MaxConnections
    }
    if maxConns == 0 {
        maxConns = 64
    }

    scaler := engine.NewScaler(engine.ScalerOptions{
        MinConnections: cfg.Adaptive.MinConnections,
        MaxConnections: maxConns,
        Interval:       time.Duration(cfg.Adaptive.IntervalSecs) * time.Second,
    }, eng)

    go scaler.RunLoop(scalerCtx, eng.Collector())
}
```

Before `eng.Stop()`, cancel the scaler:

```go
if scalerCancel != nil {
    scalerCancel()
}
```

- [ ] **Step 4: Build and verify**

Run: `go build ./cmd/cli/`
Expected: Success, no errors

- [ ] **Step 5: Run all tests**

Run: `go test ./... `
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/cli/main.go
git commit -m "feat: wire speedtest source and adaptive scaler into CLI"
```

---

### Task 8: Integration — Full Build + Vet + Test

- [ ] **Step 1: Run go vet**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 2: Run all tests**

Run: `go test ./...`
Expected: ALL PASS

- [ ] **Step 3: Verify binary runs**

Run: `go build -o /tmp/networkbooster ./cmd/cli/ && /tmp/networkbooster --help`
Expected: Shows help with all commands including `start --adaptive`

- [ ] **Step 4: Commit plan file**

```bash
git add docs/superpowers/plans/2026-04-06-phase5-adaptive-scaling-speedtest.md
git commit -m "Add Phase 5 implementation plan: adaptive scaling + speedtest source"
```

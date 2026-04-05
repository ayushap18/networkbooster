# Phase 1: Core Engine + Sources + CLI — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a working CLI tool that opens parallel connections to CDN/Speedtest servers and continuously saturates bandwidth, displaying live download/upload speed in the terminal.

**Architecture:** Monorepo Go module with `core/engine` (connection pool + adaptive scaling), `core/sources` (pluggable server backends — CDN and self-hosted for Phase 1), `config` (YAML loader), and `cmd/cli` (cobra CLI with basic live output). TDD throughout.

**Tech Stack:** Go 1.22+, Cobra (CLI), gopkg.in/yaml.v3 (config), net/http (connections), testify (assertions), net/http/httptest (test servers)

---

## File Structure

```
networkbooster/
├── go.mod
├── go.sum
├── Makefile
├── cmd/
│   └── cli/
│       └── main.go                    # CLI entry point, cobra root command
├── core/
│   ├── engine/
│   │   ├── engine.go                  # Engine struct, Start/Stop/Status
│   │   ├── engine_test.go             # Engine integration tests
│   │   ├── worker.go                  # Single download/upload worker goroutine
│   │   └── worker_test.go             # Worker unit tests
│   ├── sources/
│   │   ├── source.go                  # Source/Server interfaces + SourceType enum
│   │   ├── registry.go                # Source registry (add/list/pick sources)
│   │   ├── registry_test.go           # Registry tests
│   │   ├── cdn.go                     # CDN mirror source implementation
│   │   ├── cdn_test.go                # CDN source tests
│   │   ├── selfhosted.go              # Self-hosted source implementation
│   │   └── selfhosted_test.go         # Self-hosted source tests
│   └── metrics/
│       ├── collector.go               # Real-time metrics collection (in-memory only for Phase 1)
│       └── collector_test.go          # Metrics collector tests
├── config/
│   ├── config.go                      # Config struct + Load/Save
│   └── config_test.go                 # Config tests
└── internal/
    └── testutil/
        └── testserver.go              # Shared test HTTP server for download/upload
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `cmd/cli/main.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/ayush18/networkbooster
go mod init github.com/ayush18/networkbooster
```

- [ ] **Step 2: Create Makefile**

Create `Makefile`:

```makefile
.PHONY: cli gui all test clean

cli:
	go build -o bin/networkbooster ./cmd/cli

gui:
	@echo "GUI build not yet implemented"

all: cli

test:
	go test ./... -v

clean:
	rm -rf bin/
```

- [ ] **Step 3: Create minimal CLI entry point**

Create `cmd/cli/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "networkbooster",
	Short: "Network bandwidth booster and speed optimizer",
	Long:  "NetworkBooster continuously saturates your bandwidth using parallel connections to maximize download and upload speeds.",
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the bandwidth booster",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting NetworkBooster...")
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the bandwidth booster",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Stopping NetworkBooster...")
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current booster status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("NetworkBooster is not running.")
		return nil
	},
}

func main() {
	rootCmd.AddCommand(startCmd, stopCmd, statusCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Install dependencies and verify build**

```bash
cd /Users/ayush18/networkbooster
go get github.com/spf13/cobra
go mod tidy
make cli
./bin/networkbooster --help
```

Expected: Help text showing `start`, `stop`, `status` subcommands.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum Makefile cmd/
git commit -m "feat: scaffold project with Go module, Makefile, and Cobra CLI"
```

---

### Task 2: Source Interface + Test Server Utility

**Files:**
- Create: `core/sources/source.go`
- Create: `internal/testutil/testserver.go`

- [ ] **Step 1: Define the Source and Server interfaces**

Create `core/sources/source.go`:

```go
package sources

import (
	"context"
	"io"
	"time"
)

type SourceType int

const (
	SourceTypeCDN SourceType = iota
	SourceTypeSelfHosted
	SourceTypeSpeedtest
	SourceTypeP2P
)

func (s SourceType) String() string {
	switch s {
	case SourceTypeCDN:
		return "CDN"
	case SourceTypeSelfHosted:
		return "SelfHosted"
	case SourceTypeSpeedtest:
		return "Speedtest"
	case SourceTypeP2P:
		return "P2P"
	default:
		return "Unknown"
	}
}

type Server struct {
	ID      string
	Name    string
	URL     string
	Latency time.Duration
}

type Source interface {
	Name() string
	Type() SourceType
	Discover() ([]Server, error)
	Download(ctx context.Context, server Server) (io.ReadCloser, error)
	Upload(ctx context.Context, server Server, r io.Reader) error
	Latency(server Server) (time.Duration, error)
}
```

- [ ] **Step 2: Create shared test HTTP server**

Create `internal/testutil/testserver.go`:

```go
package testutil

import (
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
)

// NewTestServer creates an HTTP server that serves random data on GET /download
// and accepts POST to /upload (discards body). Used by source tests and engine tests.
func NewTestServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		// Stream 10MB of random data
		io.CopyN(w, rand.Reader, 10*1024*1024)
	})

	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		// Read and discard all uploaded data
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return httptest.NewServer(mux)
}
```

- [ ] **Step 3: Verify it compiles**

```bash
cd /Users/ayush18/networkbooster
go build ./core/sources/...
go build ./internal/testutil/...
```

Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add core/sources/source.go internal/testutil/testserver.go
git commit -m "feat: define Source/Server interfaces and shared test HTTP server"
```

---

### Task 3: Source Registry

**Files:**
- Create: `core/sources/registry.go`
- Create: `core/sources/registry_test.go`

- [ ] **Step 1: Write failing tests for the registry**

Create `core/sources/registry_test.go`:

```go
package sources_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/ayush18/networkbooster/core/sources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSource implements Source for testing
type fakeSource struct {
	name    string
	srcType sources.SourceType
	servers []sources.Server
}

func (f *fakeSource) Name() string                { return f.name }
func (f *fakeSource) Type() sources.SourceType     { return f.srcType }
func (f *fakeSource) Discover() ([]sources.Server, error) {
	return f.servers, nil
}
func (f *fakeSource) Download(ctx context.Context, s sources.Server) (io.ReadCloser, error) {
	return nil, nil
}
func (f *fakeSource) Upload(ctx context.Context, s sources.Server, r io.Reader) error {
	return nil
}
func (f *fakeSource) Latency(s sources.Server) (time.Duration, error) {
	return 10 * time.Millisecond, nil
}

func TestRegistry_RegisterAndList(t *testing.T) {
	reg := sources.NewRegistry()
	src := &fakeSource{name: "test-cdn", srcType: sources.SourceTypeCDN}

	reg.Register(src)

	all := reg.Sources()
	assert.Len(t, all, 1)
	assert.Equal(t, "test-cdn", all[0].Name())
}

func TestRegistry_DiscoverAll(t *testing.T) {
	reg := sources.NewRegistry()
	src := &fakeSource{
		name:    "test-cdn",
		srcType: sources.SourceTypeCDN,
		servers: []sources.Server{
			{ID: "s1", Name: "Server 1", URL: "http://example.com/download"},
			{ID: "s2", Name: "Server 2", URL: "http://example2.com/download"},
		},
	}
	reg.Register(src)

	servers, err := reg.DiscoverAll()
	require.NoError(t, err)
	assert.Len(t, servers, 2)
}

func TestRegistry_DiscoverAll_MultipleSources(t *testing.T) {
	reg := sources.NewRegistry()
	cdn := &fakeSource{
		name:    "cdn",
		srcType: sources.SourceTypeCDN,
		servers: []sources.Server{{ID: "c1", Name: "CDN 1", URL: "http://cdn.com"}},
	}
	self := &fakeSource{
		name:    "self",
		srcType: sources.SourceTypeSelfHosted,
		servers: []sources.Server{{ID: "s1", Name: "Self 1", URL: "http://self.com"}},
	}
	reg.Register(cdn)
	reg.Register(self)

	servers, err := reg.DiscoverAll()
	require.NoError(t, err)
	assert.Len(t, servers, 2)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/ayush18/networkbooster
go test ./core/sources/... -v
```

Expected: FAIL — `NewRegistry` not defined.

- [ ] **Step 3: Implement the registry**

Create `core/sources/registry.go`:

```go
package sources

import "sync"

// DiscoveredServer pairs a server with the source it came from.
type DiscoveredServer struct {
	Server Server
	Source Source
}

// Registry holds registered sources and discovers servers from all of them.
type Registry struct {
	mu      sync.RWMutex
	sources []Source
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(src Source) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources = append(r.sources, src)
}

func (r *Registry) Sources() []Source {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Source, len(r.sources))
	copy(out, r.sources)
	return out
}

// DiscoverAll runs Discover on every registered source and returns all servers.
func (r *Registry) DiscoverAll() ([]DiscoveredServer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []DiscoveredServer
	for _, src := range r.sources {
		servers, err := src.Discover()
		if err != nil {
			continue // skip failing sources, don't abort
		}
		for _, srv := range servers {
			all = append(all, DiscoveredServer{Server: srv, Source: src})
		}
	}
	return all, nil
}
```

- [ ] **Step 4: Update tests to use DiscoveredServer**

The registry returns `[]DiscoveredServer` not `[]Server`. Update the test assertions:

In `registry_test.go`, change the two `DiscoverAll` tests:

```go
func TestRegistry_DiscoverAll(t *testing.T) {
	reg := sources.NewRegistry()
	src := &fakeSource{
		name:    "test-cdn",
		srcType: sources.SourceTypeCDN,
		servers: []sources.Server{
			{ID: "s1", Name: "Server 1", URL: "http://example.com/download"},
			{ID: "s2", Name: "Server 2", URL: "http://example2.com/download"},
		},
	}
	reg.Register(src)

	discovered, err := reg.DiscoverAll()
	require.NoError(t, err)
	assert.Len(t, discovered, 2)
	assert.Equal(t, "Server 1", discovered[0].Server.Name)
}

func TestRegistry_DiscoverAll_MultipleSources(t *testing.T) {
	reg := sources.NewRegistry()
	cdn := &fakeSource{
		name:    "cdn",
		srcType: sources.SourceTypeCDN,
		servers: []sources.Server{{ID: "c1", Name: "CDN 1", URL: "http://cdn.com"}},
	}
	self := &fakeSource{
		name:    "self",
		srcType: sources.SourceTypeSelfHosted,
		servers: []sources.Server{{ID: "s1", Name: "Self 1", URL: "http://self.com"}},
	}
	reg.Register(cdn)
	reg.Register(self)

	discovered, err := reg.DiscoverAll()
	require.NoError(t, err)
	assert.Len(t, discovered, 2)
}
```

- [ ] **Step 5: Install testify and run tests**

```bash
cd /Users/ayush18/networkbooster
go get github.com/stretchr/testify
go mod tidy
go test ./core/sources/... -v
```

Expected: All 3 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add core/sources/registry.go core/sources/registry_test.go go.mod go.sum
git commit -m "feat: add source registry with discover-all support"
```

---

### Task 4: CDN Source

**Files:**
- Create: `core/sources/cdn.go`
- Create: `core/sources/cdn_test.go`

- [ ] **Step 1: Write failing tests for CDN source**

Create `core/sources/cdn_test.go`:

```go
package sources_test

import (
	"context"
	"io"
	"testing"

	"github.com/ayush18/networkbooster/core/sources"
	"github.com/ayush18/networkbooster/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCDNSource_Name(t *testing.T) {
	cdn := sources.NewCDNSource()
	assert.Equal(t, "CDN Mirrors", cdn.Name())
	assert.Equal(t, sources.SourceTypeCDN, cdn.Type())
}

func TestCDNSource_Discover(t *testing.T) {
	cdn := sources.NewCDNSource()
	servers, err := cdn.Discover()
	require.NoError(t, err)
	assert.Greater(t, len(servers), 0, "should have at least one CDN server")
	for _, s := range servers {
		assert.NotEmpty(t, s.ID)
		assert.NotEmpty(t, s.URL)
	}
}

func TestCDNSource_Download(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	cdn := sources.NewCDNSource()
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL + "/download"}

	rc, err := cdn.Download(context.Background(), server)
	require.NoError(t, err)
	defer rc.Close()

	n, err := io.Copy(io.Discard, rc)
	require.NoError(t, err)
	assert.Greater(t, n, int64(0), "should have downloaded some data")
}

func TestCDNSource_Upload_NotSupported(t *testing.T) {
	cdn := sources.NewCDNSource()
	server := sources.Server{ID: "test", Name: "Test", URL: "http://example.com"}

	err := cdn.Upload(context.Background(), server, nil)
	assert.Error(t, err, "CDN should not support upload")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./core/sources/... -v -run TestCDN
```

Expected: FAIL — `NewCDNSource` not defined.

- [ ] **Step 3: Implement CDN source**

Create `core/sources/cdn.go`:

```go
package sources

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// defaultCDNServers is a curated list of public test file endpoints.
var defaultCDNServers = []Server{
	{ID: "hetzner-de", Name: "Hetzner Germany", URL: "https://speed.hetzner.de/10GB.bin"},
	{ID: "ovh-fr", Name: "OVH France", URL: "http://proof.ovh.net/files/10Gb.dat"},
	{ID: "tele2-se", Name: "Tele2 Sweden", URL: "https://ash-speed.hetzner.com/10GB.bin"},
}

type CDNSource struct {
	client *http.Client
}

func NewCDNSource() *CDNSource {
	return &CDNSource{
		client: &http.Client{
			Timeout: 0, // no timeout — we stream continuously
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 64,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (c *CDNSource) Name() string        { return "CDN Mirrors" }
func (c *CDNSource) Type() SourceType    { return SourceTypeCDN }

func (c *CDNSource) Discover() ([]Server, error) {
	return defaultCDNServers, nil
}

func (c *CDNSource) Download(ctx context.Context, server Server) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, errors.New("unexpected status: " + resp.Status)
	}
	return resp.Body, nil
}

func (c *CDNSource) Upload(ctx context.Context, server Server, r io.Reader) error {
	return errors.New("CDN source does not support upload")
}

func (c *CDNSource) Latency(server Server) (time.Duration, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, server.URL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return time.Since(start), nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./core/sources/... -v -run TestCDN
```

Expected: All 4 CDN tests PASS.

- [ ] **Step 5: Commit**

```bash
git add core/sources/cdn.go core/sources/cdn_test.go
git commit -m "feat: add CDN mirror source with download and latency support"
```

---

### Task 5: Self-Hosted Source

**Files:**
- Create: `core/sources/selfhosted.go`
- Create: `core/sources/selfhosted_test.go`

- [ ] **Step 1: Write failing tests for self-hosted source**

Create `core/sources/selfhosted_test.go`:

```go
package sources_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/ayush18/networkbooster/core/sources"
	"github.com/ayush18/networkbooster/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelfHostedSource_Name(t *testing.T) {
	sh := sources.NewSelfHostedSource("http://myserver.com")
	assert.Equal(t, "Self-Hosted", sh.Name())
	assert.Equal(t, sources.SourceTypeSelfHosted, sh.Type())
}

func TestSelfHostedSource_Discover(t *testing.T) {
	sh := sources.NewSelfHostedSource("http://myserver.com")
	servers, err := sh.Discover()
	require.NoError(t, err)
	assert.Len(t, servers, 1)
	assert.Equal(t, "http://myserver.com", servers[0].URL)
}

func TestSelfHostedSource_Download(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	sh := sources.NewSelfHostedSource(ts.URL)
	server := sources.Server{ID: "self", Name: "Self", URL: ts.URL + "/download"}

	rc, err := sh.Download(context.Background(), server)
	require.NoError(t, err)
	defer rc.Close()

	n, err := io.Copy(io.Discard, rc)
	require.NoError(t, err)
	assert.Greater(t, n, int64(0))
}

func TestSelfHostedSource_Upload(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	sh := sources.NewSelfHostedSource(ts.URL)
	server := sources.Server{ID: "self", Name: "Self", URL: ts.URL + "/upload"}

	data := bytes.NewReader(make([]byte, 1024))
	err := sh.Upload(context.Background(), server, data)
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./core/sources/... -v -run TestSelfHosted
```

Expected: FAIL — `NewSelfHostedSource` not defined.

- [ ] **Step 3: Implement self-hosted source**

Create `core/sources/selfhosted.go`:

```go
package sources

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

type SelfHostedSource struct {
	baseURL string
	client  *http.Client
}

func NewSelfHostedSource(baseURL string) *SelfHostedSource {
	return &SelfHostedSource{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 64,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (s *SelfHostedSource) Name() string     { return "Self-Hosted" }
func (s *SelfHostedSource) Type() SourceType { return SourceTypeSelfHosted }

func (s *SelfHostedSource) Discover() ([]Server, error) {
	return []Server{
		{ID: "selfhosted-0", Name: "Self-Hosted Server", URL: s.baseURL},
	}, nil
}

func (s *SelfHostedSource) Download(ctx context.Context, server Server) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
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

func (s *SelfHostedSource) Upload(ctx context.Context, server Server, r io.Reader) error {
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

func (s *SelfHostedSource) Latency(server Server) (time.Duration, error) {
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

- [ ] **Step 4: Run tests**

```bash
go test ./core/sources/... -v -run TestSelfHosted
```

Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add core/sources/selfhosted.go core/sources/selfhosted_test.go
git commit -m "feat: add self-hosted source with download and upload support"
```

---

### Task 6: Metrics Collector

**Files:**
- Create: `core/metrics/collector.go`
- Create: `core/metrics/collector_test.go`

- [ ] **Step 1: Write failing tests for the metrics collector**

Create `core/metrics/collector_test.go`:

```go
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

	// Record 1MB over ~100ms to get a speed reading
	c.RecordBytes(metrics.DirectionDownload, 1*1024*1024)
	time.Sleep(150 * time.Millisecond)

	snap := c.Snapshot()
	// Speed should be > 0 since we recorded bytes
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./core/metrics/... -v
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement the metrics collector**

Create `core/metrics/collector.go`:

```go
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
	DownloadMbps      float64
	UploadMbps        float64
	TotalDownloadBytes int64
	TotalUploadBytes   int64
	ActiveConnections  int
	Elapsed            time.Duration
}

type Collector struct {
	mu sync.RWMutex

	totalDownload atomic.Int64
	totalUpload   atomic.Int64

	// For speed calculation: bytes in the current window
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
		elapsed = 0.001 // avoid division by zero
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
```

- [ ] **Step 4: Run tests**

```bash
go test ./core/metrics/... -v
```

Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add core/metrics/collector.go core/metrics/collector_test.go
git commit -m "feat: add real-time metrics collector with speed calculation"
```

---

### Task 7: Download Worker

**Files:**
- Create: `core/engine/worker.go`
- Create: `core/engine/worker_test.go`

- [ ] **Step 1: Write failing tests for the download worker**

Create `core/engine/worker_test.go`:

```go
package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/ayush18/networkbooster/core/engine"
	"github.com/ayush18/networkbooster/core/metrics"
	"github.com/ayush18/networkbooster/core/sources"
	"github.com/ayush18/networkbooster/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestDownloadWorker_TransfersData(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	collector := metrics.NewCollector()
	cdn := sources.NewCDNSource()
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL + "/download"}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	w := engine.NewWorker("w-1", cdn, server, collector)
	w.RunDownload(ctx)

	snap := collector.Snapshot()
	assert.Greater(t, snap.TotalDownloadBytes, int64(0), "worker should have downloaded data")
}

func TestUploadWorker_TransfersData(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	collector := metrics.NewCollector()
	sh := sources.NewSelfHostedSource(ts.URL)
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL + "/upload"}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	w := engine.NewWorker("w-1", sh, server, collector)
	w.RunUpload(ctx)

	snap := collector.Snapshot()
	assert.Greater(t, snap.TotalUploadBytes, int64(0), "worker should have uploaded data")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./core/engine/... -v
```

Expected: FAIL — `engine` package doesn't exist.

- [ ] **Step 3: Implement the worker**

Create `core/engine/worker.go`:

```go
package engine

import (
	"context"
	"io"

	"github.com/ayush18/networkbooster/core/metrics"
	"github.com/ayush18/networkbooster/core/sources"
)

const (
	// readBufSize is the buffer size for reading download data.
	readBufSize = 32 * 1024 // 32KB
	// uploadChunkSize is the size of each upload chunk.
	uploadChunkSize = 1024 * 1024 // 1MB
)

// Worker handles a single download or upload connection loop.
type Worker struct {
	ID        string
	source    sources.Source
	server    sources.Server
	collector *metrics.Collector
}

func NewWorker(id string, source sources.Source, server sources.Server, collector *metrics.Collector) *Worker {
	return &Worker{
		ID:        id,
		source:    source,
		server:    server,
		collector: collector,
	}
}

// RunDownload continuously downloads data from the server until ctx is cancelled.
// It reconnects automatically if a download stream ends.
func (w *Worker) RunDownload(ctx context.Context) {
	w.collector.AddConnection(w.ID)
	defer w.collector.RemoveConnection(w.ID)

	buf := make([]byte, readBufSize)
	for {
		if ctx.Err() != nil {
			return
		}
		rc, err := w.source.Download(ctx, w.server)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue // retry
		}
		w.readLoop(ctx, rc, buf)
		rc.Close()
	}
}

func (w *Worker) readLoop(ctx context.Context, rc io.ReadCloser, buf []byte) {
	for {
		if ctx.Err() != nil {
			return
		}
		n, err := rc.Read(buf)
		if n > 0 {
			w.collector.RecordBytes(metrics.DirectionDownload, int64(n))
		}
		if err != nil {
			return
		}
	}
}

// RunUpload continuously uploads data to the server until ctx is cancelled.
// It sends chunks of zero-filled data in a loop.
func (w *Worker) RunUpload(ctx context.Context) {
	w.collector.AddConnection(w.ID)
	defer w.collector.RemoveConnection(w.ID)

	for {
		if ctx.Err() != nil {
			return
		}
		pr, pw := io.Pipe()

		// Write data in background
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer pw.Close()
			chunk := make([]byte, uploadChunkSize)
			for {
				if ctx.Err() != nil {
					return
				}
				n, err := pw.Write(chunk)
				if err != nil {
					return
				}
				w.collector.RecordBytes(metrics.DirectionUpload, int64(n))
			}
		}()

		err := w.source.Upload(ctx, w.server, pr)
		pr.Close()
		<-done

		if err != nil && ctx.Err() != nil {
			return
		}
		// retry on error
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./core/engine/... -v -timeout 10s
```

Expected: Both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add core/engine/worker.go core/engine/worker_test.go
git commit -m "feat: add download/upload worker with continuous streaming loop"
```

---

### Task 8: Engine (Orchestrator)

**Files:**
- Create: `core/engine/engine.go`
- Create: `core/engine/engine_test.go`

- [ ] **Step 1: Write failing tests for the engine**

Create `core/engine/engine_test.go`:

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

func TestEngine_StartDownload(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL + "/download")
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{
		Connections: 4,
	})

	err := eng.Start(engine.ModeDownload)
	require.NoError(t, err)

	// Let it run briefly
	time.Sleep(300 * time.Millisecond)

	status := eng.Status()
	assert.True(t, status.Running)
	assert.Greater(t, status.Snapshot.TotalDownloadBytes, int64(0))
	assert.Greater(t, status.Snapshot.ActiveConnections, 0)

	err = eng.Stop()
	require.NoError(t, err)

	status = eng.Status()
	assert.False(t, status.Running)
}

func TestEngine_StartUpload(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL + "/upload")
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{
		Connections: 2,
	})

	err := eng.Start(engine.ModeUpload)
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	status := eng.Status()
	assert.True(t, status.Running)
	assert.Greater(t, status.Snapshot.TotalUploadBytes, int64(0))

	err = eng.Stop()
	require.NoError(t, err)
}

func TestEngine_StartBidirectional(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	// Self-hosted source needs base URL; we use the download endpoint for Discover
	sh := sources.NewSelfHostedSource(ts.URL)
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{
		Connections: 2,
	})

	err := eng.Start(engine.ModeBidirectional)
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	status := eng.Status()
	assert.True(t, status.Running)
	assert.Greater(t, status.Snapshot.TotalDownloadBytes, int64(0))
	assert.Greater(t, status.Snapshot.TotalUploadBytes, int64(0))

	eng.Stop()
}

func TestEngine_DoubleStartReturnsError(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL + "/download")
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{Connections: 1})
	eng.Start(engine.ModeDownload)
	defer eng.Stop()

	err := eng.Start(engine.ModeDownload)
	assert.Error(t, err, "starting an already running engine should error")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./core/engine/... -v -run TestEngine -timeout 10s
```

Expected: FAIL — `engine.New` not defined.

- [ ] **Step 3: Implement the engine**

Create `core/engine/engine.go`:

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
	Connections int // number of parallel connections (default 8)
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

	// Distribute connections across discovered servers round-robin
	for i := 0; i < e.opts.Connections; i++ {
		ds := discovered[i%len(discovered)]
		workerID := fmt.Sprintf("worker-%d", i)

		switch mode {
		case ModeDownload:
			w := NewWorker(workerID, ds.Source, e.downloadServer(ds), e.collector)
			e.wg.Add(1)
			go func() {
				defer e.wg.Done()
				w.RunDownload(ctx)
			}()
		case ModeUpload:
			w := NewWorker(workerID, ds.Source, e.uploadServer(ds), e.collector)
			e.wg.Add(1)
			go func() {
				defer e.wg.Done()
				w.RunUpload(ctx)
			}()
		case ModeBidirectional:
			// Half download, half upload (at least 1 of each)
			if i < (e.opts.Connections+1)/2 {
				w := NewWorker(workerID+"-dl", ds.Source, e.downloadServer(ds), e.collector)
				e.wg.Add(1)
				go func() {
					defer e.wg.Done()
					w.RunDownload(ctx)
				}()
			} else {
				w := NewWorker(workerID+"-ul", ds.Source, e.uploadServer(ds), e.collector)
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

// downloadServer returns a Server with URL pointing to download endpoint.
// For self-hosted sources, if the URL is a base URL, append /download.
func (e *Engine) downloadServer(ds sources.DiscoveredServer) sources.Server {
	srv := ds.Server
	if ds.Source.Type() == sources.SourceTypeSelfHosted {
		srv.URL = ds.Server.URL + "/download"
	}
	return srv
}

func (e *Engine) uploadServer(ds sources.DiscoveredServer) sources.Server {
	srv := ds.Server
	if ds.Source.Type() == sources.SourceTypeSelfHosted {
		srv.URL = ds.Server.URL + "/upload"
	}
	return srv
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
```

- [ ] **Step 4: Run tests**

```bash
go test ./core/engine/... -v -run TestEngine -timeout 15s
```

Expected: All 4 engine tests PASS.

- [ ] **Step 5: Commit**

```bash
git add core/engine/engine.go core/engine/engine_test.go
git commit -m "feat: add bandwidth engine orchestrator with parallel workers"
```

---

### Task 9: Config System

**Files:**
- Create: `config/config.go`
- Create: `config/config_test.go`

- [ ] **Step 1: Write failing tests for config**

Create `config/config_test.go`:

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ayush18/networkbooster/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := config.Default()
	assert.Equal(t, 8, cfg.Connections)
	assert.Equal(t, "medium", cfg.Profile)
	assert.Equal(t, "download", cfg.Mode)
}

func TestConfig_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := config.Default()
	cfg.Connections = 16
	cfg.SelfHostedURL = "http://myserver.com"

	err := config.Save(cfg, path)
	require.NoError(t, err)

	loaded, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, 16, loaded.Connections)
	assert.Equal(t, "http://myserver.com", loaded.SelfHostedURL)
}

func TestConfig_LoadMissing_ReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, config.Default(), cfg)
}

func TestConfig_LoadFromEnvPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := config.Default()
	cfg.Connections = 32
	config.Save(cfg, path)

	t.Setenv("NETWORKBOOSTER_CONFIG", path)

	loaded, err := config.LoadDefault()
	require.NoError(t, err)
	assert.Equal(t, 32, loaded.Connections)

	os.Unsetenv("NETWORKBOOSTER_CONFIG")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./config/... -v
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement config**

Create `config/config.go`:

```go
package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Mode          string `yaml:"mode"`           // download, upload, bidirectional
	Profile       string `yaml:"profile"`         // light, medium, full, custom
	Connections   int    `yaml:"connections"`
	SelfHostedURL string `yaml:"self_hosted_url,omitempty"`
}

func Default() Config {
	return Config{
		Mode:        "download",
		Profile:     "medium",
		Connections: 8,
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, err
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(cfg Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadDefault loads config from NETWORKBOOSTER_CONFIG env var,
// or from ~/.networkbooster/config.yaml if not set.
func LoadDefault() (Config, error) {
	path := os.Getenv("NETWORKBOOSTER_CONFIG")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Default(), nil
		}
		path = filepath.Join(home, ".networkbooster", "config.yaml")
	}
	return Load(path)
}
```

- [ ] **Step 4: Install yaml dep and run tests**

```bash
cd /Users/ayush18/networkbooster
go get gopkg.in/yaml.v3
go mod tidy
go test ./config/... -v
```

Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/config_test.go go.mod go.sum
git commit -m "feat: add YAML config system with defaults and env var support"
```

---

### Task 10: Wire CLI to Engine

**Files:**
- Modify: `cmd/cli/main.go`

- [ ] **Step 1: Update the CLI to wire everything together**

Replace `cmd/cli/main.go` with:

```go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ayush18/networkbooster/config"
	"github.com/ayush18/networkbooster/core/engine"
	"github.com/ayush18/networkbooster/core/sources"
	"github.com/spf13/cobra"
)

var (
	profileFlag string
	modeFlag    string
	connsFlag   int
	selfHosted  string
)

var rootCmd = &cobra.Command{
	Use:   "networkbooster",
	Short: "Network bandwidth booster and speed optimizer",
	Long:  "NetworkBooster continuously saturates your bandwidth using parallel connections to maximize download and upload speeds.",
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the bandwidth booster",
	RunE:  runStart,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current booster status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("NetworkBooster is not running. Use 'networkbooster start' to begin.")
		return nil
	},
}

func init() {
	startCmd.Flags().StringVarP(&profileFlag, "profile", "p", "", "Profile: light, medium, full (overrides config)")
	startCmd.Flags().StringVarP(&modeFlag, "mode", "m", "", "Mode: download, upload, bidirectional (overrides config)")
	startCmd.Flags().IntVarP(&connsFlag, "connections", "c", 0, "Number of parallel connections (overrides config)")
	startCmd.Flags().StringVar(&selfHosted, "self-hosted", "", "Self-hosted server URL")
}

func runStart(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadDefault()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply flag overrides
	if profileFlag != "" {
		cfg.Profile = profileFlag
	}
	if modeFlag != "" {
		cfg.Mode = modeFlag
	}
	if connsFlag > 0 {
		cfg.Connections = connsFlag
	}
	if selfHosted != "" {
		cfg.SelfHostedURL = selfHosted
	}

	// Apply profile presets
	switch cfg.Profile {
	case "light":
		if connsFlag == 0 {
			cfg.Connections = 4
		}
	case "full":
		if connsFlag == 0 {
			cfg.Connections = 64
		}
	case "medium":
		if connsFlag == 0 {
			cfg.Connections = 16
		}
	}

	// Parse mode
	var mode engine.Mode
	switch strings.ToLower(cfg.Mode) {
	case "upload":
		mode = engine.ModeUpload
	case "bidirectional", "both":
		mode = engine.ModeBidirectional
	default:
		mode = engine.ModeDownload
	}

	// Build source registry
	reg := sources.NewRegistry()
	reg.Register(sources.NewCDNSource())
	if cfg.SelfHostedURL != "" {
		reg.Register(sources.NewSelfHostedSource(cfg.SelfHostedURL))
	}

	// Create and start engine
	eng := engine.New(reg, engine.Options{
		Connections: cfg.Connections,
	})

	fmt.Printf("NetworkBooster starting...\n")
	fmt.Printf("  Mode: %s | Connections: %d | Profile: %s\n", cfg.Mode, cfg.Connections, cfg.Profile)

	if err := eng.Start(mode); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}

	fmt.Println("  Running! Press Ctrl+C to stop.\n")

	// Handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Print live stats until interrupted
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			fmt.Println("\nStopping...")
			eng.Stop()
			status := eng.Status()
			fmt.Printf("\nSession summary:\n")
			fmt.Printf("  Downloaded: %.2f MB\n", float64(status.Snapshot.TotalDownloadBytes)/(1024*1024))
			fmt.Printf("  Uploaded:   %.2f MB\n", float64(status.Snapshot.TotalUploadBytes)/(1024*1024))
			fmt.Printf("  Duration:   %s\n", status.Snapshot.Elapsed.Round(time.Second))
			return nil
		case <-ticker.C:
			status := eng.Status()
			s := status.Snapshot
			fmt.Printf("\r  ↓ %.1f Mbps  ↑ %.1f Mbps  | %d conns | ↓ %.1f MB  ↑ %.1f MB",
				s.DownloadMbps,
				s.UploadMbps,
				s.ActiveConnections,
				float64(s.TotalDownloadBytes)/(1024*1024),
				float64(s.TotalUploadBytes)/(1024*1024),
			)
		}
	}
}

func main() {
	rootCmd.AddCommand(startCmd, statusCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Build and verify**

```bash
cd /Users/ayush18/networkbooster
go mod tidy
make cli
./bin/networkbooster --help
./bin/networkbooster start --help
```

Expected: Help output shows `start` with `--profile`, `--mode`, `--connections`, `--self-hosted` flags.

- [ ] **Step 3: Run all tests to verify nothing is broken**

```bash
go test ./... -v -timeout 30s
```

Expected: All tests across all packages PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/main.go go.mod go.sum
git commit -m "feat: wire CLI start command to engine with live speed display"
```

---

### Task 11: Run Full Integration Test

This is a manual verification step to make sure the tool works end-to-end.

- [ ] **Step 1: Build the CLI**

```bash
make cli
```

- [ ] **Step 2: Run a quick download test**

```bash
./bin/networkbooster start --mode download --connections 4 --profile light
# Let it run for ~5 seconds, then press Ctrl+C
```

Expected: Live speed updates printed every second, showing non-zero download speed. Session summary printed on exit.

- [ ] **Step 3: Run with self-hosted server (optional verification)**

```bash
# In one terminal, run a simple test server:
go run internal/testutil/cmd/main.go  # (skip this — just verify the CDN test above works)
```

- [ ] **Step 4: Run all tests one final time**

```bash
go test ./... -v -timeout 30s
```

Expected: All tests PASS.

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "Phase 1 complete: core engine, CDN/self-hosted sources, CLI with live stats"
```

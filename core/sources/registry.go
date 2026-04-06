package sources

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type DiscoveredServer struct {
	Server Server
	Source Source
}

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

func (r *Registry) DiscoverAll() ([]DiscoveredServer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []DiscoveredServer
	for _, src := range r.sources {
		servers, err := src.Discover()
		if err != nil {
			continue
		}
		for _, srv := range servers {
			all = append(all, DiscoveredServer{Server: srv, Source: src})
		}
	}

	if len(all) == 0 {
		return all, nil
	}

	// Probe latency on all servers concurrently, keep only reachable ones
	type probeResult struct {
		idx     int
		latency time.Duration
		ok      bool
	}

	results := make(chan probeResult, len(all))
	for i, ds := range all {
		go func(idx int, ds DiscoveredServer) {
			lat, err := ds.Source.Latency(ds.Server)
			results <- probeResult{idx: idx, latency: lat, ok: err == nil}
		}(i, ds)
	}

	var reachable []DiscoveredServer
	for range all {
		r := <-results
		if r.ok {
			srv := all[r.idx]
			srv.Server.Latency = r.latency
			reachable = append(reachable, srv)
		}
	}

	if len(reachable) == 0 {
		// If nothing responded to latency probe, return all and hope for the best
		fmt.Println("  Warning: no servers responded to latency probe, using all discovered")
		return all, nil
	}

	// Sort by latency, keep top 5 fastest
	sort.Slice(reachable, func(i, j int) bool {
		return reachable[i].Server.Latency < reachable[j].Server.Latency
	})

	maxServers := 5
	if len(reachable) < maxServers {
		maxServers = len(reachable)
	}
	fastest := reachable[:maxServers]

	fmt.Printf("  Probed %d servers, using top %d (fastest: %s at %dms)\n",
		len(all), len(fastest),
		fastest[0].Server.Name,
		fastest[0].Server.Latency.Milliseconds())

	return fastest, nil
}

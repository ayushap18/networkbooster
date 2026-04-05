package sources

import "sync"

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
	return all, nil
}

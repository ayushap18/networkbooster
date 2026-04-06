package sources

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"time"
)

const defaultSpeedtestServerListURL = "https://www.speedtest.net/speedtest-servers-static.php"

var bundledSpeedtestServers = []Server{
	// Hetzner speed test servers — reliable, support range requests, large files
	{ID: "st-hetzner-de", Name: "Hetzner Germany", URL: "https://speed.hetzner.de/10GB.bin"},
	{ID: "st-hetzner-us", Name: "Hetzner US (Ashburn)", URL: "https://ash-speed.hetzner.com/10GB.bin"},
	{ID: "st-hetzner-fi", Name: "Hetzner Finland", URL: "https://hel1-speed.hetzner.com/10GB.bin"},
	// Scaleway test files
	{ID: "st-scaleway-fr", Name: "Scaleway France", URL: "https://multi.fr.apt.scaleway.com/linux/debian/ls-lR.gz"},
	// Linode/Akamai speed test servers
	{ID: "st-linode-us-east", Name: "Linode US East", URL: "https://speedtest.newark.linode.com/100MB-newark.bin"},
	{ID: "st-linode-us-west", Name: "Linode US West", URL: "https://speedtest.fremont.linode.com/100MB-fremont.bin"},
	{ID: "st-linode-eu", Name: "Linode EU (London)", URL: "https://speedtest.london.linode.com/100MB-london.bin"},
	{ID: "st-linode-ap", Name: "Linode AP (Singapore)", URL: "https://speedtest.singapore.linode.com/100MB-singapore.bin"},
	{ID: "st-linode-jp", Name: "Linode JP (Tokyo)", URL: "https://speedtest.tokyo2.linode.com/100MB-tokyo2.bin"},
	// Vultr test files
	{ID: "st-vultr-us", Name: "Vultr US (NJ)", URL: "https://nj-us-ping.vultr.com/vultr.com.100MB.bin"},
	{ID: "st-vultr-eu", Name: "Vultr EU (Amsterdam)", URL: "https://ams-nl-ping.vultr.com/vultr.com.100MB.bin"},
	{ID: "st-vultr-ap", Name: "Vultr AP (Singapore)", URL: "https://sgp-ping.vultr.com/vultr.com.100MB.bin"},
	{ID: "st-vultr-au", Name: "Vultr AU (Sydney)", URL: "https://syd-au-ping.vultr.com/vultr.com.100MB.bin"},
	{ID: "st-vultr-jp", Name: "Vultr JP (Tokyo)", URL: "https://hnd-jp-ping.vultr.com/vultr.com.100MB.bin"},
	// OVH test files
	{ID: "st-ovh-fr", Name: "OVH France", URL: "https://proof.ovh.net/files/100Mb.dat"},
	{ID: "st-ovh-ca", Name: "OVH Canada", URL: "https://proof.ovh.ca/files/100Mb.dat"},
	// Telia carrier test
	{ID: "st-telia-se", Name: "Telia Sweden", URL: "http://speedtest.tele2.net/100MB.zip"},
	// Clouvider looking glass
	{ID: "st-clouvider-uk", Name: "Clouvider UK", URL: "https://lon.speedtest.clouvider.net/backend/garbage/100MB.bin"},
	{ID: "st-clouvider-us", Name: "Clouvider US", URL: "https://dal.speedtest.clouvider.net/backend/garbage/100MB.bin"},
	{ID: "st-clouvider-de", Name: "Clouvider DE", URL: "https://fra.speedtest.clouvider.net/backend/garbage/100MB.bin"},
}

// xmlSettings mirrors the Ookla speedtest-servers XML structure.
type xmlSettings struct {
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

// SpeedtestSource implements Source for Ookla-compatible speedtest servers.
type SpeedtestSource struct {
	serverListURL string
	client        *http.Client
}

// NewSpeedtestSource returns a SpeedtestSource that fetches the official Ookla
// server list on Discover().
func NewSpeedtestSource() *SpeedtestSource {
	return NewSpeedtestSourceWithURL(defaultSpeedtestServerListURL)
}

// NewSpeedtestSourceWithURL returns a SpeedtestSource that fetches the server
// list from the provided URL — useful for testing with a mock HTTP server.
func NewSpeedtestSourceWithURL(serverListURL string) *SpeedtestSource {
	return &SpeedtestSource{
		serverListURL: serverListURL,
		client: &http.Client{
			Timeout: 0, // no timeout — workers manage their own lifecycle via context
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 64,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (s *SpeedtestSource) Name() string     { return "Speedtest" }
func (s *SpeedtestSource) Type() SourceType { return SourceTypeSpeedtest }

// BundledServers returns the hardcoded list of well-known speedtest servers.
func (s *SpeedtestSource) BundledServers() []Server {
	out := make([]Server, len(bundledSpeedtestServers))
	copy(out, bundledSpeedtestServers)
	return out
}

// Discover attempts a live XML fetch from the Ookla server list. If that
// fails for any reason it falls back to the bundled servers.
func (s *SpeedtestSource) Discover() ([]Server, error) {
	servers, err := s.fetchLiveServers()
	if err != nil || len(servers) == 0 {
		return s.BundledServers(), nil
	}
	return servers, nil
}

// fetchLiveServers fetches and parses the Ookla XML server list.
func (s *SpeedtestSource) fetchLiveServers() ([]Server, error) {
	resp, err := s.client.Get(s.serverListURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("server list fetch failed: " + resp.Status)
	}

	var settings xmlSettings
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&settings); err != nil {
		return nil, err
	}

	servers := make([]Server, 0, len(settings.Servers))
	for _, xs := range settings.Servers {
		if xs.URL == "" || xs.ID == "" {
			continue
		}
		name := xs.Name
		if xs.Sponsor != "" {
			name = xs.Sponsor + " – " + xs.Name + ", " + xs.Country
		}
		servers = append(servers, Server{
			ID:   "ookla-" + xs.ID,
			Name: name,
			URL:  xs.URL,
		})
	}
	return servers, nil
}

// Download performs an HTTP GET to server.URL and returns the response body.
func (s *SpeedtestSource) Download(ctx context.Context, server Server) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", "bytes=0-104857599") // 100MB chunk
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

// Upload performs an HTTP POST to server.URL with r as the request body.
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

// Latency measures round-trip time to server.URL using a HEAD request.
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

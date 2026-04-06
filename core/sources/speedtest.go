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
	{ID: "st-nyc-1", Name: "New York, US (Speedtest)", URL: "http://speedtest.net/api/js/servers?engine=js&search=new+york&limit=1"},
	{ID: "st-lax-1", Name: "Los Angeles, US (Lumen)", URL: "http://lax.speedtest.net/speedtest/upload.php"},
	{ID: "st-chi-1", Name: "Chicago, US (Comcast)", URL: "http://ord.speedtest.net/speedtest/upload.php"},
	{ID: "st-lon-1", Name: "London, UK (Vodafone)", URL: "http://lon.speedtest.net/speedtest/upload.php"},
	{ID: "st-fra-1", Name: "Frankfurt, DE (Deutsche Telekom)", URL: "http://fra.speedtest.net/speedtest/upload.php"},
	{ID: "st-ams-1", Name: "Amsterdam, NL (KPN)", URL: "http://ams.speedtest.net/speedtest/upload.php"},
	{ID: "st-par-1", Name: "Paris, FR (Orange)", URL: "http://par.speedtest.net/speedtest/upload.php"},
	{ID: "st-sin-1", Name: "Singapore (Singtel)", URL: "http://sin.speedtest.net/speedtest/upload.php"},
	{ID: "st-tok-1", Name: "Tokyo, JP (NTT)", URL: "http://tok.speedtest.net/speedtest/upload.php"},
	{ID: "st-syd-1", Name: "Sydney, AU (Telstra)", URL: "http://syd.speedtest.net/speedtest/upload.php"},
	{ID: "st-sao-1", Name: "São Paulo, BR (Claro)", URL: "http://sao.speedtest.net/speedtest/upload.php"},
	{ID: "st-dxb-1", Name: "Dubai, AE (Etisalat)", URL: "http://dxb.speedtest.net/speedtest/upload.php"},
	{ID: "st-hkg-1", Name: "Hong Kong (PCCW)", URL: "http://hkg.speedtest.net/speedtest/upload.php"},
	{ID: "st-mum-1", Name: "Mumbai, IN (Jio)", URL: "http://mum.speedtest.net/speedtest/upload.php"},
	{ID: "st-sea-1", Name: "Seattle, US (CenturyLink)", URL: "http://sea.speedtest.net/speedtest/upload.php"},
	{ID: "st-dfw-1", Name: "Dallas, US (AT&T)", URL: "http://dfw.speedtest.net/speedtest/upload.php"},
	{ID: "st-mia-1", Name: "Miami, US (FPL FiberNet)", URL: "http://mia.speedtest.net/speedtest/upload.php"},
	{ID: "st-yyz-1", Name: "Toronto, CA (Rogers)", URL: "http://yyz.speedtest.net/speedtest/upload.php"},
	{ID: "st-mad-1", Name: "Madrid, ES (Telefonica)", URL: "http://mad.speedtest.net/speedtest/upload.php"},
	{ID: "st-zrh-1", Name: "Zurich, CH (Swisscom)", URL: "http://zrh.speedtest.net/speedtest/upload.php"},
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
			Timeout: 10 * time.Second,
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

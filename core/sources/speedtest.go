package sources

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultSpeedtestServerListURL = "https://www.speedtest.net/speedtest-servers-static.php"

// Ookla speedtest servers use this URL pattern for downloads.
// random4000x4000.jpg is ~30MB of random data per request.
const ooklaDownloadPath = "/speedtest/random4000x4000.jpg"
const ooklaUploadPath = "/speedtest/upload.php"

// xmlSettings mirrors the Ookla speedtest-servers XML structure.
type xmlSettings struct {
	Servers []xmlServer `xml:"servers>server"`
}

type xmlServer struct {
	URL     string `xml:"url,attr"`
	Host    string `xml:"host,attr"`
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
			Timeout: 0, // no timeout — workers manage lifecycle via context
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 64,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (s *SpeedtestSource) Name() string     { return "Speedtest" }
func (s *SpeedtestSource) Type() SourceType { return SourceTypeSpeedtest }

// BundledServers returns a small fallback list of known Ookla servers.
func (s *SpeedtestSource) BundledServers() []Server {
	return []Server{
		{ID: "ookla-8795", Name: "Tripleplay – Noida, India", URL: "http://speedtestnoida.tripleplay.in:8080"},
		{ID: "ookla-27751", Name: "Costel Networks – Noida, India", URL: "http://speedtest.costelnetworks.com:8080"},
		{ID: "ookla-10020", Name: "Powernet – Gurgaon, India", URL: "http://spgur.pcpli.net:8080"},
		{ID: "ookla-50174", Name: "Siti Broadband – Noida, India", URL: "http://noidaspeedtest.sitibroadband.co.in:8080"},
		{ID: "ookla-36995", Name: "Cityline Networks – Greater Noida, India", URL: "http://citylinegrn.speedtest.bhaukaalbaba.com:8080"},
		{ID: "ookla-16377", Name: "Airtel – Mumbai, India", URL: "http://speedtestmum.bharti.com:8080"},
		{ID: "ookla-9214", Name: "BSNL – New Delhi, India", URL: "http://speedtest.bsnl.co.in:8080"},
		{ID: "ookla-21070", Name: "Jio – Mumbai, India", URL: "http://speedtestmumbai.jio.com:8080"},
		{ID: "ookla-27249", Name: "Excitel – New Delhi, India", URL: "http://speedtest1.excitel.com:8080"},
		{ID: "ookla-15312", Name: "ACT Fibernet – Bangalore, India", URL: "http://speedtest.actcorp.in:8080"},
	}
}

// ooklaBaseURL extracts the base URL from an Ookla server XML entry.
// XML gives: "http://host:port/speedtest/upload.php" or host attr: "host:port"
// We need: "http://host:port"
func ooklaBaseURL(xmlURL, host string) string {
	// If host attr is available, use it
	if host != "" {
		return "http://" + host
	}
	// Strip the path from the URL
	if idx := strings.Index(xmlURL, "/speedtest/"); idx > 0 {
		return xmlURL[:idx]
	}
	// Strip trailing upload.php or similar
	if idx := strings.LastIndex(xmlURL, "/"); idx > 8 { // after http://
		return xmlURL[:idx]
	}
	return xmlURL
}

// Discover fetches the Ookla server list, falls back to bundled on failure.
func (s *SpeedtestSource) Discover() ([]Server, error) {
	servers, err := s.fetchLiveServers()
	if err != nil || len(servers) == 0 {
		return s.BundledServers(), nil
	}
	return servers, nil
}

// fetchLiveServers fetches and parses the Ookla XML server list.
func (s *SpeedtestSource) fetchLiveServers() ([]Server, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(s.serverListURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("server list fetch failed: " + resp.Status)
	}

	var settings xmlSettings
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&settings); err != nil {
		return nil, err
	}

	servers := make([]Server, 0, len(settings.Servers))
	for _, xs := range settings.Servers {
		if xs.URL == "" || xs.ID == "" {
			continue
		}
		baseURL := ooklaBaseURL(xs.URL, xs.Host)
		name := xs.Name
		if xs.Sponsor != "" {
			name = fmt.Sprintf("%s – %s, %s", xs.Sponsor, xs.Name, xs.Country)
		}
		servers = append(servers, Server{
			ID:   "ookla-" + xs.ID,
			Name: name,
			URL:  baseURL,
		})
	}
	if len(servers) == 0 {
		return nil, errors.New("no servers in list")
	}
	return servers, nil
}

// Download requests random data from the Ookla server's download endpoint.
func (s *SpeedtestSource) Download(ctx context.Context, server Server) (io.ReadCloser, error) {
	dlURL := server.URL + ooklaDownloadPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, errors.New("download failed: " + resp.Status)
	}
	return resp.Body, nil
}

// Upload POSTs data to the Ookla server's upload endpoint.
func (s *SpeedtestSource) Upload(ctx context.Context, server Server, r io.Reader) error {
	ulURL := server.URL + ooklaUploadPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ulURL, r)
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

// Latency measures round-trip time using a small HTTP GET to the server.
func (s *SpeedtestSource) Latency(server Server) (time.Duration, error) {
	// Use /speedtest/latency.txt — standard Ookla latency endpoint
	latURL := server.URL + "/speedtest/latency.txt"
	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	resp, err := client.Get(latURL)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return time.Since(start), nil
}

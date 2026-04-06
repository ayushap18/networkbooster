package sources

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

var defaultCDNServers = []Server{
	{ID: "hetzner-us", Name: "Hetzner US", URL: "https://ash-speed.hetzner.com/10GB.bin"},
	{ID: "hetzner-fi", Name: "Hetzner Finland", URL: "https://hel1-speed.hetzner.com/10GB.bin"},
	{ID: "telia-se", Name: "Telia Sweden", URL: "http://speedtest.tele2.net/10GB.zip"},
}

type CDNSource struct {
	client *http.Client
}

func NewCDNSource() *CDNSource {
	return &CDNSource{
		client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 64,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (c *CDNSource) Name() string     { return "CDN Mirrors" }
func (c *CDNSource) Type() SourceType { return SourceTypeCDN }

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

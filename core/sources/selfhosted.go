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

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

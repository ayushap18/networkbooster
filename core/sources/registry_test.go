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

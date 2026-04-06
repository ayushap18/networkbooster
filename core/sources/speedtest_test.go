package sources_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ayush18/networkbooster/core/sources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ooklaTestServer mimics Ookla speedtest server endpoints.
func ooklaTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/speedtest/random4000x4000.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		io.CopyN(w, rand.Reader, 1024*1024) // 1MB for test
	})
	mux.HandleFunc("/speedtest/upload.php", func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/speedtest/latency.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test=test"))
	})
	return httptest.NewServer(mux)
}

func TestSpeedtestSource_Name(t *testing.T) {
	st := sources.NewSpeedtestSource()
	assert.Equal(t, "Speedtest", st.Name())
	assert.Equal(t, sources.SourceTypeSpeedtest, st.Type())
}

func TestSpeedtestSource_BundledServers(t *testing.T) {
	st := sources.NewSpeedtestSource()
	bundled := st.BundledServers()
	assert.GreaterOrEqual(t, len(bundled), 5, "should have at least 5 bundled servers")
	for _, s := range bundled {
		assert.NotEmpty(t, s.ID)
		assert.NotEmpty(t, s.URL)
		assert.NotEmpty(t, s.Name)
	}
}

const sampleServerXML = `<?xml version="1.0" encoding="UTF-8"?>
<settings>
  <servers>
    <server url="http://example.com/speedtest/upload.php" host="example.com:8080" lat="40.71" lon="-74.00"
            name="New York" country="United States" sponsor="TestISP" id="1001"/>
    <server url="http://example2.com/speedtest/upload.php" host="example2.com:8080" lat="51.51" lon="-0.13"
            name="London" country="United Kingdom" sponsor="TestISP2" id="1002"/>
  </servers>
</settings>`

func TestSpeedtestSource_LiveDiscovery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(sampleServerXML))
	}))
	defer ts.Close()

	st := sources.NewSpeedtestSourceWithURL(ts.URL)
	servers, err := st.Discover()
	require.NoError(t, err)
	require.Len(t, servers, 2)

	assert.Equal(t, "ookla-1001", servers[0].ID)
	assert.Equal(t, "http://example.com:8080", servers[0].URL)
	assert.Contains(t, servers[0].Name, "TestISP")

	assert.Equal(t, "ookla-1002", servers[1].ID)
	assert.Equal(t, "http://example2.com:8080", servers[1].URL)
	assert.Contains(t, servers[1].Name, "TestISP2")
}

func TestSpeedtestSource_LiveDiscoveryFails_FallsToBundled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	st := sources.NewSpeedtestSourceWithURL(ts.URL)
	servers, err := st.Discover()
	require.NoError(t, err)
	bundled := st.BundledServers()
	assert.Equal(t, len(bundled), len(servers))
}

func TestSpeedtestSource_Download(t *testing.T) {
	ts := ooklaTestServer()
	defer ts.Close()

	st := sources.NewSpeedtestSource()
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL}

	rc, err := st.Download(context.Background(), server)
	require.NoError(t, err)
	defer rc.Close()

	n, err := io.Copy(io.Discard, rc)
	require.NoError(t, err)
	assert.Greater(t, n, int64(0))
}

func TestSpeedtestSource_Upload(t *testing.T) {
	ts := ooklaTestServer()
	defer ts.Close()

	st := sources.NewSpeedtestSource()
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL}

	data := bytes.NewReader(make([]byte, 1024))
	err := st.Upload(context.Background(), server, data)
	require.NoError(t, err)
}

func TestSpeedtestSource_Latency(t *testing.T) {
	ts := ooklaTestServer()
	defer ts.Close()

	st := sources.NewSpeedtestSource()
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL}

	latency, err := st.Latency(server)
	require.NoError(t, err)
	assert.Greater(t, latency.Nanoseconds(), int64(0))
}

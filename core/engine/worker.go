package engine

import (
	"context"
	"io"
	"time"

	"github.com/ayush18/networkbooster/core/metrics"
	"github.com/ayush18/networkbooster/core/sources"
)

const (
	readBufSize     = 256 * 1024 // 256KB for better throughput
	uploadChunkSize = 1024 * 1024
)

type Worker struct {
	ID        string
	source    sources.Source
	server    sources.Server
	collector *metrics.Collector
}

func NewWorker(id string, source sources.Source, server sources.Server, collector *metrics.Collector) *Worker {
	return &Worker{
		ID:        id,
		source:    source,
		server:    server,
		collector: collector,
	}
}

func (w *Worker) RunDownload(ctx context.Context) {
	w.collector.AddConnection(w.ID)
	defer w.collector.RemoveConnection(w.ID)

	buf := make([]byte, readBufSize)
	for {
		if ctx.Err() != nil {
			return
		}
		rc, err := w.source.Download(ctx, w.server)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// Back off before retrying to avoid tight error loops
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return
			}
			continue
		}
		w.readLoop(ctx, rc, buf)
		rc.Close()
	}
}

func (w *Worker) readLoop(ctx context.Context, rc io.ReadCloser, buf []byte) {
	for {
		if ctx.Err() != nil {
			return
		}
		n, err := rc.Read(buf)
		if n > 0 {
			w.collector.RecordBytes(metrics.DirectionDownload, int64(n))
		}
		if err != nil {
			return
		}
	}
}

func (w *Worker) RunUpload(ctx context.Context) {
	w.collector.AddConnection(w.ID)
	defer w.collector.RemoveConnection(w.ID)

	for {
		if ctx.Err() != nil {
			return
		}
		pr, pw := io.Pipe()

		done := make(chan struct{})
		go func() {
			defer close(done)
			defer pw.Close()
			chunk := make([]byte, uploadChunkSize)
			for {
				if ctx.Err() != nil {
					return
				}
				n, err := pw.Write(chunk)
				if err != nil {
					return
				}
				w.collector.RecordBytes(metrics.DirectionUpload, int64(n))
			}
		}()

		err := w.source.Upload(ctx, w.server, pr)
		pr.Close()
		<-done

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// Back off before retrying
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}
}

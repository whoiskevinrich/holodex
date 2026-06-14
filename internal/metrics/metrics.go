// Package metrics exposes a minimal, dependency-free Prometheus endpoint (F13,
// ADR-019/ADR-026). It implements just the four metrics Phase 2 requires and
// writes the text exposition format by hand, keeping the lean go.mod the project
// favours (the same "standard library, no dependency" rationale behind choosing
// slog over a logging framework) rather than pulling in client_golang's tree.
package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Registry holds the process-wide metrics. The zero value is not usable; call New.
type Registry struct {
	indexedFiles atomic.Int64
	scanDur      *histogram
	searchDur    *histogram

	// queueDepth is the live thumbnail-queue gauge source, set once at startup
	// (SetQueueDepthSource) before serving and read only on the cold /metrics
	// path. The write happens-before any scrape, so it needs no lock. nil → 0.
	queueDepth func() int
}

// New builds the registry with histogram buckets tuned to each metric: scans run
// from milliseconds to minutes, searches stay sub-second.
func New() *Registry {
	return &Registry{
		scanDur:   newHistogram([]float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300}),
		searchDur: newHistogram([]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5}),
	}
}

// SetQueueDepthSource wires the live thumbnail-queue depth read at scrape time
// (F11.8). Call once at startup before serving.
func (r *Registry) SetQueueDepthSource(fn func() int) { r.queueDepth = fn }

// ObserveScan records a completed scan pass: its duration and how many files were
// (re)indexed (added + updated).
func (r *Registry) ObserveScan(d time.Duration, indexed int) {
	r.indexedFiles.Add(int64(indexed))
	r.scanDur.observe(d.Seconds())
}

// ObserveSearch records the latency of one search query.
func (r *Registry) ObserveSearch(d time.Duration) { r.searchDur.observe(d.Seconds()) }

// Handler serves the Prometheus exposition at GET /metrics (F13.1).
func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var sb strings.Builder

		writeCounter(&sb, "holodex_indexed_files_total",
			"Total media files indexed (added or updated) since process start.",
			r.indexedFiles.Load())

		depth := 0
		if r.queueDepth != nil {
			depth = r.queueDepth()
		}
		writeGauge(&sb, "holodex_thumbnail_queue_depth",
			"Pending thumbnail generation jobs (F11.8).", float64(depth))

		r.scanDur.write(&sb, "holodex_scan_duration_seconds",
			"Duration of a full library scan pass, in seconds.")
		r.searchDur.write(&sb, "holodex_search_duration_seconds",
			"Latency of a search query, in seconds.")

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(sb.String()))
	}
}

// ---- exposition helpers ----

func writeCounter(sb *strings.Builder, name, help string, v int64) {
	sb.WriteString("# HELP " + name + " " + help + "\n")
	sb.WriteString("# TYPE " + name + " counter\n")
	sb.WriteString(name + " " + strconv.FormatInt(v, 10) + "\n")
}

func writeGauge(sb *strings.Builder, name, help string, v float64) {
	sb.WriteString("# HELP " + name + " " + help + "\n")
	sb.WriteString("# TYPE " + name + " gauge\n")
	sb.WriteString(name + " " + formatFloat(v) + "\n")
}

// histogram is a cumulative-bucket observer. counts[i] holds the number of
// observations whose smallest containing bucket is i; cumulative le-counts are
// summed at render time.
type histogram struct {
	mu      sync.Mutex
	buckets []float64 // upper bounds, ascending, no +Inf
	counts  []uint64
	sum     float64
	count   uint64
}

func newHistogram(buckets []float64) *histogram {
	return &histogram{buckets: buckets, counts: make([]uint64, len(buckets))}
}

func (h *histogram) observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sum += v
	h.count++
	for i, b := range h.buckets {
		if v <= b {
			h.counts[i]++
			return
		}
	}
	// v exceeds every finite bucket; it still counts toward +Inf and _count.
}

func (h *histogram) write(sb *strings.Builder, name, help string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sb.WriteString("# HELP " + name + " " + help + "\n")
	sb.WriteString("# TYPE " + name + " histogram\n")
	var cum uint64
	for i, b := range h.buckets {
		cum += h.counts[i]
		sb.WriteString(name + `_bucket{le="` + formatFloat(b) + `"} ` + strconv.FormatUint(cum, 10) + "\n")
	}
	sb.WriteString(name + `_bucket{le="+Inf"} ` + strconv.FormatUint(h.count, 10) + "\n")
	sb.WriteString(name + "_sum " + formatFloat(h.sum) + "\n")
	sb.WriteString(name + "_count " + strconv.FormatUint(h.count, 10) + "\n")
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

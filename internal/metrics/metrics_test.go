package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func scrape(t *testing.T, r *Registry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	r.Handler()(rec, httptest.NewRequest("GET", "/metrics", nil))
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content-type = %q", ct)
	}
	return rec.Body.String()
}

func TestExposition(t *testing.T) {
	r := New()
	r.SetQueueDepthSource(func() int { return 3 })
	r.ObserveScan(2*time.Second, 5)
	r.ObserveScan(45*time.Second, 2)
	r.ObserveSearch(20 * time.Millisecond)

	out := scrape(t, r)

	want := []string{
		"# TYPE holodex_indexed_files_total counter",
		"holodex_indexed_files_total 7", // 5 + 2
		"# TYPE holodex_thumbnail_queue_depth gauge",
		"holodex_thumbnail_queue_depth 3",
		"# TYPE holodex_scan_duration_seconds histogram",
		`holodex_scan_duration_seconds_bucket{le="+Inf"} 2`,
		"holodex_scan_duration_seconds_count 2",
		"# TYPE holodex_search_duration_seconds histogram",
		"holodex_search_duration_seconds_count 1",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("exposition missing %q\n--- got ---\n%s", w, out)
		}
	}
}

// A 2s scan falls in the le="2.5" bucket; cumulative counts must be monotonic and
// the 45s scan must only appear from le="60" onward.
func TestHistogramBucketing(t *testing.T) {
	r := New()
	r.ObserveScan(2*time.Second, 0)  // ≤ 2.5
	r.ObserveScan(45*time.Second, 0) // ≤ 60
	out := scrape(t, r)

	checks := map[string]string{
		`holodex_scan_duration_seconds_bucket{le="2.5"} `: "1",
		`holodex_scan_duration_seconds_bucket{le="5"} `:   "1",
		`holodex_scan_duration_seconds_bucket{le="60"} `:  "2",
		`holodex_scan_duration_seconds_bucket{le="300"} `: "2",
	}
	for prefix, val := range checks {
		line := findLine(out, prefix)
		if line != prefix+val {
			t.Errorf("bucket line = %q, want %q", line, prefix+val)
		}
	}
}

func findLine(out, prefix string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return ""
}

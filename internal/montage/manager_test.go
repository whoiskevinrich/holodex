package montage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"holodex/internal/repo"
)

// fakeRepo is an in-memory Repository.
type fakeRepo struct {
	mu    sync.Mutex
	cands map[int64]repo.MontageCandidate
	rows  map[int64]repo.MontageRow
	asked [][]int64 // MontageAttempted calls
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{cands: map[int64]repo.MontageCandidate{}, rows: map[int64]repo.MontageRow{}}
}

func (f *fakeRepo) add(id int64, dur int) {
	f.mu.Lock()
	f.cands[id] = repo.MontageCandidate{ID: id, FilePath: "/m/" + string(rune('a'+id)) + ".mkv", DurationSec: dur}
	f.mu.Unlock()
}

func (f *fakeRepo) row(id int64) (repo.MontageRow, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	return r, ok
}

func (f *fakeRepo) SetMontageState(_ context.Context, id int64, state, errText string) error {
	f.mu.Lock()
	f.rows[id] = repo.MontageRow{VideoID: id, State: state, Error: errText}
	f.mu.Unlock()
	return nil
}

func (f *fakeRepo) MontageState(_ context.Context, id int64) (repo.MontageRow, bool, error) {
	r, ok := f.row(id)
	return r, ok, nil
}

func (f *fakeRepo) MontageAttempted(_ context.Context, ids []int64) (map[int64]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, ids)
	out := map[int64]bool{}
	for _, id := range ids {
		if _, ok := f.rows[id]; ok {
			out[id] = true
		}
	}
	return out, nil
}

func (f *fakeRepo) MontageBackfillCandidates(_ context.Context, minDur, _ int) ([]repo.MontageCandidate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []repo.MontageCandidate
	for id, c := range f.cands {
		if _, done := f.rows[id]; !done && c.DurationSec >= minDur {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeRepo) MontageCandidateByID(_ context.Context, id int64) (repo.MontageCandidate, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.cands[id]
	return c, ok, nil
}

type fakeStills struct{ depth atomic.Int64 }

func (s *fakeStills) QueueDepth() int { return int(s.depth.Load()) }

func testManager(t *testing.T, mode Mode, r Repository, stills StillQueue) *Manager {
	t.Helper()
	m := New(Config{Mode: mode, Dir: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil)), r, stills)
	m.stillPoll = 5 * time.Millisecond
	return m
}

func TestParseMode(t *testing.T) {
	for in, want := range map[string]Mode{"": ModeManual, "manual": ModeManual, "off": ModeOff, "automatic": ModeAutomatic} {
		if got, err := ParseMode(in); err != nil || got != want {
			t.Errorf("ParseMode(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"auto", "on", "Manual", "eager"} {
		if _, err := ParseMode(bad); err == nil {
			t.Errorf("ParseMode(%q) = nil error; an unknown mode must fail startup", bad)
		}
	}
}

func TestNew_Defaults(t *testing.T) {
	m := New(Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), newFakeRepo(), nil)
	if m.cfg.Mode != ModeManual || m.cfg.Workers != 1 {
		t.Errorf("defaults = mode %q workers %d; want manual, 1", m.cfg.Mode, m.cfg.Workers)
	}
}

// G2: manual mode never queues from a browse, only from Request.
func TestManual_OnlyRequestEnqueues(t *testing.T) {
	r := newFakeRepo()
	m := testManager(t, ModeManual, r, nil)
	m.EnqueueVisible(context.Background(), []int64{1, 2, 3})
	if h, n := m.queue.counts(); h+n != 0 {
		t.Fatalf("browse queued %d in manual mode", h+n)
	}
	if len(r.asked) != 0 {
		t.Errorf("manual browse reached the repo: %v", r.asked)
	}
	if err := m.Request(7); err != nil {
		t.Fatal(err)
	}
	if h, _ := m.queue.counts(); h != 1 {
		t.Errorf("Request queued high=%d, want 1", h)
	}
}

// G3: automatic mode queues only never-attempted ids from a browse.
func TestAutomatic_EnqueueVisibleSkipsAttempted(t *testing.T) {
	r := newFakeRepo()
	_ = r.SetMontageState(context.Background(), 2, StateFailed, "x")
	m := testManager(t, ModeAutomatic, r, nil)
	m.EnqueueVisible(context.Background(), []int64{1, 2, 3})
	if _, n := m.queue.counts(); n != 2 || m.queue.isPending(2) {
		t.Errorf("normal=%d pending(2)=%v; want 2 queued and the failed id skipped", n, m.queue.isPending(2))
	}
}

// G4: off mode refuses requests and starts no workers.
func TestOff_RefusesAndRunReturns(t *testing.T) {
	m := testManager(t, ModeOff, newFakeRepo(), nil)
	if err := m.Request(1); !errors.Is(err, ErrDisabled) {
		t.Errorf("Request in off mode = %v, want ErrDisabled", err)
	}
	done := make(chan struct{})
	go func() { m.Run(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return at once in off mode")
	}
	if st := m.QueueStats(); st.Workers != 0 || st.Mode != ModeOff {
		t.Errorf("stats = %+v, want off with 0 workers", st)
	}
}

// G5: a worker takes nothing while any still is waiting.
func TestWorker_WaitsBehindStills(t *testing.T) {
	r := newFakeRepo()
	r.add(1, 600)
	stills := &fakeStills{}
	stills.depth.Store(3)
	m := testManager(t, ModeManual, r, stills)
	var ran atomic.Int32
	m.gen = func(context.Context, string, []float64, string) error { ran.Add(1); return nil }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)
	_ = m.Request(1)
	time.Sleep(60 * time.Millisecond)
	if ran.Load() != 0 {
		t.Fatal("montage ran while stills were waiting")
	}
	stills.depth.Store(0)
	waitFor(t, func() bool { _, ok := r.row(1); return ok })
	if got, _ := r.row(1); got.State != StateGenerated {
		t.Errorf("state = %q, want generated", got.State)
	}
}

// G6: sampling.
func TestClipStarts(t *testing.T) {
	cases := []struct {
		dur  int
		want int
	}{{600, 6}, {60, 6}, {30, 6}, {20, 4}, {10, 2}, {6, 2}, {5, 0}, {0, 0}}
	for _, c := range cases {
		s := clipStarts(c.dur)
		if len(s) != c.want {
			t.Errorf("clipStarts(%d) has %d clips, want %d", c.dur, len(s), c.want)
			continue
		}
		d := float64(c.dur)
		for i, v := range s {
			if v < 0 || v+clipSeconds > d {
				t.Errorf("clipStarts(%d)[%d] = %v runs outside the video", c.dur, i, v)
			}
			if i > 0 && v <= s[i-1] {
				t.Errorf("clipStarts(%d) not ascending: %v", c.dur, s)
			}
		}
	}
	s := clipStarts(600)
	if s[0] < 0.10*600-1 || s[len(s)-1] > 0.90*600 {
		t.Errorf("clipStarts(600) = %v strays outside 10%%–90%%", s)
	}
}

// G7: the argv.
func TestBuildArgs(t *testing.T) {
	args := buildArgs("/abs/a.mkv", []float64{60, 120}, 400, "/d/1.mp4.tmp")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-ss 60.000 -t 1.000 -i /abs/a.mkv -ss 120.000 -t 1.000 -i /abs/a.mkv",
		"concat=n=2:v=1:a=0[out]",
		"scale=w='min(400,iw)':h=-2",
		"-an", "-pix_fmt yuv420p", "-movflags +faststart", "-f mp4", "/d/1.mp4.tmp",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv missing %q:\n%s", want, joined)
		}
	}
	if i, j := slices.Index(args, "-ss"), slices.Index(args, "-i"); i > j {
		t.Error("the seek must come before -i (input-side seek)")
	}
}

// G8: outcomes are recorded in video_montages, and a too-short video fails
// without running ffmpeg.
func TestProcess_RecordsOutcomes(t *testing.T) {
	r := newFakeRepo()
	r.add(1, 600)
	r.add(2, 600)
	r.add(3, 4)
	m := testManager(t, ModeManual, r, nil)
	m.gen = func(_ context.Context, src string, _ []float64, _ string) error {
		if strings.Contains(src, "c") { // id 2
			return errors.New("ffmpeg: exit status 1: moov atom not found")
		}
		return nil
	}
	ctx := context.Background()
	m.process(ctx, 1)
	m.process(ctx, 2)
	m.process(ctx, 3)
	if got, _ := r.row(1); got.State != StateGenerated {
		t.Errorf("id 1 = %+v, want generated", got)
	}
	if got, _ := r.row(2); got.State != StateFailed || !strings.Contains(got.Error, "moov") {
		t.Errorf("id 2 = %+v, want failed with the ffmpeg reason", got)
	}
	if got, _ := r.row(3); got.State != StateFailed || got.Error != ErrTooShort.Error() {
		t.Errorf("id 3 = %+v, want failed: too short", got)
	}
}

// G9: a file change re-queues only a video that already had a montage.
func TestInvalidate(t *testing.T) {
	r := newFakeRepo()
	_ = r.SetMontageState(context.Background(), 1, StateGenerated, "")
	m := testManager(t, ModeManual, r, nil)
	m.Invalidate(context.Background(), 1)
	m.Invalidate(context.Background(), 2)
	if !m.queue.isPending(1) || m.queue.isPending(2) {
		t.Errorf("pending(1)=%v pending(2)=%v; want only the attempted id", m.queue.isPending(1), m.queue.isPending(2))
	}
	off := testManager(t, ModeOff, r, nil)
	off.Invalidate(context.Background(), 1)
	if off.queue.isPending(1) {
		t.Error("off mode re-queued on a file change")
	}
}

// G10: dedup.
func TestRequest_Dedup(t *testing.T) {
	m := testManager(t, ModeManual, newFakeRepo(), nil)
	_ = m.Request(5)
	_ = m.Request(5)
	if h, _ := m.queue.counts(); h != 1 {
		t.Errorf("high = %d, want 1", h)
	}
}

func TestStatus(t *testing.T) {
	r := newFakeRepo()
	m := testManager(t, ModeManual, r, nil)
	ctx := context.Background()
	if st, _ := m.Status(ctx, 1); st.State != "none" {
		t.Errorf("never attempted = %q, want none", st.State)
	}
	_ = m.Request(1)
	if st, _ := m.Status(ctx, 1); st.State != "waiting" {
		t.Errorf("queued = %q, want waiting", st.State)
	}
	m.setMaking(1, true)
	if st, _ := m.Status(ctx, 1); st.State != "making" {
		t.Errorf("running = %q, want making", st.State)
	}
	m.setMaking(1, false)
	m.queue.done(1)
	_ = r.SetMontageState(ctx, 1, StateFailed, "can't read file")
	if st, _ := m.Status(ctx, 1); st.State != "failed" || st.Error != "can't read file" {
		t.Errorf("failed = %+v", st)
	}
	if err := os.WriteFile(Path(m.cfg.Dir, 1), make([]byte, 248), 0o644); err != nil {
		t.Fatal(err)
	}
	// A failed Remake over an older file still reports the failure.
	if st, _ := m.Status(ctx, 1); st.State != "failed" {
		t.Errorf("failed remake with an old file = %q, want failed", st.State)
	}
	_ = r.SetMontageState(ctx, 1, StateGenerated, "")
	if st, _ := m.Status(ctx, 1); !reflect.DeepEqual(st, Status{State: "ready", SizeBytes: 248}) {
		t.Errorf("ready = %+v", st)
	}
}

// An owner request promotes a video already waiting in the backfill tier.
func TestRequest_PromotesBackfillItem(t *testing.T) {
	m := testManager(t, ModeAutomatic, newFakeRepo(), nil)
	m.queue.push(1, false)
	m.queue.push(2, false)
	_ = m.Request(2)
	if id, _ := m.queue.pop(); id != 2 {
		t.Errorf("first pop = %d, want the requested id 2", id)
	}
	if h, n := m.queue.counts(); h != 0 || n != 1 {
		t.Errorf("after pop high=%d normal=%d, want 0 and 1", h, n)
	}
}

// Requests are refused, not left waiting, when the workers couldn't start.
func TestRequest_UnavailableWhenDirFails(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(Config{Mode: ModeManual, Dir: filepath.Join(blocker, "montages")},
		slog.New(slog.NewTextHandler(io.Discard, nil)), newFakeRepo(), nil)
	m.Run(context.Background())
	if err := m.Request(1); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Request = %v, want ErrUnavailable", err)
	}
}

func TestPath(t *testing.T) {
	if got := Path("/data/montages", 42); got != filepath.Join("/data/montages", "42.mp4") {
		t.Errorf("Path = %q", got)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

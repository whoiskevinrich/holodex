package writequeue_test

import (
	"context"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/writeback"
	"holodex/internal/writequeue"
)

// ADR-110: a genres job makes every tag key on the file match the written set.

// runJob enqueues fields for videoID on a queue whose file reads are faked, and
// returns the one batch the worker wrote (nil when it wrote nothing).
func runJob(t *testing.T, r *repo.Repo, videoID int64, fields []writequeue.JobField,
	present map[string][]string, current map[string]string,
) []writeback.FieldWrite {
	t.Helper()
	var mu sync.Mutex
	var got []writeback.FieldWrite
	wrote := false
	write := func(_ context.Context, _ string, fs []writeback.FieldWrite) error {
		mu.Lock()
		defer mu.Unlock()
		got, wrote = fs, true
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := writequeue.New(r, write, testLogger(), 1, "")
	writequeue.SetFileReaders(q,
		func(context.Context, string, string) (map[string][]string, error) { return present, nil },
		func(context.Context, string, []writeback.Mapped) (map[string]string, error) { return current, nil },
	)
	q.Start(ctx)
	if _, err := q.Enqueue(ctx, videoID, fields); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitFor(t, func() bool {
		n, _ := r.PendingWritebackCount(ctx)
		return n == 0
	})
	// The pending count drops when the job is claimed; wait for its job_run too.
	waitFor(t, func() bool {
		runs, _ := r.ListJobRuns(ctx, 10)
		return slices.ContainsFunc(runs, func(j model.JobRun) bool { return j.Kind == model.JobKindWriteback })
	})
	mu.Lock()
	defer mu.Unlock()
	if !wrote {
		return nil
	}
	sort.Slice(got, func(i, j int) bool { return got[i].TagName < got[j].TagName })
	return got
}

func byTag(fs []writeback.FieldWrite) map[string]writeback.FieldWrite {
	m := make(map[string]writeback.FieldWrite, len(fs))
	for _, f := range fs {
		m[f.TagName] = f
	}
	return m
}

func TestTagKeyFilter_RemovesValuesNotInTheWrittenSet(t *testing.T) {
	r := newRepo(t)
	id := seedVideo(t, r, "MP4")
	got := byTag(runJob(t, r, id,
		[]writequeue.JobField{{Field: "genres", Values: []string{"Drama"}, Source: "manual"}},
		map[string][]string{
			"Keys:Keywords":     {"Drama", "Heist"}, // filtered to Drama
			"ItemList:Category": {"Heist"},          // nothing left → deleted
			"Keys:Genres":       {"drama"},          // same tag, different case → unchanged, not written
		}, nil))

	if g := got["QuickTime:Genre"]; !slices.Equal(g.Values, []string{"Drama"}) {
		t.Errorf("Genre = %+v, want [Drama]", g)
	}
	if k := got["Keys:Keywords"]; !slices.Equal(k.Values, []string{"Drama"}) || k.Delete {
		t.Errorf("Keywords = %+v, want [Drama]", k)
	}
	if c := got["ItemList:Category"]; !c.Delete || len(c.Values) != 0 {
		t.Errorf("Category = %+v, want a delete", c)
	}
	if _, ok := got["Keys:Genres"]; ok {
		t.Error("an unchanged key was rewritten")
	}
}

func TestTagKeyFilter_KeepsAnAliasOfAWrittenTag(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	id := seedVideo(t, r, "MP4")
	tag, err := r.AttachTagToVideo(ctx, id, "Sci-Fi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.AddEntityAlias(ctx, model.EntityTag, tag.ID, "Science Fiction"); err != nil {
		t.Fatal(err)
	}
	got := byTag(runJob(t, r, id,
		[]writequeue.JobField{{Field: "genres", Values: []string{"Sci-Fi"}, Source: "manual"}},
		map[string][]string{"Keys:Keywords": {"Science Fiction", "Heist"}}, nil))
	if k := got["Keys:Keywords"]; !slices.Equal(k.Values, []string{"Science Fiction"}) {
		t.Errorf("Keywords = %+v, want the alias kept and Heist dropped", k)
	}
}

func TestTagKeyFilter_EmptyGenresDeletesGenre(t *testing.T) {
	r := newRepo(t)
	id := seedVideo(t, r, "MP4")
	got := byTag(runJob(t, r, id,
		[]writequeue.JobField{{Field: "genres", Source: "manual"}},
		map[string][]string{"Keys:Keywords": {"to-watch"}},
		map[string]string{"genres": "to-watch"}))
	if g := got["QuickTime:Genre"]; !g.Delete {
		t.Errorf("Genre = %+v, want a delete", g)
	}
	if k := got["Keys:Keywords"]; !k.Delete {
		t.Errorf("Keywords = %+v, want a delete", k)
	}
}

// A Genre delete on a file with no Genre and nothing else to filter is not a
// write at all — on Matroska it would be a full remux for nothing.
func TestTagKeyFilter_NothingToClearWritesNothing(t *testing.T) {
	r := newRepo(t)
	id := seedVideo(t, r, "Matroska")
	if got := runJob(t, r, id,
		[]writequeue.JobField{{Field: "genres", Source: "manual"}},
		map[string][]string{}, map[string]string{"genres": ""}); got != nil {
		t.Errorf("wrote %+v, want no write", got)
	}
}

// A revert restores the tag keys it snapshotted; filtering them against the
// restored Genre would undo the undo.
func TestTagKeyFilter_RevertIsNotFiltered(t *testing.T) {
	r := newRepo(t)
	id := seedVideo(t, r, "MP4")
	got := byTag(runJob(t, r, id, []writequeue.JobField{
		{Field: "genres", Values: []string{"Drama"}, Source: writequeue.SourceRevert},
		{Field: writeback.TagKeyFieldPrefix + "Keys:Keywords", Values: []string{"Drama", "Heist"}, Source: writequeue.SourceRevert},
	}, map[string][]string{"Keys:Keywords": {"Drama"}}, nil))
	if k := got["Keys:Keywords"]; !slices.Equal(k.Values, []string{"Drama", "Heist"}) {
		t.Errorf("Keywords = %+v, want the snapshot restored unfiltered", k)
	}
}

// A job payload is data: a tag-key field naming anything but an extra tag key
// never reaches the writer (security C2).
func TestTagKeyFilter_ForgedTagKeyIsNotWritten(t *testing.T) {
	r := newRepo(t)
	id := seedVideo(t, r, "MP4")
	got := byTag(runJob(t, r, id, []writequeue.JobField{
		{Field: "title", Values: []string{"T"}, Source: "manual"},
		{Field: writeback.TagKeyFieldPrefix + "System:FileName", Values: []string{"x.mp4"}, Source: "manual"},
	}, nil, nil))
	if _, ok := got["System:FileName"]; ok || len(got) != 1 {
		t.Errorf("wrote %+v, want only the title", got)
	}
}

// ADR-110 D4: an ignored tag the file supplied is re-linked as manual when a
// genres write succeeds, so the rescan that follows (the file no longer has it)
// keeps it on the video.
func TestTagKeyFilter_IgnoredFileTagSurvivesRescan(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	v := &model.Video{
		FilePath: t.TempDir() + "/v.mp4", FileSize: 1, Title: "t", Container: "MP4",
		FileMtime: time.Now().UTC().Truncate(time.Second),
		Tags:      []model.Tag{{Name: "to-watch"}},
	}
	id, err := r.UpsertVideo(ctx, v, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := r.GetVideo(ctx, id)
	if err != nil || len(got.Tags) != 1 {
		t.Fatalf("seed: tags %+v err %v", got, err)
	}
	if _, err := r.SetTagWritebackEnabled(ctx, got.Tags[0].ID, false); err != nil {
		t.Fatal(err)
	}

	runJob(t, r, id, []writequeue.JobField{{Field: "genres", Source: "manual"}},
		map[string][]string{}, map[string]string{"genres": "to-watch"})

	// Rescan: the file no longer carries the tag.
	v.ID, v.Tags = id, nil
	if _, err := r.UpsertVideo(ctx, v, nil); err != nil {
		t.Fatal(err)
	}
	after, _, err := r.GetVideo(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Tags) != 1 || after.Tags[0].Name != "to-watch" {
		t.Errorf("tags after rescan = %+v, want to-watch kept", after.Tags)
	}
}

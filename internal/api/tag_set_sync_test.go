package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"holodex/internal/model"
)

type tagSetMedia struct {
	Video struct {
		Tags []struct {
			Name    string `json:"name"`
			Written *bool  `json:"written"`
			OnFile  *bool  `json:"on_file"`
		} `json:"tags"`
	} `json:"video"`
	Resolved []struct {
		Canonical string `json:"canonical"`
		Items     []struct {
			Value  string `json:"value"`
			OnFile *bool  `json:"on_file"`
		} `json:"items"`
		FileOnly []string `json:"file_only"`
		InSync   *bool    `json:"in_sync"`
	} `json:"resolved"`
}

func fetchTagSetMedia(t *testing.T, url string) tagSetMedia {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET media: %v", err)
	}
	defer resp.Body.Close()
	var body tagSetMedia
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode media: %v", err)
	}
	return body
}

func boolStr(b *bool) string {
	if b == nil {
		return "nil"
	}
	if *b {
		return "true"
	}
	return "false"
}

// TestGetMedia_TagSetSync covers F72 P0-2/P0-3 (ADR-111 D3/D4): the genres row
// carries the write set against the file's recorded tag set, and every attached tag
// says whether it is written and whether it is on the file. Acceptance 2 + 7.
func TestGetMedia_TagSetSync(t *testing.T) {
	_, srv, r, vid, _ := genreWritebackServer(t)
	ctx := context.Background()

	// "Science Fiction" exists with alias Sci-Fi before the scan, so the file's
	// Sci-Fi links to it — and must count as on the file, not as a drop.
	sf, err := r.AttachTagToVideo(ctx, vid, "Science Fiction")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.AddEntityAlias(ctx, model.EntityTag, sf.ID, "Sci-Fi"); err != nil {
		t.Fatal(err)
	}
	// Rescan: the file holds Drama, Noir, heist, Sci-Fi (P0-1 records every name).
	if _, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "A", Container: "Matroska",
		FileMtime: time.Now().UTC().Truncate(time.Second),
		Tags:      []model.Tag{{Name: "Drama"}, {Name: "Noir"}, {Name: "heist"}, {Name: "Sci-Fi"}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AttachTagToVideo(ctx, vid, "crime"); err != nil {
		t.Fatal(err)
	}
	noir, err := r.AttachTagToVideo(ctx, vid, "noir")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetTagWritebackEnabled(ctx, noir.ID, false); err != nil {
		t.Fatal(err)
	}

	body := fetchTagSetMedia(t, srv.URL+"/api/v1/media/"+itoa(vid))

	var found bool
	for _, f := range body.Resolved {
		if !strings.EqualFold(f.Canonical, "genres") {
			continue
		}
		found = true
		got := map[string]string{}
		for _, it := range f.Items {
			got[it.Value] = boolStr(it.OnFile)
		}
		want := map[string]string{"drama": "true", "heist": "true", "science fiction": "true", "crime": "false"}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("item %q on_file = %s, want %s (items %v)", k, got[k], v, got)
			}
		}
		if _, ok := got["noir"]; ok {
			t.Errorf("writeback-off noir is in the write set")
		}
		if !slices.Equal(f.FileOnly, []string{"Noir"}) {
			t.Errorf("file_only = %v, want [Noir] (file spelling; Sci-Fi is an alias, not a drop)", f.FileOnly)
		}
		if boolStr(f.InSync) != "false" {
			t.Errorf("in_sync = %s, want false", boolStr(f.InSync))
		}
	}
	if !found {
		t.Fatal("no genres row")
	}

	tags := map[string][2]string{}
	for _, tg := range body.Video.Tags {
		tags[tg.Name] = [2]string{boolStr(tg.Written), boolStr(tg.OnFile)}
	}
	want := map[string][2]string{
		"drama":           {"true", "true"},
		"crime":           {"true", "false"},
		"noir":            {"false", "true"},
		"science fiction": {"true", "true"},
	}
	for k, v := range want {
		if tags[k] != v {
			t.Errorf("tag %q written/on_file = %v, want %v", k, tags[k], v)
		}
	}
}

// TestGetMedia_TagSetSync_InSyncAndEmptyWriteSet: a file whose tags equal the write set
// reads in_sync true; with every tag ignored the row survives an empty write set, since
// that write clears Genre (P0-2d, ADR-110 D6). Acceptance 4 + 6.
func TestGetMedia_TagSetSync_InSyncAndEmptyWriteSet(t *testing.T) {
	_, srv, r, vid, _ := genreWritebackServer(t)
	ctx := context.Background()
	if _, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "A", Container: "Matroska",
		FileMtime: time.Now().UTC().Truncate(time.Second),
		Tags:      []model.Tag{{Name: "Drama"}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	genres := func() (inSync string, items int, fileOnly []string, ok bool) {
		for _, f := range fetchTagSetMedia(t, srv.URL+"/api/v1/media/"+itoa(vid)).Resolved {
			if strings.EqualFold(f.Canonical, "genres") {
				return boolStr(f.InSync), len(f.Items), f.FileOnly, true
			}
		}
		return "", 0, nil, false
	}
	if s, _, _, ok := genres(); !ok || s != "true" {
		t.Fatalf("in_sync = %q (row %v), want true", s, ok)
	}

	drama, err := r.AttachTagToVideo(ctx, vid, "drama")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetTagWritebackEnabled(ctx, drama.ID, false); err != nil {
		t.Fatal(err)
	}
	s, n, fo, ok := genres()
	if !ok {
		t.Fatal("genres row dropped with an empty write set — the write that clears Genre is hidden")
	}
	if s != "false" || n != 0 || !slices.Equal(fo, []string{"Drama"}) {
		t.Errorf("in_sync=%s items=%d file_only=%v, want false/0/[Drama]", s, n, fo)
	}
}

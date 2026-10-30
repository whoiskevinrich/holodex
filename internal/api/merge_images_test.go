package api_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"holodex/internal/model"
)

// A merge carries the merged-away person's photo into the survivor's empty slot
// (HOLODEX-607), and the file follows its row: the survivor serves the very bytes the
// merged person served, not the placeholder or a 404.
func TestPersonMerge_CarriedPhotoServesFromSurvivor(t *testing.T) {
	srv, r, alice := personImageServer(t, "")
	ctx := context.Background()
	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/y.mkv", Title: "Other", Duration: 60, Width: 1920, Height: 1080,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		People:    []model.Person{{Name: "Ally"}},
	}, nil)
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	linkPeople(t, r, vid, "Ally")
	ally, _, err := r.PersonIDByName(ctx, "Ally")
	if err != nil {
		t.Fatalf("person id: %v", err)
	}

	get := func(personID int64) []byte {
		t.Helper()
		resp, err := http.Get(srv.URL + "/api/v1/people/" + itoa(personID) + "/image/headshot")
		if err != nil {
			t.Fatalf("get headshot: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" {
			t.Fatalf("headshot of %d = %d %q, want 200 image/jpeg", personID, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
		b, _ := io.ReadAll(resp.Body)
		return b
	}

	if code, out := uploadImage(t, srv.URL+"/api/v1/people/"+itoa(ally)+"/image", "", "headshot", pngUpload(t, 64, 64)); code != http.StatusCreated {
		t.Fatalf("upload = %d %v", code, out)
	}
	before := get(ally)

	if code, out := postTok(t, srv.URL+"/api/v1/people/"+itoa(alice)+"/merge", "", map[string]int64{"from_id": ally}); code != http.StatusOK {
		t.Fatalf("merge = %d %v", code, out)
	}
	if after := get(alice); !bytes.Equal(after, before) {
		t.Errorf("survivor headshot is not the merged person's photo (%d bytes, want %d)", len(after), len(before))
	}
}

// The studio merge (the shared entity endpoint) settles its files the same way.
func TestStudioMerge_CarriedLogoServesFromSurvivor(t *testing.T) {
	srv, r, ghibli := studioImageServer(t, "")
	ctx := context.Background()
	vid, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/t.mkv", Title: "Other",
		FileMtime: time.Now().UTC().Truncate(time.Second),
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, vid, []string{"Ghibli"}, nil); err != nil {
		t.Fatalf("seed studio: %v", err)
	}
	var other int64
	studios, _ := r.ListStudios(ctx, false)
	for _, s := range studios {
		if s.Name == "Ghibli" {
			other = s.ID
		}
	}

	get := func(studioID int64) []byte {
		t.Helper()
		resp, err := http.Get(srv.URL + "/api/v1/studios/" + itoa(studioID) + "/images/" + model.StudioImageLogo)
		if err != nil {
			t.Fatalf("get logo: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("logo of %d = %d, want 200", studioID, resp.StatusCode)
		}
		b, _ := io.ReadAll(resp.Body)
		return b
	}

	if code := uploadStudioImage(t, srv, "", other, model.StudioImageLogo, tinyJPEG(t)); code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("upload = %d", code)
	}
	before := get(other)

	if code, out := postTok(t, srv.URL+"/api/v1/studios/"+itoa(ghibli)+"/merge", "", map[string]int64{"from_id": other}); code != http.StatusOK {
		t.Fatalf("merge = %d %v", code, out)
	}
	if after := get(ghibli); !bytes.Equal(after, before) {
		t.Errorf("survivor logo is not the merged studio's (%d bytes, want %d)", len(after), len(before))
	}
}

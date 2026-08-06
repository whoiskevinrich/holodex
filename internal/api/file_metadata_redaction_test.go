package api_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// findKeysAnywhere walks a decoded JSON tree (map[string]any / []any) collecting
// every value found under any of the given keys, regardless of nesting shape —
// each endpoint under test wraps its video list differently ("items", "videos",
// a per-shelf "items", etc.), so this checks for the leak without hardcoding each
// endpoint's envelope.
func findKeysAnywhere(v any, keys map[string]bool, found map[string][]any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if keys[k] {
				found[k] = append(found[k], val)
			}
			findKeysAnywhere(val, keys, found)
		}
	case []any:
		for _, item := range t {
			findKeysAnywhere(item, keys, found)
		}
	}
}

// TestFileMetadataRedactedAcrossVideoEndpoints (F52, HOLODEX-251/HOLODEX-252):
// "hide file metadata unless in owner mode" must hold on every public endpoint
// that serializes a model.Video, not just listMedia/getMedia — this covers a
// redaction gap found during the F52 security review, where getRelated,
// getPerson, getTag, getStudio, and search each independently serialize video
// lists from the same repo layer and were missed by the original redaction pass.
func TestFileMetadataRedactedAcrossVideoEndpoints(t *testing.T) {
	srv, r := identityServer(t, "secret")
	ctx := context.Background()

	id, err := r.UpsertVideo(ctx, &model.Video{
		FilePath: "/m/dune.mkv", Title: "Dune", Duration: 60, Width: 1920, Height: 1080,
		Container: "Matroska", VideoCodec: "hevc", AudioCodec: "eac3", BitrateKbps: 5000,
		FileMtime: time.Now().UTC().Truncate(time.Second),
		Tags:      []model.Tag{{Name: "scifi"}},
	}, nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := r.ReconcileVideoPeople(ctx, id, []repo.PersonRoleName{{Name: "Denis Villeneuve", Role: "director"}}, nil); err != nil {
		t.Fatalf("link person: %v", err)
	}
	pid, _, err := r.PersonIDByName(ctx, "Denis Villeneuve")
	if err != nil {
		t.Fatalf("person id: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, id, []string{"Legendary"}, nil); err != nil {
		t.Fatalf("link studio: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil || len(studios) == 0 {
		t.Fatalf("list studios: %v (%d studios)", err, len(studios))
	}
	sid := studios[0].ID
	tags, err := r.ListTags(ctx, false)
	if err != nil || len(tags) == 0 {
		t.Fatalf("list tags: %v (%d tags)", err, len(tags))
	}
	tid := tags[0].ID

	fileKeys := map[string]bool{"file_path": true, "video_codec": true}

	cases := []struct {
		name            string
		url             string
		guaranteesVideo bool // whether this endpoint is guaranteed (by construction) to include the seeded video
	}{
		{"listMedia", srv.URL + "/api/v1/media", true},
		{"getMedia", fmt.Sprintf("%s/api/v1/media/%d", srv.URL, id), true},
		{"getRelated", fmt.Sprintf("%s/api/v1/media/%d/related", srv.URL, id), false},
		{"getPerson", fmt.Sprintf("%s/api/v1/people/%d", srv.URL, pid), true},
		{"getTag", fmt.Sprintf("%s/api/v1/tags/%d", srv.URL, tid), true},
		{"getStudio", fmt.Sprintf("%s/api/v1/studios/%d", srv.URL, sid), true},
		{"search", srv.URL + "/api/v1/search?q=dune", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, visitorBody := getJSONTok(t, tc.url, "")
			visitorFound := map[string][]any{}
			findKeysAnywhere(visitorBody, fileKeys, visitorFound)
			for _, fp := range visitorFound["file_path"] {
				if fp != "" {
					t.Errorf("%s: visitor response leaked file_path: %v", tc.name, fp)
				}
			}
			if len(visitorFound["video_codec"]) > 0 {
				t.Errorf("%s: visitor response leaked video_codec key (should be omitted entirely): %v", tc.name, visitorFound["video_codec"])
			}

			_, ownerBody := getJSONTok(t, tc.url, "secret")
			ownerFound := map[string][]any{}
			findKeysAnywhere(ownerBody, fileKeys, ownerFound)
			if tc.guaranteesVideo {
				if len(ownerFound["video_codec"]) == 0 {
					t.Errorf("%s: owner response missing video_codec — expected the seeded video to appear", tc.name)
				}
				foundPath := false
				for _, fp := range ownerFound["file_path"] {
					if fp == "/m/dune.mkv" {
						foundPath = true
					}
				}
				if !foundPath {
					t.Errorf("%s: owner response missing the seeded video's file_path: %v", tc.name, ownerFound["file_path"])
				}
			}
		})
	}
}

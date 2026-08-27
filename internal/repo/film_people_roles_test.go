package repo_test

import (
	"context"
	"errors"
	"testing"

	"holodex/internal/repo"
)

// TestFilmPeopleRolesCRUD covers add/edit/remove for HOLODEX-281, and the
// inherited-cast (video_people, via FilmCast) vs credited-role (film_people_roles,
// via FilmPeopleRoles) distinction the spec calls out.
func TestFilmPeopleRolesCRUD(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	filmID, err := r.CreateFilm(ctx, "Roles Test", 2023)
	if err != nil {
		t.Fatalf("create film: %v", err)
	}
	videoID, err := r.UpsertVideo(ctx, sampleVideo("/m/roles.mkv", "Scene", nil, nil), nil)
	if err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if _, err := r.AttachFilmVideo(ctx, filmID, videoID, nil, false); err != nil {
		t.Fatalf("attach video: %v", err)
	}
	// The video's own people are inherited cast (video_people); ReconcileVideoPeople
	// replaces the whole list per call, so both names are linked together.
	linkPeople(t, r, videoID, "Cast Member", "Director Person")
	castID := personIDByName(t, r, "Cast Member")
	directorID := personIDByName(t, r, "Director Person")

	// Freshly attached, no credited roles yet.
	roles, err := r.FilmPeopleRoles(ctx, filmID)
	if err != nil {
		t.Fatalf("film people roles (empty): %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("film people roles before any credit: got %+v, want empty", roles)
	}

	billing := int64(1)
	if err := r.AddFilmPersonRole(ctx, filmID, directorID, "Director", &billing); err != nil {
		t.Fatalf("add film person role: %v", err)
	}

	roles, err = r.FilmPeopleRoles(ctx, filmID)
	if err != nil {
		t.Fatalf("film people roles: %v", err)
	}
	if len(roles) != 1 || roles[0].PersonID != directorID || roles[0].Role != "Director" ||
		roles[0].BillingOrder == nil || *roles[0].BillingOrder != 1 {
		t.Fatalf("film people roles after add: got %+v", roles)
	}

	// Adding the same person again is a conflict, not a silent upsert.
	if err := r.AddFilmPersonRole(ctx, filmID, directorID, "Producer", nil); !errors.Is(err, repo.ErrFilmPersonAlreadyCredited) {
		t.Fatalf("re-add same person: got %v, want ErrFilmPersonAlreadyCredited", err)
	}

	// Edit changes role text and billing_order in place.
	newBilling := int64(2)
	if err := r.EditFilmPersonRole(ctx, filmID, directorID, "Director/Producer", &newBilling); err != nil {
		t.Fatalf("edit film person role: %v", err)
	}
	roles, err = r.FilmPeopleRoles(ctx, filmID)
	if err != nil {
		t.Fatalf("film people roles after edit: %v", err)
	}
	if len(roles) != 1 || roles[0].Role != "Director/Producer" || roles[0].BillingOrder == nil || *roles[0].BillingOrder != 2 {
		t.Fatalf("film people roles after edit: got %+v", roles)
	}

	// Editing an uncredited person is ErrNotFound.
	if err := r.EditFilmPersonRole(ctx, filmID, castID, "Actor", nil); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("edit uncredited person: got %v, want ErrNotFound", err)
	}

	// The inherited cast union is unaffected by film_people_roles -- both names
	// appear (they're both in video_people), independent of who's credited.
	cast, err := r.FilmCast(ctx, filmID)
	if err != nil {
		t.Fatalf("film cast: %v", err)
	}
	if len(cast) != 2 {
		t.Fatalf("film cast: got %+v, want 2 (unaffected by credited roles)", cast)
	}

	// Remove clears the credited role but never touches inherited cast.
	if err := r.RemoveFilmPersonRole(ctx, filmID, directorID); err != nil {
		t.Fatalf("remove film person role: %v", err)
	}
	roles, err = r.FilmPeopleRoles(ctx, filmID)
	if err != nil {
		t.Fatalf("film people roles after remove: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("film people roles after remove: got %+v, want empty", roles)
	}
	cast, err = r.FilmCast(ctx, filmID)
	if err != nil {
		t.Fatalf("film cast after remove: %v", err)
	}
	if len(cast) != 2 {
		t.Fatalf("film cast after remove: got %+v, want still 2", cast)
	}

	// Removing an uncredited person is ErrNotFound (not idempotent).
	if err := r.RemoveFilmPersonRole(ctx, filmID, directorID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("re-remove: got %v, want ErrNotFound", err)
	}
}

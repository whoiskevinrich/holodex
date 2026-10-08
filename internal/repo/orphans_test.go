package repo_test

import (
	"context"
	"database/sql"
	"testing"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// HOLODEX-535: people, studios and tags share one orphan stamp (migration 0059
// triggers) and one sweep (repo.SweepOrphans).

const longAgo = "2000-01-01T00:00:00Z"

// orphanedAt reads an entity's orphaned_at ("" when NULL).
func orphanedAt(t *testing.T, db *sql.DB, table string, id int64) string {
	t.Helper()
	var v sql.NullString
	if err := db.QueryRow(`SELECT orphaned_at FROM `+table+` WHERE id = ?`, id).Scan(&v); err != nil {
		t.Fatalf("read %s %d orphaned_at: %v", table, id, err)
	}
	return v.String
}

// ageOrphan backdates a stamp past any grace period.
func ageOrphan(t *testing.T, db *sql.DB, table string, id int64) {
	t.Helper()
	if _, err := db.Exec(`UPDATE `+table+` SET orphaned_at = ? WHERE id = ?`, longAgo, id); err != nil {
		t.Fatalf("age %s %d: %v", table, id, err)
	}
}

func exists(t *testing.T, db *sql.DB, table string, id int64) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE id = ?`, id).Scan(&n); err != nil {
		t.Fatalf("count %s %d: %v", table, id, err)
	}
	return n == 1
}

func sweep(t *testing.T, r *repo.Repo) (deleted, skipped int) {
	t.Helper()
	deleted, skipped, err := r.SweepOrphans(context.Background(), 30)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	return deleted, skipped
}

func seedVideo(t *testing.T, r *repo.Repo, path string) int64 {
	t.Helper()
	id, err := r.UpsertVideo(context.Background(), sampleVideo(path, path, nil, nil), nil)
	if err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
	return id
}

// linkedStudio links a fresh video to `name` and returns (videoID, studioID).
func linkedStudio(t *testing.T, r *repo.Repo, path, name string) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	vid := seedVideo(t, r, path)
	if err := r.ReconcileVideoStudios(ctx, vid, []string{name}, nil); err != nil {
		t.Fatalf("link studio: %v", err)
	}
	studios, err := r.ListStudios(ctx, false)
	if err != nil {
		t.Fatalf("list studios: %v", err)
	}
	for _, s := range studios {
		if s.Name == name {
			return vid, s.ID
		}
	}
	t.Fatalf("studio %q not linked", name)
	return 0, 0
}

func TestOrphanStudio_StampSweepAndRelink(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	vid, sid := linkedStudio(t, r, "/m/a.mkv", "Acme")
	if got := orphanedAt(t, db, "studios", sid); got != "" {
		t.Fatalf("linked studio stamped %q", got)
	}

	// Unlinking stamps, and never deletes in the same transaction.
	if err := r.ReconcileVideoStudios(ctx, vid, nil, nil); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if !exists(t, db, "studios", sid) || orphanedAt(t, db, "studios", sid) == "" {
		t.Fatalf("unlinked studio: want kept and stamped")
	}

	// Inside the grace period the sweep leaves it alone; relinking clears the stamp.
	if d, _ := sweep(t, r); d != 0 {
		t.Fatalf("fresh orphan swept (deleted=%d)", d)
	}
	if err := r.ReconcileVideoStudios(ctx, vid, []string{"Acme"}, nil); err != nil {
		t.Fatalf("relink: %v", err)
	}
	if got := orphanedAt(t, db, "studios", sid); got != "" {
		t.Fatalf("relinked studio still stamped %q", got)
	}

	// Past the grace period an unauthored orphan is deleted, enrichment with it.
	if err := r.ReconcileVideoStudios(ctx, vid, nil, nil); err != nil {
		t.Fatalf("unlink again: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO entity_enrichment (entity_type, entity_id, provider, field_key, value, fetched_at)
		VALUES ('studio', ?, 'p', 'overview', 'x', ?)`, sid, longAgo); err != nil {
		t.Fatalf("seed enrichment: %v", err)
	}
	ageOrphan(t, db, "studios", sid)
	if d, s := sweep(t, r); d != 1 || s != 0 {
		t.Fatalf("sweep = (%d deleted, %d skipped), want (1, 0)", d, s)
	}
	if exists(t, db, "studios", sid) {
		t.Fatal("aged unauthored studio survived the sweep")
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM entity_enrichment WHERE entity_type = 'studio' AND entity_id = ?`, sid).Scan(&n); err != nil || n != 0 {
		t.Fatalf("enrichment rows left = %d (err %v), want 0", n, err)
	}
}

// The HOLODEX-535 repro: an owner-aliased studio used to be deleted the moment its
// last video was unlinked.
func TestOrphanStudio_AuthoredSurvives(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	vid, sid := linkedStudio(t, r, "/m/a.mkv", "Acme")
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityStudio, sid, "Acme Pictures"); err != nil {
		t.Fatalf("alias: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, vid, nil, nil); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if _, err := r.GetStudio(ctx, sid); err != nil {
		t.Fatalf("aliased studio gone after unlink: %v", err)
	}
	ageOrphan(t, db, "studios", sid)
	if d, s := sweep(t, r); d != 0 || s != 1 {
		t.Fatalf("sweep = (%d deleted, %d skipped), want (0, 1)", d, s)
	}
	if !exists(t, db, "studios", sid) {
		t.Fatal("aliased studio swept")
	}
}

func TestOrphanStudio_SharedNeverStamped(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	a, sid := linkedStudio(t, r, "/m/a.mkv", "Ghibli")
	b := seedVideo(t, r, "/m/b.mkv")
	if err := r.ReconcileVideoStudios(ctx, b, []string{"Ghibli"}, nil); err != nil {
		t.Fatalf("link b: %v", err)
	}
	if err := r.ReconcileVideoStudios(ctx, a, nil, nil); err != nil {
		t.Fatalf("unlink a: %v", err)
	}
	if got := orphanedAt(t, db, "studios", sid); got != "" {
		t.Fatalf("still-linked studio stamped %q", got)
	}
}

// A stale stamp on a linked entity never gets it deleted: the sweep re-checks links.
func TestOrphanSweep_IgnoresStaleStampOnLinkedEntity(t *testing.T) {
	r, db := newRepoDB(t)
	_, sid := linkedStudio(t, r, "/m/a.mkv", "Acme")
	ageOrphan(t, db, "studios", sid)
	if d, _ := sweep(t, r); d != 0 || !exists(t, db, "studios", sid) {
		t.Fatalf("linked studio with a stale stamp was swept (deleted=%d)", d)
	}
}

func TestOrphanTag_DetachStampsAndSweeps(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	vid := seedVideo(t, r, "/m/a.mkv")
	tag, err := r.AttachTagToVideo(ctx, vid, "noir")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if got := orphanedAt(t, db, "tags", tag.ID); got != "" {
		t.Fatalf("attached tag stamped %q", got)
	}
	if err := r.DetachTagFromVideo(ctx, vid, tag.ID); err != nil {
		t.Fatalf("detach: %v", err)
	}
	if orphanedAt(t, db, "tags", tag.ID) == "" {
		t.Fatal("detached tag not stamped")
	}
	ageOrphan(t, db, "tags", tag.ID)
	if d, s := sweep(t, r); d != 1 || s != 0 {
		t.Fatalf("sweep = (%d deleted, %d skipped), want (1, 0)", d, s)
	}
	if exists(t, db, "tags", tag.ID) {
		t.Fatal("aged unauthored tag survived")
	}
}

func TestOrphanTag_BareCreateIsStamped(t *testing.T) {
	r, db := newRepoDB(t)
	tag, err := r.ResolveOrCreateTag(context.Background(), "bare")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if orphanedAt(t, db, "tags", tag.ID) == "" {
		t.Fatal("bare tag not stamped at creation")
	}
	// Still searchable: the stamp must not disturb the FTS mirror.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tags_fts WHERE tags_fts MATCH 'bare'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("tags_fts match = %d (err %v), want 1", n, err)
	}
}

func TestOrphanTag_AuthoredSurvives(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		author func(t *testing.T, r *repo.Repo, tagID int64)
	}{
		{"alias", func(t *testing.T, r *repo.Repo, id int64) {
			if _, err := r.AddEntityAlias(ctx, model.EntityTag, id, "alt"); err != nil {
				t.Fatal(err)
			}
		}},
		{"category", func(t *testing.T, r *repo.Repo, id int64) {
			c, err := r.CreateCategory(ctx, "mood")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := r.AssignTagsToCategory(ctx, c.ID, []int64{id}); err != nil {
				t.Fatal(err)
			}
		}},
		{"parent", func(t *testing.T, r *repo.Repo, id int64) {
			p, err := r.ResolveOrCreateTag(ctx, "genre")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := r.SetTagParent(ctx, id, &p.ID); err != nil {
				t.Fatal(err)
			}
		}},
		{"child", func(t *testing.T, r *repo.Repo, id int64) {
			c, err := r.ResolveOrCreateTag(ctx, "subgenre")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := r.SetTagParent(ctx, c.ID, &id); err != nil {
				t.Fatal(err)
			}
		}},
		{"writeback excluded", func(t *testing.T, r *repo.Repo, id int64) {
			if _, err := r.SetTagWritebackEnabled(ctx, id, false); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, db := newRepoDB(t)
			tag, err := r.ResolveOrCreateTag(ctx, "noir")
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			tc.author(t, r, tag.ID)
			ageOrphan(t, db, "tags", tag.ID)
			sweep(t, r)
			if !exists(t, db, "tags", tag.ID) {
				t.Fatalf("tag with %s swept", tc.name)
			}
		})
	}
}

// Purge removes links by FK cascade; the triggers still stamp all three kinds
// (people orphaned by a purge were never stamped before 0059).
func TestOrphan_PurgeCascadeStampsAllKinds(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	vid, sid := linkedStudio(t, r, "/m/a.mkv", "Acme")
	tag, err := r.AttachTagToVideo(ctx, vid, "noir")
	if err != nil {
		t.Fatalf("attach tag: %v", err)
	}
	if err := r.ReconcileVideoPeople(ctx, vid, []repo.PersonRoleName{{Name: "Alice", Role: "actor"}}, nil); err != nil {
		t.Fatalf("link person: %v", err)
	}
	pid := personIDByName(t, r, "Alice")

	if err := r.HardDelete(ctx, vid); err != nil {
		t.Fatalf("hard delete: %v", err)
	}
	for _, c := range []struct {
		table string
		id    int64
	}{{"people", pid}, {"studios", sid}, {"tags", tag.ID}} {
		if orphanedAt(t, db, c.table, c.id) == "" {
			t.Errorf("%s %d not stamped by purge cascade", c.table, c.id)
		}
	}
}

// A film credit pins a person: deleting them would cascade the owner-asserted
// film_people_roles row away.
func TestOrphanPerson_FilmCreditSurvives(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	vid := seedVideo(t, r, "/m/a.mkv")
	if err := r.ReconcileVideoPeople(ctx, vid, []repo.PersonRoleName{{Name: "Carol"}}, nil); err != nil {
		t.Fatalf("link: %v", err)
	}
	carol := personIDByName(t, r, "Carol")
	film, err := r.CreateFilm(ctx, "Noir", 1950)
	if err != nil {
		t.Fatalf("film: %v", err)
	}
	if err := r.AddFilmPersonRole(ctx, film, carol, "actor", nil); err != nil {
		t.Fatalf("credit: %v", err)
	}
	if err := r.ReconcileVideoPeople(ctx, vid, nil, nil); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	ageOrphan(t, db, "people", carol)
	if d, s := sweep(t, r); d != 0 || s != 1 || !exists(t, db, "people", carol) {
		t.Fatalf("sweep = (%d deleted, %d skipped), want film-credited person kept", d, s)
	}
}

// A keep-separate pair is an owner decision (kept); a review-queue pair is a
// machine suggestion and goes with the swept entity, since ids can be reused.
func TestOrphanSweep_IdentityPairs(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	kept, err := r.ResolveOrCreateTag(ctx, "kept")
	if err != nil {
		t.Fatal(err)
	}
	gone, err := r.ResolveOrCreateTag(ctx, "gone")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO entity_keep_separate (entity_type, id_lo, id_hi) VALUES ('tag', 0, ?)`, kept.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO identity_review_queue (entity_type, id_lo, id_hi, variation) VALUES ('tag', 0, ?, 'punctuation')`, gone.ID); err != nil {
		t.Fatal(err)
	}
	ageOrphan(t, db, "tags", kept.ID)
	ageOrphan(t, db, "tags", gone.ID)
	if d, s := sweep(t, r); d != 1 || s != 1 {
		t.Fatalf("sweep = (%d deleted, %d skipped), want (1, 1)", d, s)
	}
	if !exists(t, db, "tags", kept.ID) || exists(t, db, "tags", gone.ID) {
		t.Fatal("want keep-separate tag kept, review-queued tag swept")
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM identity_review_queue WHERE entity_type = 'tag' AND id_hi = ?`, gone.ID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("review pairs left = %d (err %v), want 0", n, err)
	}
}

// The stamp is not a completeness input: emptying a tag must not dirty the
// library (cd_tags_au is scoped to name/parent/writeback in 0059).
func TestOrphanTag_StampDoesNotDirtyCompleteness(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	vid := seedVideo(t, r, "/m/a.mkv")
	seedVideo(t, r, "/m/b.mkv")
	tag, err := r.AttachTagToVideo(ctx, vid, "noir")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM completeness_dirty`); err != nil {
		t.Fatalf("reset dirty: %v", err)
	}
	if err := r.DetachTagFromVideo(ctx, vid, tag.ID); err != nil {
		t.Fatalf("detach: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM completeness_dirty WHERE entity_type = 'video' AND entity_id != ?`, vid).Scan(&n); err != nil {
		t.Fatalf("count dirty: %v", err)
	}
	if n != 0 {
		t.Fatalf("orphan stamp dirtied %d unrelated videos, want 0", n)
	}
}

func TestOrphanPerson_AuthoredSurvivesUnauthoredSwept(t *testing.T) {
	r, db := newRepoDB(t)
	ctx := context.Background()
	vid := seedVideo(t, r, "/m/a.mkv")
	if err := r.ReconcileVideoPeople(ctx, vid, []repo.PersonRoleName{{Name: "Alice"}, {Name: "Bob"}}, nil); err != nil {
		t.Fatalf("link: %v", err)
	}
	alice, bob := personIDByName(t, r, "Alice"), personIDByName(t, r, "Bob")
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityPerson, alice, "Ally"); err != nil {
		t.Fatalf("alias: %v", err)
	}
	if err := r.ReconcileVideoPeople(ctx, vid, nil, nil); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	ageOrphan(t, db, "people", alice)
	ageOrphan(t, db, "people", bob)
	if d, s := sweep(t, r); d != 1 || s != 1 {
		t.Fatalf("sweep = (%d deleted, %d skipped), want (1, 1)", d, s)
	}
	if !exists(t, db, "people", alice) || exists(t, db, "people", bob) {
		t.Fatal("want aliased Alice kept, unauthored Bob deleted")
	}
}

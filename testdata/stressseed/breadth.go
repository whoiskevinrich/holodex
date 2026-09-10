package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image/color"

	"holodex/internal/repo"
)

// The breadth axis (HOLODEX-350), and the one dimension that is not in the
// ladder table.
//
// Depth and breadth find different bugs. The ladder tortures one entity until a
// layout breaks; breadth leaves every entity friendly and asks what happens when
// there are two thousand of them. A 50-person cast finds a wrapping bug on one
// page; two thousand people find that /people has no pagination at all.
//
// It is deliberately *not* a dimension. A dimension addresses its entities so an
// assertion can name one, and the whole point here is that no individual bulk
// entity matters — only how many there are. So breadth entities are pool
// entities: unaddressed, above poolBase, outside every reserved block, and
// recorded in the manifest as a range and a count rather than one by one.
//
// D3 (one factor at a time) applies to the breadth axis itself, which is why
// every bulk entity is aggressively neutral: one person, one studio and one tag
// per video, a well-formed mid-tone image, a short plain name. If /people is slow
// at 2000 rows, the only variable that could have caused it is 2000.

// bulkPrefix distinguishes a breadth entity from a ladder pool entity ("stress
// person 001") at a glance, in the database and on the page. Both are supporting
// cast; only one of them is supposed to be numerous.
const bulkPrefix = "stress bulk"

// categoriesTable is the one seeded table with no entityKind behind it. A
// category has a page and a list, but no dimension addresses one — there is
// nothing about a category to torture that a tag does not already cover — so it
// exists only in the breadth pool, and only needs its table name.
const categoriesTable = "categories"

// imageNeutral is the breadth pool's image: correctly framed, correctly sized,
// mid-tone against every skin. It is declared here rather than in imagePalette
// because it is the opposite of a rung — the palette exists to break things, and
// this exists to break nothing, so that a slow grid is attributable to the number
// of tiles rather than to what is in them.
//
// Bulk videos are given one at all (rather than left imageless, which would be
// cheaper) because the browse grid is a poster grid: timing a grid of empty
// frames would answer a question nobody asked. The AC wants numbers for the perf
// conversation, and a number measured without image decode is not one.
var imageNeutral = imageVariant{
	key:   "neutral",
	value: "well-formed mid-tone plate",
	plate: color.NRGBA{R: 58, G: 64, B: 78, A: 255},
	ink:   color.NRGBA{R: 232, G: 236, B: 244, A: 255},
}

// idRange is where one table's bulk rows landed. Two numbers and a count, because
// that is the whole of what a breadth assertion can meaningfully say.
type idRange struct {
	Lo int64 `json:"lo"`
	Hi int64 `json:"hi"`
	N  int   `json:"n"`
}

// breadthPool is what a run populated, keyed by table name. It is reported and
// written to the manifest so the fixture states its own scale rather than
// leaving the reader to count rows.
type breadthPool struct {
	Count  int                `json:"count"`
	Tables map[string]idRange `json:"tables"`
}

func newBreadthPool(count int) *breadthPool {
	return &breadthPool{Count: count, Tables: map[string]idRange{}}
}

// record extends a table's range. Called per row rather than per batch so the
// range is what actually landed, not what was intended.
func (b *breadthPool) record(table string, id int64) {
	r, seen := b.Tables[table]
	if !seen {
		r = idRange{Lo: id, Hi: id}
	}
	r.Lo = min(r.Lo, id)
	r.Hi = max(r.Hi, id)
	r.N++
	b.Tables[table] = r
}

// bulkName names one breadth entity. Zero-padded to four digits so a list sorts
// in creation order rather than lexicographically scrambled (…1, 10, 100, 2),
// and lowercase because the tag writer folds to lowercase anyway — a seeded name
// and a read-back name should be the same string.
//
// Four digits rather than a width derived from count: a fixture seeded at 100 and
// one seeded at 2000 then name their first entity identically, so a note or an
// assertion written against one scale still reads correctly at the other.
func bulkName(kind string, i int) string {
	return fmt.Sprintf("%s %s %04d", bulkPrefix, kind, i+1)
}

// breadthCeiling is the largest -count the pool range can hold.
//
// Bulk people, studios and tags share the [poolBase, derivedBase) gap with the
// supporting entities the ladder itself creates, and the derived kinds' reserved
// blocks start at the top of it. Overrun it and steer() refuses — correctly, but
// only after the run has written every one of those rows. Checking up front turns
// a slow failure into an immediate one that can state the actual limit.
func breadthCeiling() int {
	d := demands(ladder)
	// The ladder's own pool draw, per kind. People is the largest in practice, but
	// taking the max keeps the ceiling correct if a future rung changes that.
	ladderPool := max(d.people, d.studios, d.tags, d.cast, d.scenes)
	return int(derivedBase-poolBase) - ladderPool
}

// seedBreadthVideos populates the video half of the breadth pool: count videos,
// each carrying exactly one person, one studio and one tag, all distinct. That
// 1:1 shape is what makes the pool count of every one of those four kinds equal
// to count, without a second knob deciding how they are distributed.
//
// It runs at the same moment as the scene pool — the one point where the videos
// sequence has been steered out of the addressed range and the people, studio and
// tag sequences have not yet been steered up to derivedBase.
func seedBreadthVideos(ctx context.Context, r *repo.Repo, ff fixtureFields, images imageWriter, count int, pool *breadthPool) ([]int64, error) {
	if count > breadthCeiling() {
		return nil, fmt.Errorf("-count %d exceeds %d, the most the pool range [%d,%d) can hold.\n"+
			"Bulk people, studios and tags share that gap with the ladder's own supporting\n"+
			"entities, and the derived kinds' reserved blocks start at %d",
			count, breadthCeiling(), poolBase, derivedBase, derivedBase)
	}

	ids := make([]int64, 0, count)
	for i := range count {
		l := links{
			cast:    []string{bulkName("person", i)},
			studios: []string{bulkName("studio", i)},
			tags:    []string{bulkName("tag", i)},
		}
		name := bulkName("video", i)
		id, err := upsertVideo(ctx, r, ff, fmt.Sprintf("/stress/bulk/%04d.mp4", i+1), name, l, name)
		if err != nil {
			return nil, fmt.Errorf("bulk video %d: %w", i+1, err)
		}
		if err := assertPooled(kindVideo, id); err != nil {
			return nil, err
		}
		if err := images.seed(ctx, kindVideo, id, imageNeutral); err != nil {
			return nil, fmt.Errorf("bulk video %d images: %w", i+1, err)
		}
		pool.record(kindVideo.table(), id)
		ids = append(ids, id)
	}
	return ids, nil
}

// recordBulkDerived finds the people, studios and tags the bulk videos brought
// into existence and records where they landed.
//
// They are looked up rather than assumed: all three kinds are derived (the
// reconcile over a video's resolved file layer creates them), so the seeder never
// sees their ids. A name that resolved to an existing entity instead of a new one
// would silently shrink the pool, which is exactly the kind of miscount a breadth
// assertion would then be written against.
//
// The lookup is LookupEntityIDByName rather than PersonIDByName/TagIDByName
// because it resolves the way the reconcile that created these rows resolves —
// canonical name key, then alias — where the bare helpers are a plain
// case-insensitive name match that never consults entity_aliases. The three
// entityKind values are the entity-type strings that function takes.
func recordBulkDerived(ctx context.Context, r *repo.Repo, count int, pool *breadthPool) error {
	for _, kind := range []entityKind{kindPerson, kindStudio, kindTag} {
		for i := range count {
			name := bulkName(string(kind), i)
			id, ok, err := r.LookupEntityIDByName(ctx, string(kind), name)
			if err != nil {
				return fmt.Errorf("look up %q: %w", name, err)
			}
			if !ok {
				return fmt.Errorf("bulk %s %q does not exist; the reconcile over the bulk "+
					"videos' file layer was meant to create it", kind, name)
			}
			if err := assertPooled(kind, id); err != nil {
				return err
			}
			pool.record(kind.table(), id)
		}
	}
	return nil
}

// seedBreadthFilms populates count films, each attaching one bulk video as its
// only scene.
//
// It runs after the whole ladder walk, not alongside the bulk videos: the film
// dimensions address films in blocks below poolBase, and a bulk film created
// before them would eat the addresses they are steered into.
//
// One scene each rather than none, because a film's cast, studios and tags are
// all derived from its attached videos — a film with no scenes renders as an
// empty page and a bare tile, so a list of two thousand of them would measure
// less than the real thing.
func seedBreadthFilms(ctx context.Context, database *sql.DB, r *repo.Repo, count int, videoIDs []int64, pool *breadthPool) error {
	if count == 0 {
		return nil
	}
	if len(videoIDs) < count {
		return fmt.Errorf("bulk films need %d bulk videos to attach as scenes, but only %d exist",
			count, len(videoIDs))
	}
	if err := steer(ctx, database, kindFilm.table(), poolBase); err != nil {
		return err
	}
	for i := range count {
		name := bulkName("film", i)
		// Year 0 stores SQL NULL, for the reason materializeFilm gives: the name is
		// the whole key, so a year would be a second identity axis for no benefit.
		id, err := r.CreateFilm(ctx, name, 0)
		if err != nil {
			if errors.Is(err, repo.ErrFilmExists) {
				return fmt.Errorf("bulk film name %q is already taken by film %d; reset() "+
					"should have emptied the table", name, id)
			}
			return fmt.Errorf("create bulk film %d: %w", i+1, err)
		}
		if err := assertPooled(kindFilm, id); err != nil {
			return err
		}
		scene := int64(1)
		if _, err := r.AttachFilmVideo(ctx, id, videoIDs[i], &scene, false); err != nil {
			return fmt.Errorf("attach scene to bulk film %d: %w", i+1, err)
		}
		pool.record(kindFilm.table(), id)
	}
	return nil
}

// seedBreadthCategories populates count categories, each holding one bulk tag.
//
// Categories are the one entity the ladder does not address at all, so this is
// the fixture's only coverage of them — /tags renders the whole category list
// beside the whole tag list, and /categories/{id} renders its members in full.
//
// A category cannot share a name with a tag (migration 0035's collision
// triggers), which the bulk naming already satisfies: "stress bulk category 0001"
// and "stress bulk tag 0001" differ.
func seedBreadthCategories(ctx context.Context, database *sql.DB, r *repo.Repo, count int, pool *breadthPool) error {
	if count == 0 {
		return nil
	}
	if err := steer(ctx, database, categoriesTable, poolBase); err != nil {
		return err
	}
	for i := range count {
		name := bulkName("category", i)
		cat, err := r.CreateCategory(ctx, name)
		if err != nil {
			return fmt.Errorf("create bulk category %d: %w", i+1, err)
		}
		if cat.ID < poolBase {
			return fmt.Errorf("bulk category %d landed at id %d, below the pool base %d",
				i+1, cat.ID, poolBase)
		}
		tagName := bulkName("tag", i)
		tid, ok, err := r.TagIDByName(ctx, tagName)
		if err != nil {
			return fmt.Errorf("look up %q: %w", tagName, err)
		}
		if !ok {
			return fmt.Errorf("bulk tag %q does not exist; the bulk videos were meant to "+
				"create it", tagName)
		}
		if _, err := r.AssignTagsToCategory(ctx, cat.ID, []int64{tid}); err != nil {
			return fmt.Errorf("assign %q to bulk category %d: %w", tagName, i+1, err)
		}
		pool.record(categoriesTable, cat.ID)
	}
	return nil
}

// assertPooled is the breadth half of the contract every pool entity carries: it
// lives above poolBase, outside every reserved block, and below the derived
// blocks that start at derivedBase.
//
// Checked per row rather than trusted, for the same reason seedScenePool checks
// its own: a steer that silently did nothing would scatter bulk entities through
// a dimension's block, and nothing downstream would notice — the ids would still
// look plausible.
func assertPooled(kind entityKind, id int64) error {
	if id < poolBase {
		return fmt.Errorf("bulk %s landed at id %d, inside the addressed range below %d",
			kind, id, poolBase)
	}
	if kind.derived() && id >= derivedBase {
		return fmt.Errorf("bulk %s landed at id %d, inside the derived blocks at %d; "+
			"-count is too large for the pool range", kind, id, derivedBase)
	}
	return nil
}

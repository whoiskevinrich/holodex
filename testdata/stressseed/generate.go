package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// seededTables are the tables the fixture owns outright. Every run clears them
// before generating, which is what makes the fixture reproducible rather than
// merely idempotent: an upsert-in-place run would leave rows from a rung that
// has since been deleted, and an interrupted run would leave half a fixture
// behind with no way to tell. Clearing is safe here and nowhere else — the
// caller has already proved this database is the fixture's own (see claim.go).
//
// Ordered parents-last: videos first sheds most of the link rows by cascade.
//
// identity_review_queue is in the list despite holding no fixture entity of its
// own. Every entity created through resolveOrCreateByName is run past
// FlagNearMiss, which files a suggested merge here — and the table stores bare
// (entity_type, id_lo, id_hi) integers with no foreign key, so nothing cascades
// when the entities are deleted. Leave it and a row filed against ids 20001/20003
// outlives the palette that produced them, and reappears on the owner's
// duplicates page pointing at whatever now holds those addresses.
//
// entity_enrichment is here for exactly the same reason as identity_review_queue,
// and it is load-bearing rather than tidy: migration 0005 gives the table NO
// foreign key — it stores bare (entity_type, entity_id) integers — so nothing
// cascades when a video, person or studio is deleted. Without this entry the rows
// simply outlive the entities they described, and the next seed hands their ids to
// new entities, which inherit a chip row nothing in the ladder put there at an
// address the manifest describes as un-enriched. That covers both the ladder's own
// rows (HOLODEX-348) and any an operator created by enriching through the stub
// during a QA pass.
var seededTables = []string{"videos", "films", "people", "studios", "tags", "categories", "identity_review_queue", "entity_enrichment"}

// generate builds every dimension in the ladder and returns what it addressed.
//
// The walk is one-factor-at-a-time (D3): each rung starts from the neutral
// baseline, mutates exactly one axis, and becomes exactly one entity. There is
// deliberately no nesting here — a cross-product would be a loop inside this
// loop, and its absence is the design.
func generate(ctx context.Context, database *sql.DB, r *repo.Repo, ff fixtureFields, targets imageTargets, count int) ([]entry, *breadthPool, error) {
	if err := validateLadder(ladder); err != nil {
		return nil, nil, err
	}
	if err := reset(ctx, database); err != nil {
		return nil, nil, err
	}
	// The database half of the clear cascades from reset()'s entity tables; the
	// files do not cascade from anything, and a stale one would make the `missing`
	// rung show a working image. See clearImages.
	if err := clearImages(targets); err != nil {
		return nil, nil, err
	}
	images := imageWriter{repo: r, targets: targets}

	// Supporting entities are steered once, into the pool range above every
	// dimension block, so a person created to satisfy a cardinality rung can
	// never land on an address an assertion was written against.
	for _, table := range []string{"people", "studios", "tags"} {
		if err := steer(ctx, database, table, poolBase); err != nil {
			return nil, nil, err
		}
	}

	var entries []entry
	var scenePool, bulkVideos []int64
	pool := newBreadthPool(count)
	poolSeeded := false
	for _, dim := range ladder {
		// Everything that is not an addressed video needs the videos sequence moved
		// out of the addressed range first — the scene pool and the derived kinds'
		// carrier videos alike. It cannot happen earlier: SQLite's AUTOINCREMENT
		// counter only moves forward, and rewinding it below rows that already
		// exist would collide rather than renumber. validateLadder guarantees every
		// video dimension comes first, so the first non-video dimension is the one
		// point where both conditions hold.
		//
		// The breadth pool's videos ride along here for the same reason, plus one of
		// their own: bulk people, studios and tags have to be created while those
		// sequences are still in the [poolBase, derivedBase) gap, before the derived
		// dimensions steer them up to derivedBase. See breadth.go.
		if dim.entity != kindVideo && !poolSeeded {
			poolSeeded = true
			if err := steer(ctx, database, kindVideo.table(), poolBase); err != nil {
				return nil, nil, err
			}
			var err error
			if scenePool, err = seedScenePool(ctx, r, ff, demands(ladder).scenes); err != nil {
				return nil, nil, fmt.Errorf("scene pool: %w", err)
			}
			if bulkVideos, err = seedBreadthVideos(ctx, r, ff, images, count, pool); err != nil {
				return nil, nil, fmt.Errorf("breadth videos: %w", err)
			}
			if err := recordBulkDerived(ctx, r, count, pool); err != nil {
				return nil, nil, fmt.Errorf("breadth pool: %w", err)
			}
		}
		if err := steer(ctx, database, dim.entity.table(), dim.block); err != nil {
			return nil, nil, err
		}
		for _, rg := range dim.rungs {
			s := baseline()
			rg.apply(&s)

			id, err := materialize(ctx, r, ff, dim, rg, s, scenePool)
			if err != nil {
				return nil, nil, fmt.Errorf("%s=%s: %w", dim.key, rg.variant, err)
			}
			// The block is only a real address if nothing escapes it. Steering
			// puts the first entity in the right place; this catches the case
			// where a dimension has outgrown the block it declared, which would
			// otherwise silently overwrite the next dimension's addresses.
			if id < dim.block || id >= dim.block+blockSize {
				return nil, nil, fmt.Errorf(
					"dimension %q overflowed its reserved block: %s landed at id %d, outside [%d,%d).\n"+
						"Give it a larger block or move the dimensions above it",
					dim.key, rg.variant, id, dim.block, dim.block+blockSize)
			}

			// Unconditional, and a no-op for every dimension that is not an image one:
			// the baseline carries imageNone, so only an image rung has anything to
			// write. Done here rather than inside materialize because this is the one
			// place that holds the id and the spec at the same time, for all five kinds.
			if err := images.seed(ctx, dim.entity, id, s.image); err != nil {
				return nil, nil, fmt.Errorf("%s=%s images: %w", dim.key, rg.variant, err)
			}

			entries = append(entries, entry{
				ID:        id,
				Entity:    dim.entity,
				Dimension: dim.key,
				Variant:   rg.variant,
				Value:     rg.value,
				Name:      encodeName(dim.entity, s),
				URL:       dim.entity.urlFor(id),
				Axes:      axesOf(dim, s, ff.enrich.fields),
			})
		}
	}

	// A ladder of nothing but video dimensions never reaches the block above, so
	// the pool would silently not exist. Real ladders always have a non-video
	// dimension; test ones need not, and a breadth pool that is quietly absent is
	// worse than one that is late.
	if !poolSeeded {
		if err := steer(ctx, database, kindVideo.table(), poolBase); err != nil {
			return nil, nil, err
		}
		var err error
		if bulkVideos, err = seedBreadthVideos(ctx, r, ff, images, count, pool); err != nil {
			return nil, nil, fmt.Errorf("breadth videos: %w", err)
		}
		if err := recordBulkDerived(ctx, r, count, pool); err != nil {
			return nil, nil, fmt.Errorf("breadth pool: %w", err)
		}
	}

	// Films and categories come last, after every film dimension has been placed.
	// A bulk film created alongside the bulk videos would consume the addresses the
	// film dimensions are steered into, because films are addressed in blocks below
	// poolBase — the one kind whose breadth half cannot share the videos' moment.
	if err := seedBreadthFilms(ctx, database, r, count, bulkVideos, pool); err != nil {
		return nil, nil, fmt.Errorf("breadth films: %w", err)
	}
	if err := seedBreadthCategories(ctx, database, r, count, pool); err != nil {
		return nil, nil, fmt.Errorf("breadth categories: %w", err)
	}
	return entries, pool, nil
}

// validateLadder enforces the invariants the table's readers assume. They are
// checked rather than trusted because the failure mode is silent: two dimensions
// sharing a block do not error, they overwrite each other's addresses.
func validateLadder(dims []dimension) error {
	blocks := map[int64]string{}
	highest := map[entityKind]int64{}
	sawOther, sawOtherKey := false, ""
	var sawOtherKind entityKind
	for _, dim := range dims {
		if dim.entity.table() == "" || dim.entity.urlFor(1) == "" {
			return fmt.Errorf("dimension %q addresses entity kind %q, which has no table or route mapping",
				dim.key, dim.entity)
		}
		if len(dim.rungs) == 0 {
			return fmt.Errorf("dimension %q declares no rungs, so it would reserve a block "+
				"and contribute nothing to the fixture", dim.key)
		}
		if len(dim.rungs) > blockSize {
			return fmt.Errorf("dimension %q declares %d rungs but a block holds %d",
				dim.key, len(dim.rungs), blockSize)
		}
		lo, hi := dim.entity.addressSpace()
		if dim.block < lo || dim.block+blockSize > hi {
			return fmt.Errorf("dimension %q block %d is outside the addressable range "+
				"[%d,%d) for a %s", dim.key, dim.block, lo, hi, dim.entity)
		}
		if other, taken := blocks[dim.block]; taken {
			return fmt.Errorf("dimensions %q and %q both claim block %d", other, dim.key, dim.block)
		}
		blocks[dim.block] = dim.key

		// Table order is load-bearing for dimensions sharing an entity kind:
		// generate() steers one AUTOINCREMENT counter per kind, in table order,
		// and steering it backwards over rows that already exist places the next
		// entity somewhere neither the block nor the overflow check predicts.
		if prev, seen := highest[dim.entity]; seen && dim.block <= prev {
			return fmt.Errorf("dimension %q declares block %d after a %s dimension at %d; "+
				"blocks must ascend within an entity kind, because they share one ID sequence",
				dim.key, dim.block, dim.entity, prev)
		}
		highest[dim.entity] = dim.block

		// Every video dimension must precede every dimension of any other kind.
		// generate() steers the videos sequence into the pool range at the first
		// non-video dimension — for the scene pool, and for the carrier videos the
		// derived kinds need — and an AUTOINCREMENT counter cannot be rewound below
		// rows that already exist, so a video dimension after one of those would
		// try to steer backwards and collide instead of landing in its block.
		// Tracked as a bool rather than by testing the remembered key against "":
		// a dimension whose key was left empty would make an empty sentinel
		// indistinguishable from "none seen yet", silently disabling the one guard
		// standing between a mis-ordered table and corrupted addresses.
		if dim.entity == kindVideo && sawOther {
			return fmt.Errorf("video dimension %q is declared after the %s dimension %q; "+
				"all video dimensions must come first, because the videos sequence is "+
				"steered into the pool range once the non-video half starts",
				dim.key, sawOtherKind, sawOtherKey)
		}
		if dim.entity != kindVideo {
			sawOther, sawOtherKey, sawOtherKind = true, dim.key, dim.entity
		}

		// D2's only escape hatch, and it has to be argued for in the table.
		if dim.noEmptyRung == "" && !hasEmptyRung(dim) {
			return fmt.Errorf("dimension %q has no empty rung and gives no reason; the zero "+
				"case is half the layout bug class (spec D2). Add the rung, or set "+
				"noEmptyRung to why the app cannot reach that state either", dim.key)
		}
		if dim.noEmptyRung != "" && hasEmptyRung(dim) {
			return fmt.Errorf("dimension %q has an empty rung but also claims it cannot: %q",
				dim.key, dim.noEmptyRung)
		}

		seen := map[string]bool{}
		for _, rg := range dim.rungs {
			if seen[rg.variant] {
				return fmt.Errorf("dimension %q repeats the variant %q", dim.key, rg.variant)
			}
			seen[rg.variant] = true
		}
	}

	// The film cast is drawn from the person pool the video people ladder creates,
	// because a person with no video link would be orphan-stamped by the very
	// reconcile that maintains the people table. So the cast ladder cannot reach
	// higher than the people ladder does — and it must fail here, at the table,
	// rather than as a per-rung "person not found" once seeding is already underway.
	if d := demands(dims); d.cast > d.people {
		return fmt.Errorf("the film cast ladder reaches %d but the video people ladder only "+
			"reaches %d, and film cast is drawn from the people that ladder creates.\n"+
			"Raise the top people rung to at least %d, or lower the top cast rung", d.cast, d.people, d.cast)
	}
	return nil
}

// materialize turns one spec into one entity and its links, and returns the id it
// was addressed at.
func materialize(ctx context.Context, r *repo.Repo, ff fixtureFields, dim dimension, rg rung, s spec, scenePool []int64) (int64, error) {
	switch {
	case dim.entity.derived():
		return materializeNamed(ctx, r, ff, dim, rg, s)
	case dim.entity == kindFilm:
		return materializeFilm(ctx, r, ff, s, scenePool)
	default:
		return materializeVideo(ctx, r, ff, dim, rg, s)
	}
}

// materializeVideo builds one addressed video rung.
func materializeVideo(ctx context.Context, r *repo.Repo, ff fixtureFields, dim dimension, rg rung, s spec) (int64, error) {
	// The path is stable and unique per rung. It never points at a real file — the
	// seeder bypasses the scanner by design (D1) — but it is what UpsertVideo
	// identifies a row by, so it has to be derived from the address.
	id, err := upsertVideo(ctx, r, ff, fmt.Sprintf("/stress/%s/%s.mp4", dim.key, rg.variant),
		title(dim, s), poolLinks(s), s.text.value)
	if err != nil {
		return 0, err
	}

	// The enrichment axis is written after the video exists, because a shadow-store
	// row is keyed by the entity id. It is the one axis that is not a link and not a
	// column — it is the ADR-090 precedence layer, seeded so the chip row is there
	// on boot rather than after somebody remembers to enrich five times.
	if s.namespaces > 0 {
		if err := seedEnrichment(ctx, r, ff.enrich, id, s.namespaces); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// materializeNamed builds one addressed person, studio or tag: the entity whose
// *name* is the rung.
//
// Each rung seeds its own carrier video, because nothing in the repo creates one
// of these from a name alone — a person and a studio are reconciled from a
// video's resolved file layer, a tag is attached to a video. The carrier is a
// pool entity: it exists so the addressed entity can, carries only the one entity
// under test, and is deliberately not addressed itself.
//
// One carrier per rung rather than one per dimension, so exactly one row enters
// the table per rung and the block's addresses stay in rung order.
func materializeNamed(ctx context.Context, r *repo.Repo, ff fixtureFields, dim dimension, rg rung, s spec) (int64, error) {
	// The same rule the video half uses: the entity's name is the raw variant when
	// this dimension is the one torturing names, and the coordinate otherwise. An
	// image dimension needs the coordinate — every rung shares the baseline text, so
	// naming them all from it would fold the whole dimension into one entity via
	// resolveOrCreateByName.
	name := title(dim, s)

	var l links
	switch dim.entity {
	case kindPerson:
		l.cast = []string{name}
	case kindStudio:
		l.studios = []string{name}
	case kindTag:
		// Deliberately not l.tags: the repo has no per-video tag read-back, so a tag
		// attached inside upsertVideo would have to be attached a second time just to
		// learn its id. bindNamedEntity does the one attach and keeps the row it
		// returns — one write, and the code stops calling a write a lookup.
	default:
		return 0, fmt.Errorf("no carrier shape for derived kind %q", dim.entity)
	}

	// The carrier's title is the coordinate, not the variant: it is the one page in
	// this dimension that is *not* the thing under test, so it should say so rather
	// than wear the tortured name too. It says "carrier" out loud because on an
	// image dimension the addressed entity is named from the same coordinate, and
	// two pages showing the identical heading would be a fixture that cannot tell
	// you which one you are looking at.
	carrier, err := upsertVideo(ctx, r, ff,
		fmt.Sprintf("/stress/%s/%s.mp4", dim.key, rg.variant),
		encodeName(dim.entity, s)+" carrier", l, "")
	if err != nil {
		return 0, fmt.Errorf("carrier video: %w", err)
	}
	// A carrier landing inside an addressed block would mean the videos sequence
	// was never steered out of the addressed range — which can only happen if the
	// ladder put a derived dimension before the video ones, so fail here rather
	// than hand out an address that belongs to something else.
	if carrier < poolBase {
		return 0, fmt.Errorf("carrier video landed at id %d, inside the addressed range "+
			"below %d — a derived dimension has been declared before the video ones",
			carrier, poolBase)
	}
	return bindNamedEntity(ctx, r, dim.entity, carrier, name)
}

// bindNamedEntity returns the addressed entity, and for a tag creates it.
//
// The asymmetry is the ADR-075 D3 one, the same split upsertVideo already makes:
// a person and a studio are *derived* from the carrier's file layer, so they
// already exist by now and reading them back is the only proof the derivation ran
// at all — a miss here is HOLODEX-344's wiped links caught one step earlier. A tag
// is *authored*, so it has to be attached, and that attach is what returns the row.
func bindNamedEntity(ctx context.Context, r *repo.Repo, kind entityKind, carrier int64, name string) (int64, error) {
	switch kind {
	case kindPerson:
		id, ok, err := r.PersonIDByName(ctx, name)
		if err != nil {
			return 0, fmt.Errorf("look up person %q: %w", name, err)
		}
		if !ok {
			return 0, fmt.Errorf("no person named %q after seeding its carrier — the "+
				"reconcile dropped the name rather than creating it", name)
		}
		return id, nil

	case kindStudio:
		// There is no StudioIDByName, so the carrier's own link list is the lookup —
		// which also asserts the reconcile produced exactly one studio rather than
		// splitting the name into several.
		byVideo, err := r.StudiosForVideos(ctx, []int64{carrier})
		if err != nil {
			return 0, fmt.Errorf("look up studios of carrier %d: %w", carrier, err)
		}
		got := byVideo[carrier]
		if len(got) != 1 {
			return 0, fmt.Errorf("carrier %d resolved to %d studios, want exactly 1 named %q",
				carrier, len(got), name)
		}
		if got[0].Name != name {
			return 0, fmt.Errorf("carrier %d resolved to studio %q, want %q — the name was "+
				"rewritten on the way in", carrier, got[0].Name, name)
		}
		return got[0].ID, nil

	case kindTag:
		tag, err := r.AttachTagToVideo(ctx, carrier, name)
		if err != nil {
			return 0, fmt.Errorf("attach tag %q: %w", name, err)
		}
		// Tags are lowercased on the way in (curationNorm). namePalette lowercases
		// them first so the two agree; if they ever stop agreeing, the manifest would
		// name a tag that does not exist.
		if tag.Name != name {
			return 0, fmt.Errorf("tag stored as %q but the ladder seeded %q — namePalette "+
				"and the repo's tag normalisation have diverged", tag.Name, name)
		}
		return tag.ID, nil

	default:
		return 0, fmt.Errorf("no lookup for derived kind %q", kind)
	}
}

// links are the entity names a video carries. They are passed in rather than
// derived from the spec inside upsertVideo, because the two kinds of video want
// opposite things from the same writer: a cardinality rung wants n interchangeable
// supporting entities, while a derived rung's carrier wants exactly one entity
// with a specific, deliberately awful name. Both still go in through one function,
// so a link written one way here and another way there stays impossible.
type links struct {
	cast    []string
	studios []string
	tags    []string
}

// poolLinks is what a cardinality rung asks for: n supporting entities per axis.
func poolLinks(s spec) links {
	return links{
		cast:    poolNames("person", s.people),
		studios: poolNames("studio", s.studios),
		tags:    poolNames("tag", s.tags),
	}
}

// upsertVideo writes one video, its file layer, and every relationship it
// carries. Shared by the addressed rungs, the scene pool and the derived kinds'
// carriers.
func upsertVideo(ctx context.Context, r *repo.Repo, ff fixtureFields, filePath, name string, l links, overview string) (int64, error) {
	cast := l.cast
	studios := l.studios

	// People and studios go in as file-layer tags, which is what makes them
	// survive: the server re-derives video_people and video_studios from these on
	// startup and on every relink. See filelayer.go for what happened when they
	// did not.
	extra, err := ff.person.linkTags(cast)
	if err != nil {
		return 0, err
	}
	studioTags, err := ff.studio.linkTags(studios)
	if err != nil {
		return 0, err
	}
	extra = append(extra, studioTags...)

	// The overview carries the same text variant as the title, so one rung tortures
	// both of the video's free-text fields at once. That is still one axis — the
	// variant — not two: it is the same knob reaching a second container, which has
	// its own clamp and its own wrapping behaviour.
	//
	// The empty rung writes no row at all rather than an empty one. The resolver
	// would drop an empty value anyway (firstNonEmpty), so the two are
	// indistinguishable on the page; writing nothing is what "absent" actually
	// means, and leaves no empty metadata row for a later rescan to reason about.
	if overview != "" {
		extra = append(extra, model.ExtraMetadata{SourceKey: ff.overview.fileKey, Value: overview})
	}

	now := time.Now().UTC()
	id, err := r.UpsertVideo(ctx, &model.Video{
		FilePath:  filePath,
		Title:     name,
		FileSize:  1 << 20,
		Duration:  600,
		Width:     1920,
		Height:    1080,
		Container: "mp4",
		IndexedAt: now,
		FileMtime: now,
	}, extra)
	if err != nil {
		return 0, fmt.Errorf("upsert video: %w", err)
	}

	// Reconciling here too, rather than leaving the links for the server's startup
	// backfill, so the fixture is complete the moment seeding finishes instead of
	// only after it has been served once. This is not a second source of truth: it
	// is the same names the derivation will resolve from the tags just written, so
	// the backfill re-derives an identical set and changes nothing. One call per
	// entity kind, not one per name — both APIs are a full replace, so a loop would
	// leave only the last link standing.
	people := make([]repo.PersonRoleName, 0, len(cast))
	for _, person := range cast {
		people = append(people, repo.PersonRoleName{Name: person, Role: ff.person.role})
	}
	if err := r.ReconcileVideoPeople(ctx, id, people, nil); err != nil {
		return 0, fmt.Errorf("link %d people: %w", len(cast), err)
	}
	if err := r.ReconcileVideoStudios(ctx, id, studios, nil); err != nil {
		return 0, fmt.Errorf("link %d studios: %w", len(studios), err)
	}

	// Tags are the exception: they are authored, not derived (ADR-075 D3), so there
	// is no file tag to write and no reconcile to agree with. AttachTagToVideo is
	// one row at a time because that is the only API — it is additive rather than a
	// replace, so the loop is correct here where it would be a bug above.
	for _, tag := range l.tags {
		if _, err := r.AttachTagToVideo(ctx, id, tag); err != nil {
			return 0, fmt.Errorf("attach tag %q: %w", tag, err)
		}
	}
	return id, nil
}

// materializeFilm builds one film, attaches its scenes from the pool, and credits
// its cast.
//
// Neither relationship is derived: film_videos and film_people_roles are asserted
// owner links with no reconciler at all (ADR-085 section 2), so unlike the video
// half there is no file layer to write and nothing that could re-derive these away.
func materializeFilm(ctx context.Context, r *repo.Repo, ff fixtureFields, s spec, scenePool []int64) (int64, error) {
	name := encodeName(kindFilm, s)

	// Year 0 stores SQL NULL. Films are addressed by name here, and CreateFilm
	// resolves-or-creates on (name, year) — so a year would be a second identity
	// axis for no benefit, while NULL leaves the name as the whole key.
	id, err := r.CreateFilm(ctx, name, 0)
	if err != nil {
		// A pre-existing film is normally a benign resolve-to-existing, but reset()
		// has just emptied the table: here it can only mean two rungs encoded the
		// same name, which would silently fuse two addresses into one entity.
		if errors.Is(err, repo.ErrFilmExists) {
			return 0, fmt.Errorf("film name %q is already taken by film %d — two rungs "+
				"encode the same (cast, scenes) coordinate", name, id)
		}
		return 0, fmt.Errorf("create film: %w", err)
	}

	if s.scenes > len(scenePool) {
		return 0, fmt.Errorf("needs %d scenes but the pool holds %d", s.scenes, len(scenePool))
	}
	for i := range s.scenes {
		// Scene numbers are 1-based and contiguous, so the order the film page
		// renders is the order the manifest implies. A nil scene number would never
		// collide (the UNIQUE relies on SQL NULL-distinctness) but would also stop
		// the dimension exercising the scene badge at all.
		scene := int64(i + 1)
		if _, err := r.AttachFilmVideo(ctx, id, scenePool[i], &scene, false); err != nil {
			return 0, fmt.Errorf("attach scene %d: %w", scene, err)
		}
	}

	for i, person := range poolNames("person", s.cast) {
		// The person already exists — validateLadder proved the people ladder
		// reaches this far — so a miss here is a real inconsistency, not a
		// create-if-absent case. Creating one would produce a person with no video
		// link, which the next people reconcile would orphan-stamp anyway.
		pid, ok, err := r.PersonIDByName(ctx, person)
		if err != nil {
			return 0, fmt.Errorf("look up %q: %w", person, err)
		}
		if !ok {
			return 0, fmt.Errorf("person %q does not exist; the video people ladder is "+
				"meant to have created it", person)
		}
		// Billing order is what the cast list sorts by, so seeding it makes that
		// order deterministic instead of incidental — the difference between an
		// assertion about the first-billed tile and one about an arbitrary tile.
		billing := int64(i + 1)
		if err := r.AddFilmPersonRole(ctx, id, pid, ff.person.role, &billing); err != nil {
			return 0, fmt.Errorf("credit %q: %w", person, err)
		}
	}
	return id, nil
}

// seedScenePool builds the videos films attach as scenes.
//
// A scene is a baseline video with a tortured title, and nothing else. Baseline
// rather than bare, because a film's cast, studios and tags are all derived live
// from its attached videos (FilmCast/FilmStudios/FilmTags) — scene videos with no
// relationships would leave three whole sections of the film page empty at every
// rung, so the fixture would never render them at all. Baseline rather than
// varied, because a per-scene cast would make those derived lists grow with the
// scenes rung, and a film page failure could then be the scene count or the cast
// size. Constant and non-zero is what makes the sections render without becoming
// a second varied axis.
//
// They are pool entities, above poolBase and outside every reserved block, for
// the same reason the person and tag pools are: a scene is supporting cast for
// the film dimensions, and giving it an address an assertion could be written
// against would make the block boundaries meaningless.
//
// Deliberately NOT drawn from the addressed video rungs, which is the obvious
// reading of the spec's "scenes are drawn from existing video rungs". Attaching
// the people=25 video to a film would put a film section on that page, so a
// layout failure there could be the cast or the attachment — exactly the
// attribution loss OFAT (D3) exists to prevent. The pool carries the text palette
// instead, which is what that line was actually buying: the film's scene list
// inherits the full text torture without any addressed entity gaining a second
// varied axis.
func seedScenePool(ctx context.Context, r *repo.Repo, ff fixtureFields, size int) ([]int64, error) {
	ids := make([]int64, 0, size)
	for i := range size {
		s := baseline()
		s.text = textPalette[i%len(textPalette)]

		id, err := upsertVideo(ctx, r, ff, fmt.Sprintf("/stress/scene/%03d.mp4", i+1),
			s.text.value, poolLinks(s), s.text.value)
		if err != nil {
			return nil, fmt.Errorf("scene video %d: %w", i+1, err)
		}
		// The pool's whole contract is that it lives outside the addressed range, so
		// it is checked rather than assumed: a steer that silently did nothing would
		// otherwise scatter scene videos through a dimension's block.
		if id < poolBase {
			return nil, fmt.Errorf("scene video %d landed at id %d, inside the addressed "+
				"range below %d", i+1, id, poolBase)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// title is what the entity carries in the UI: its encoded coordinate, unless the
// dimension has declared that it owns the title outright. See dimension.ownsTitle
// for why that exception has to exist.
func title(dim dimension, s spec) string {
	if dim.ownsTitle {
		return s.text.value
	}
	return encodeName(dim.entity, s)
}

// poolNames renders the first n supporting entities of a kind.
func poolNames(kind string, n int) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, poolName(kind, i))
	}
	return out
}

// poolName names a supporting entity. Zero-padded and lowercase: padding keeps
// them ordered in a list, and lowercase matches what the tag writer stores
// anyway, so a seeded name and a read-back name are the same string.
func poolName(kind string, i int) string {
	return fmt.Sprintf("stress %s %03d", kind, i+1)
}

// reset clears the fixture's own rows and rewinds the ID sequences behind them,
// so a run always builds from empty and always lands on the same addresses.
func reset(ctx context.Context, database *sql.DB) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reset: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op once committed

	for _, table := range seededTables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	// Deliberately no sequence rewind here: every sequence generate() depends on
	// is steered explicitly before it is used, so rewinding would be redundant
	// rather than defensive. Removed after a mutation test showed it made no
	// difference to reproducibility — steering is what holds the addresses still.
	return tx.Commit()
}

// steer points a table's AUTOINCREMENT counter at `next`, so the next row
// inserted through the ordinary repo API lands there.
//
// This is how reserved blocks (D4) are honoured without a caller-chosen-ID
// escape hatch: no repo method accepts one, and adding raw INSERTs to get them
// would mean the fixture stopped exercising the write path the app itself uses —
// tag folding, association rules, FTS triggers and all. Steering the counter
// keeps the real API and still lands the row on a chosen address.
//
// sqlite_sequence has no unique index, so this is delete-then-insert rather than
// an upsert.
func steer(ctx context.Context, database *sql.DB, table string, next int64) error {
	if next < 1 {
		return fmt.Errorf("steer %s: next id %d must be positive", table, next)
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin steer %s: %w", table, err)
	}
	defer func() { _ = tx.Rollback() }()

	// Refuse to steer backwards over rows that already exist. Everything above
	// assumes an AUTOINCREMENT counter only moves forward — a lowered counter
	// either collides on insert or, worse, lands the next row inside a block that
	// already holds someone else's entities, which no later check would catch
	// because the id is still inside the block it was supposed to be in. The
	// ordering rules in validateLadder exist to prevent this; this is the assertion
	// that they worked, at the one place the assumption is actually used.
	var highest sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(id) FROM `+table).Scan(&highest); err != nil {
		return fmt.Errorf("read highest id in %s: %w", table, err)
	}
	if highest.Valid && highest.Int64 >= next {
		return fmt.Errorf("cannot steer %s to %d: it already holds id %d.\n"+
			"Steering backwards over existing rows would put the next entity somewhere "+
			"neither its block nor the overflow check predicts — check the dimension "+
			"ordering in ladder.go", table, next, highest.Int64)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM sqlite_sequence WHERE name = ?`, table); err != nil {
		return fmt.Errorf("clear sequence for %s: %w", table, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sqlite_sequence (name, seq) VALUES (?, ?)`, table, next-1); err != nil {
		return fmt.Errorf("set sequence for %s: %w", table, err)
	}
	return tx.Commit()
}

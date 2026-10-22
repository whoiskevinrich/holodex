package main

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"holodex/internal/fieldsource"
	"holodex/internal/model"
)

// The film and category text dimensions (HOLODEX-359). The film page and the
// category page overflowed below ~440px (PRs #314-#316) with nothing in the
// fixture able to reproduce it, and the film description's expand chevron
// (HOLODEX-361) needs prose that fits as well as prose that does not. These pin
// what the geometry harness selects on: each rung names its entity with the
// intended string, a film rung also fills the description, and the manifest says so.

// The short case is the one the chevron gate cannot do without, so it has to lead
// both palettes and stay short; the long cases are the ones the overflow fixes need.
func TestProsePaletteLeadsWithAShortPlainRung(t *testing.T) {
	for _, kind := range []entityKind{kindFilm, kindCategory} {
		t.Run(string(kind), func(t *testing.T) {
			palette := prosePalette(kind)
			if palette[0].key != "plain" {
				t.Fatalf("the first rung is %q, want `plain` — the short case", palette[0].key)
			}
			if n := utf8.RuneCountInString(palette[0].value); n > 40 {
				t.Errorf("the plain rung is %d runes; it has to fit on one line at 375px to be "+
					"the no-chevron case", n)
			}

			keys := map[string]string{}
			for _, v := range palette {
				if _, dup := keys[v.key]; dup {
					t.Errorf("variant %q appears twice", v.key)
				}
				keys[v.key] = v.value
				if v.value == "" {
					t.Errorf("rung %q is empty, but a %s cannot have an empty name", v.key, kind)
				}
				if kind == kindCategory && !strings.HasSuffix(v.value, categoryMark) {
					t.Errorf("category rung %q is %q, missing the mark that keeps it from "+
						"folding onto the tagtext tag of the same string", v.key, v.value)
				}
			}
			for _, want := range []string{"unbroken", "cjk", "rtl", "bidi"} {
				if _, ok := keys[want]; !ok {
					t.Errorf("the %s palette has no %q rung", kind, want)
				}
			}

			// lorem is the one rung whose presence differs, and for a stated reason: a
			// film name has no length limit, a category name is capped by the HTTP layer.
			_, hasLorem := keys["lorem"]
			if wantLorem := kind == kindFilm; hasLorem != wantLorem {
				t.Errorf("lorem kept=%v, want %v: %s", hasLorem, wantLorem,
					nameRejects(kind, textVariant{"lorem", lorem}))
			}
			if _, hasEmpty := keys["empty"]; hasEmpty {
				t.Error("the empty rung was kept, but the repo refuses an empty name")
			}
		})
	}
}

// The category mark exists only to dodge migration 0035's tag collision, so it is
// asserted against the fold that trigger uses: no category rung may fold onto a
// tagtext rung, or CreateCategory refuses it.
func TestCategoryRungsDoNotFoldOntoTagRungs(t *testing.T) {
	fold := func(s string) string { return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "") }
	tags := map[string]string{}
	for _, v := range namePalette(kindTag) {
		tags[fold(v.value)] = v.key
	}
	for _, v := range prosePalette(kindCategory) {
		if tag, clash := tags[fold(v.value)]; clash {
			t.Errorf("category rung %q folds onto tag rung %q", v.key, tag)
		}
	}
}

func TestFilmAndCategoryTextRungsLandAsDeclared(t *testing.T) {
	entries, database := seed(t)

	byDim := map[string]map[string]entry{}
	for _, e := range entries {
		if byDim[e.Dimension] == nil {
			byDim[e.Dimension] = map[string]entry{}
		}
		byDim[e.Dimension][e.Variant] = e
	}

	t.Run("filmtext", func(t *testing.T) {
		for _, v := range prosePalette(kindFilm) {
			e, ok := byDim["filmtext"][v.key]
			if !ok {
				t.Fatalf("no filmtext entity for the %q rung", v.key)
			}
			if e.Entity != kindFilm || !strings.HasPrefix(e.URL, "/films/") {
				t.Errorf("filmtext=%s is a %s at %s, want a film", v.key, e.Entity, e.URL)
			}
			var name string
			if err := database.QueryRow(`SELECT name FROM films WHERE id = ?`, e.ID).Scan(&name); err != nil {
				t.Fatalf("read film %d: %v", e.ID, err)
			}
			if name != v.value {
				t.Errorf("film %d (text=%s) is named %q, want the raw variant %q", e.ID, v.key, name, v.value)
			}
			source, value := filmDescription(t, database, e.ID)
			if source != fieldsource.Manual || value != v.value {
				t.Errorf("film %d (text=%s) description is %s/%q, want a manual decision carrying "+
					"the variant", e.ID, v.key, source, value)
			}
			if e.Axes.Film == nil || e.Axes.Film.Text != v.key {
				t.Errorf("film %d manifest axes %+v, want film.text=%q", e.ID, e.Axes.Film, v.key)
			}
		}
		// The other film dimensions stay without a description: that absence is the
		// empty case filmtext's noEmptyRung points at, and their manifest says so.
		for dim, rungs := range byDim {
			for _, e := range rungs {
				if e.Entity != kindFilm || dim == "filmtext" {
					continue
				}
				if source, _ := filmDescription(t, database, e.ID); source != "" {
					t.Errorf("%s=%s (film %d) carries a description; only filmtext writes one",
						dim, e.Variant, e.ID)
				}
				if e.Axes.Film.Text != "" {
					t.Errorf("%s=%s reports film.text=%q but has no description",
						dim, e.Variant, e.Axes.Film.Text)
				}
			}
		}
	})

	t.Run("categorytext", func(t *testing.T) {
		for _, v := range prosePalette(kindCategory) {
			e, ok := byDim["categorytext"][v.key]
			if !ok {
				t.Fatalf("no categorytext entity for the %q rung", v.key)
			}
			if e.Entity != kindCategory || !strings.HasPrefix(e.URL, "/categories/") {
				t.Errorf("categorytext=%s is a %s at %s, want a category", v.key, e.Entity, e.URL)
			}
			var name string
			if err := database.QueryRow(`SELECT name FROM categories WHERE id = ?`, e.ID).Scan(&name); err != nil {
				t.Fatalf("read category %d: %v", e.ID, err)
			}
			if name != v.value {
				t.Errorf("category %d (text=%s) is named %q, want %q", e.ID, v.key, name, v.value)
			}
			if e.Axes.Name == nil || e.Axes.Name.Value != name || e.Axes.Name.Text != v.key {
				t.Errorf("category %d manifest axes %+v, want name %q / text %q", e.ID, e.Axes.Name, name, v.key)
			}
			if got := countWhere(t, database, "category_tags", "category_id", e.ID); got != baseline().tags {
				t.Errorf("category %d holds %d tags, want the baseline's %d", e.ID, got, baseline().tags)
			}
		}
	})

	// Distinct coordinates: a name shared by two rungs would make a screenshot or a
	// manifest search ambiguous about which page it is.
	t.Run("names are distinct", func(t *testing.T) {
		seen := map[string]string{}
		for _, dim := range []string{"filmtext", "categorytext"} {
			for variant, e := range byDim[dim] {
				if other, dup := seen[e.Name]; dup {
					t.Errorf("%s=%s and %s encode the same name %q", dim, variant, other, e.Name)
				}
				seen[e.Name] = dim + "=" + variant
			}
		}
	})
}

// filmDescription reads the standing description decision on a film, or "" if
// there is none.
func filmDescription(t *testing.T, database *sql.DB, filmID int64) (source, value string) {
	t.Helper()
	err := database.QueryRow(`
		SELECT source, manual_value FROM field_source_decisions
		WHERE entity_type = ? AND entity_id = ? AND field_key = ?`,
		model.EnrichEntityFilm, filmID, filmDescriptionField).Scan(&source, &value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ""
	}
	if err != nil {
		t.Fatalf("read description of film %d: %v", filmID, err)
	}
	return source, value
}

func countWhere(t *testing.T, database *sql.DB, table, col string, id int64) int {
	t.Helper()
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE `+col+` = ?`, id).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

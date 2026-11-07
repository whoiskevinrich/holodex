package api

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"holodex/internal/cache"
	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/repo"
	"holodex/internal/resolver"
)

// R1 (testing-strategy §40.4): a stale resolved value is the quietest failure this feature
// has, and the cause would be a resolver input with no trigger to mark its video dirty. So
// nothing here is a list anyone must remember to extend: each table and each videos column is
// renamed in turn, and a drain that then fails shows it reads that input.

// drainOutputs are written by the drain, not read as inputs.
var drainOutputs = []string{"completeness_dirty", "entity_completeness", "entity_completeness_missing", "resolved_values"}

// notInputs are read along the way but decide no value: studio_image_halo says how a studio
// image is drawn, and it comes along with the studio's images.
var notInputs = []string{"studio_image_halo"}

func r1Fixture(t *testing.T) (*Handlers, *sql.DB, *bytes.Buffer) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	var logs bytes.Buffer
	r := repo.New(database)
	h := NewHandlers(r, slog.New(slog.NewTextHandler(&logs, nil)), nil, t.TempDir(), nil, nil)
	mpath := filepath.Join(t.TempDir(), "metadata-mappings.yaml")
	const mappings = "fields:\n" +
		"  - canonical: title\n    label: Title\n    sources: [fake:title, file:title]\n" +
		"  - canonical: director\n    label: Director\n    sources: [fake:director, file:Director]\n" +
		"  - canonical: studio\n    label: Studio\n    sources: [fake:studio, file:Publisher]\n" +
		"  - canonical: container\n    label: Container\n    sources: [file:container]\n" +
		"  - canonical: video_codec\n    label: Codec\n    sources: [file:video_codec]\n" +
		"  - canonical: runtime\n    label: Runtime\n    sources: [file:duration_sec]\n" +
		"  - canonical: path\n    label: Path\n    sources: [file:file_path]\n"
	if err := os.WriteFile(mpath, []byte(mappings), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := mapping.NewStore(mpath)
	if err != nil {
		t.Fatal(err)
	}
	h.SetMetadataFields(store, cache.Noop{})
	if _, err := r.UpsertVideo(context.Background(), &model.Video{
		FilePath: "/m/a.mkv", FileSize: 1, Title: "a", Duration: 60, FileMtime: time.Unix(1_700_000_000, 0).UTC(),
	}, []model.ExtraMetadata{{SourceKey: "Director", Value: "Ann Lee"}}); err != nil {
		t.Fatal(err)
	}
	// One row of every kind a loader walks, or a query behind `if len(ids) > 0` never runs.
	for _, q := range []string{
		`INSERT INTO people (name) VALUES ('Ann Lee')`,
		`INSERT INTO studios (name) VALUES ('Acme')`,
		`INSERT INTO tags (name) VALUES ('noir')`,
		`INSERT INTO films (name) VALUES ('A film')`,
		`INSERT INTO video_people (video_id, person_id, role) VALUES (1, 1, 'actor')`,
		`INSERT INTO video_studios (video_id, studio_id) VALUES (1, 1)`,
		`INSERT INTO video_tags (video_id, tag_id) VALUES (1, 1)`,
		`INSERT INTO film_videos (film_id, video_id, created_at) VALUES (1, 1, '2026-01-01T00:00:00Z')`,
	} {
		if _, err := database.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	return h, database, &logs
}

// drainFails marks everything dirty beforehand, since MarkAll itself reads some inputs, and
// reports whether a drain over the renamed schema fell over, in its error or its log.
func drainFails(ctx context.Context, h *Handlers, logs *bytes.Buffer, rename, restore func() error) (bool, error) {
	if err := h.repo.MarkAllCompletenessDirty(ctx); err != nil {
		return false, err
	}
	if err := rename(); err != nil {
		return false, err
	}
	logs.Reset()
	err := h.drainCompletenessStrict(ctx)
	if rerr := restore(); rerr != nil {
		return false, rerr
	}
	failed := err != nil || strings.Contains(logs.String(), "no such")
	if drainErr := h.drainCompletenessStrict(ctx); drainErr != nil {
		return false, drainErr
	}
	return failed, nil
}

func names(t *testing.T, d *sql.DB, q string) []string {
	t.Helper()
	rows, err := d.Query(q)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestEveryResolverInputTableHasATrigger(t *testing.T) {
	h, d, logs := r1Fixture(t)
	ctx := context.Background()
	if err := h.drainCompletenessStrict(ctx); err != nil {
		t.Fatalf("the unrenamed schema drains: %v", err)
	}
	tables := names(t, d, `SELECT name FROM sqlite_master WHERE type = 'table'
		AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '%fts%' AND name != 'schema_migrations'`)
	triggered := names(t, d, `SELECT DISTINCT tbl_name FROM sqlite_master WHERE type = 'trigger' AND name LIKE 'cd\_%' ESCAPE '\'`)

	var read []string
	for _, table := range tables {
		rename := func(from, to string) func() error {
			return func() error {
				_, err := d.Exec(`ALTER TABLE ` + from + ` RENAME TO ` + to)
				return err
			}
		}
		failed, err := drainFails(ctx, h, logs, rename(table, table+"_probe"), rename(table+"_probe", table))
		if err != nil {
			t.Fatalf("probing %s: %v", table, err)
		}
		if failed {
			read = append(read, table)
		}
	}
	if !slices.Contains(read, "video_metadata") || !slices.Contains(read, "entity_enrichment") {
		t.Fatalf("the probe missed tables the resolver certainly reads, so it proves nothing: %v", read)
	}
	for _, table := range read {
		if !slices.Contains(triggered, table) && !slices.Contains(drainOutputs, table) && !slices.Contains(notInputs, table) {
			t.Errorf("the drain reads %s, which has no cd_* trigger: a write there would leave resolved values stale", table)
		}
	}
}

// The drain lists whole videos rows, so renaming a column shows nothing about what resolves.
// Vary each field of a video instead: only one that changes the result is an input, and its
// column must be in cd_videos_au's list.
func TestEveryResolverInputColumnIsWatched(t *testing.T) {
	h, d, _ := r1Fixture(t)
	var trigger string
	if err := d.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'cd_videos_au'`).Scan(&trigger); err != nil {
		t.Fatal(err)
	}
	fields := h.mappings.Current().Fields()
	base := resolver.Resolve(&model.Video{}, nil, nil, nil, fields, resolver.Options{})
	typ := reflect.TypeFor[model.Video]()
	var inputs []string
	for i := range typ.NumField() {
		v := model.Video{}
		f := reflect.ValueOf(&v).Elem().Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString("x")
		case reflect.Int, reflect.Int64:
			f.SetInt(7)
		case reflect.Bool:
			f.SetBool(true)
		default:
			continue // slices, pointers and times: links and dates, never resolved from the row
		}
		if !reflect.DeepEqual(resolver.Resolve(&v, nil, nil, nil, fields, resolver.Options{}), base) {
			inputs = append(inputs, typ.Field(i).Name)
		}
	}
	if !slices.Equal(inputs, []string{"Title"}) {
		t.Errorf("resolved values read these video fields: %v. Each one's column needs adding to cd_videos_au", inputs)
	}
	if !strings.Contains(trigger, "UPDATE OF title") {
		t.Errorf("cd_videos_au doesn't watch title: %s", trigger)
	}
}

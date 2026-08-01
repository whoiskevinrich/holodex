package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"holodex/internal/model"
)

// Tag Categories (HOLODEX-240, ADR-078): a deliberately reduced entity --
// create, rename, delete only, no provenance/alias/merge. Kept outside
// resolveOrCreateByName's identity spine (D1/D4): Category is created exactly
// one way (an owner's explicit action), never through the scanner, so it has
// none of the scanner-driven-duplicate problem the spine amortizes for
// person/studio/tag. The cross-table name collision with tags (D3) is
// pre-checked here for a friendly 409 -- migration 0034's paired DB triggers
// are the actual correctness backstop, catching any insert/update path that
// bypasses this check.

// ErrCategoryNameCollidesWithTag is returned by CreateCategory/RenameCategory
// when name already names a tag (ADR-078 D3's app-layer pre-flight check).
var ErrCategoryNameCollidesWithTag = errors.New("category: name collides with an existing tag")

// ErrTagNameCollidesWithCategory is returned by resolveOrCreateByName's tag
// path when name already names a category -- the symmetric pre-flight check
// (ADR-078 D3) on the tag side of the collision.
var ErrTagNameCollidesWithCategory = errors.New("tag: name collides with an existing category")

// nameCollidesInTable reports whether name already names a row in table,
// folded the same way tags fold their own uniqueness (nameKeyExpr's tag
// variant) -- required so the cross-table comparison is meaningful (ADR-078
// Forces). excludeID excludes that id (a rename's self-match); 0 excludes
// nothing (create), since real ids start at 1. table is a trusted internal
// literal ("tags" | "categories"), never user input.
func nameCollidesInTable(ctx context.Context, qr queryRower, table, name string, excludeID int64) (bool, error) {
	var x int
	err := qr.QueryRowContext(ctx,
		`SELECT 1 FROM `+table+` WHERE `+nameKeyExpr(model.EntityTag, "name")+` = `+nameKeyExpr(model.EntityTag, "?")+` AND id <> ?`,
		name, excludeID).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%s collision check: %w", table, err)
	}
	return true, nil
}

// CreateCategory creates a category named name, rejecting a collision (tag-
// style fold) with an existing tag (ErrCategoryNameCollidesWithTag) or
// another category (ErrNameTaken). Unlike resolveOrCreateByName's tag path,
// this never resolves to an existing category on collision -- category
// creation is an explicit owner action, not an attach-by-name.
func (r *Repo) CreateCategory(ctx context.Context, name string) (*model.Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("empty name")
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if collides, err := nameCollidesInTable(ctx, r.db, "tags", name, 0); err != nil {
		return nil, err
	} else if collides {
		return nil, ErrCategoryNameCollidesWithTag
	}
	if collides, err := nameCollidesInTable(ctx, r.db, "categories", name, 0); err != nil {
		return nil, err
	} else if collides {
		return nil, ErrNameTaken
	}

	res, err := r.db.ExecContext(ctx, `INSERT INTO categories (name) VALUES (?)`, name)
	if err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetCategory(ctx, id)
}

// RenameCategory renames a category, rejecting a collision exactly as
// CreateCategory does. Renaming to the current exact name is a no-op success
// (mirrors RenameEntity). ErrNotFound if id doesn't exist.
func (r *Repo) RenameCategory(ctx context.Context, id int64, newName string) (*model.Category, error) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, errors.New("empty name")
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	var oldName string
	switch err := r.db.QueryRowContext(ctx, `SELECT name FROM categories WHERE id = ?`, id).Scan(&oldName); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("rename category: load: %w", err)
	}
	if oldName == newName {
		return r.GetCategory(ctx, id)
	}
	if collides, err := nameCollidesInTable(ctx, r.db, "tags", newName, 0); err != nil {
		return nil, err
	} else if collides {
		return nil, ErrCategoryNameCollidesWithTag
	}
	if collides, err := nameCollidesInTable(ctx, r.db, "categories", newName, id); err != nil {
		return nil, err
	} else if collides {
		return nil, ErrNameTaken
	}

	if _, err := r.db.ExecContext(ctx, `UPDATE categories SET name = ? WHERE id = ?`, newName, id); err != nil {
		return nil, fmt.Errorf("rename category: %w", err)
	}
	return r.GetCategory(ctx, id)
}

// DeleteCategory deletes a category. Every category_tags row referencing it
// is dropped by ON DELETE CASCADE (ADR-078 D2) -- no dependent-tag block, the
// member tags themselves are unaffected. ErrNotFound if id doesn't exist.
func (r *Repo) DeleteCategory(ctx context.Context, id int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	res, err := r.db.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListCategories returns every category, name-ordered, with its member-tag
// count (the /tags pill's count badge) and member tag ids (the "Remove from
// category…" picker's client-side filter, HOLODEX-240) but no per-row tag
// objects (no per-row use for those on the list surface, spec Non-Goals).
// Always a non-nil slice so the JSON serializes as [] not null.
func (r *Repo) ListCategories(ctx context.Context) ([]model.Category, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name FROM categories ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	out := []model.Category{}
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Member tag ids, in one pass over category_tags -- TagCount is len(TagIDs),
	// derived below rather than a separate COUNT/JOIN query above (personal-
	// library scale, so this one extra full pass is cheap either way).
	byID := make(map[int64]*model.Category, len(out))
	for i := range out {
		byID[out[i].ID] = &out[i]
	}
	memberRows, err := r.db.QueryContext(ctx, `SELECT category_id, tag_id FROM category_tags`)
	if err != nil {
		return nil, fmt.Errorf("list categories: member tags: %w", err)
	}
	defer memberRows.Close()
	for memberRows.Next() {
		var catID, tagID int64
		if err := memberRows.Scan(&catID, &tagID); err != nil {
			return nil, err
		}
		if c, ok := byID[catID]; ok {
			c.TagIDs = append(c.TagIDs, tagID)
		}
	}
	if err := memberRows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].TagCount = len(out[i].TagIDs)
	}
	return out, nil
}

// GetCategory returns a category by id with its member tags, or ErrNotFound.
func (r *Repo) GetCategory(ctx context.Context, id int64) (*model.Category, error) {
	var c model.Category
	switch err := r.db.QueryRowContext(ctx, `SELECT id, name FROM categories WHERE id = ?`, id).Scan(&c.ID, &c.Name); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("get category: %w", err)
	}
	tags, err := r.TagsForCategory(ctx, id)
	if err != nil {
		return nil, err
	}
	c.Tags = tags
	c.TagCount = len(tags)
	return &c, nil
}

// ResolveOrCreateTag resolves-or-creates a tag by name with no video attach
// (HOLODEX-240) -- the first step of the /categories/{id} "+ Add tag"
// control (AssignTagsToCategory does the second, linking the resolved id).
// Shares resolveOrCreateByName, the same choke point AttachTagToVideo and the
// scanner already route through, so a name that matches an existing tag/alias
// resolves to it instead of duplicating, and the deny-list/length-cap/
// category-collision checks (ADR-078 D3) all apply here too.
func (r *Repo) ResolveOrCreateTag(ctx context.Context, name string) (*model.Tag, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	tid, tagName, err := resolveOrCreateTagName(ctx, tx, name)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &model.Tag{ID: tid, Name: tagName}, nil
}

// TagsForCategory returns a category's member tags, name-ordered.
func (r *Repo) TagsForCategory(ctx context.Context, categoryID int64) ([]model.Tag, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.name FROM tags t
		JOIN category_tags ct ON ct.tag_id = t.id
		WHERE ct.category_id = ?
		ORDER BY t.name COLLATE NOCASE`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("tags for category: %w", err)
	}
	defer rows.Close()
	out := []model.Tag{}
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// categoryExists reports that a category id is present (ErrNotFound
// otherwise) -- a cheap existence check ahead of the assign/unassign writes
// below, mirroring EntityExists/PersonExists.
func (r *Repo) categoryExists(ctx context.Context, id int64) error {
	var x int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM categories WHERE id = ?`, id).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// AssignTagsToCategory links every one of tagIDs to categoryID in a single
// batched statement (idempotent: INSERT OR IGNORE, so a bulk "Add to
// category…" action re-assigning an already-member tag alongside new ones is
// a no-op for that tag, not an error) and returns the updated category.
// ErrNotFound if categoryID doesn't exist.
func (r *Repo) AssignTagsToCategory(ctx context.Context, categoryID int64, tagIDs []int64) (*model.Category, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if err := r.categoryExists(ctx, categoryID); err != nil {
		return nil, err
	}
	if len(tagIDs) > 0 {
		rows := strings.TrimSuffix(strings.Repeat("(?,?),", len(tagIDs)), ",")
		args := make([]any, 0, len(tagIDs)*2)
		for _, tid := range tagIDs {
			args = append(args, categoryID, tid)
		}
		if _, err := r.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO category_tags (category_id, tag_id) VALUES `+rows, args...); err != nil {
			return nil, fmt.Errorf("assign tags to category: %w", err)
		}
	}
	return r.GetCategory(ctx, categoryID)
}

// UnassignTagsFromCategory removes every one of tagIDs' link to categoryID in
// a single batched statement (a no-op for a tag that wasn't a member) and
// returns the updated category. ErrNotFound if categoryID doesn't exist.
func (r *Repo) UnassignTagsFromCategory(ctx context.Context, categoryID int64, tagIDs []int64) (*model.Category, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if err := r.categoryExists(ctx, categoryID); err != nil {
		return nil, err
	}
	if len(tagIDs) > 0 {
		args := append([]any{categoryID}, toAnySlice(tagIDs)...)
		if _, err := r.db.ExecContext(ctx,
			`DELETE FROM category_tags WHERE category_id = ? AND tag_id IN (`+placeholders(len(tagIDs))+`)`, args...); err != nil {
			return nil, fmt.Errorf("unassign tags from category: %w", err)
		}
	}
	return r.GetCategory(ctx, categoryID)
}

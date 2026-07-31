---
key: HOLODEX-240
status: In Progress
depends-on: [HOLODEX-239]
release_note: "Tags can now be grouped into hand-curated categories — browsable and filterable alongside tags, with no effect on file writeback."
---

## Gates — definition of done
- [x] spec          docs/specs/tag-categories.md · S1
- [ ] architecture  not started
- [ ] backend       not started
- [ ] frontend      not started
- [~] testing       deferred until: backend/frontend implemented
- [~] security      deferred until: architecture + backend implemented

## Up next   (ordered — position is the priority; top line is the next action)
1. Write the ADR covering the Category entity's reduced lifecycle + the many-to-many junction shape  [architecture]
2. Migrations: `categories` table + `tag_categories` junction; shared name-collision fold with tags  [backend]
3. Category CRUD endpoints (create/rename/delete, cascade-unassign on delete)  [backend]
4. `/tags` name-search endpoint/param (prerequisite — doesn't exist today) + All/Tags/Categories filter  [backend, frontend]
5. `/categories/{id}` page: member-tag chips, add/remove, no video grid  [frontend]
6. Bulk "Add to category…" / "Remove from category…" on `/tags` Manage mode, reusing `EntityPicker`  [frontend]
7. Browse-page "Categories" facet, expanding to member tag IDs against the existing Tags facet  [backend, frontend]
8. Regenerate testing-strategy for the new endpoints/flows  [testing]
9. Security review  [security]

## Session log   (append-only)
S1 · /product-brainstorming /write-spec — spec drafted, epic filed (blocked-by HOLODEX-239), Draft PR opened
